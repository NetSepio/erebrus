package agents

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/api/v1/middleware"
	caddy "github.com/NetSepio/erebrus/api/v1/service"
	"github.com/NetSepio/erebrus/model"
	"github.com/gin-gonic/gin"
)

// ApplyRoutes applies router to gin Router
func ApplyRoutes(r *gin.RouterGroup) {
	g := r.Group("/agents")
	{
		g.POST("", addAgent)
		g.GET("", getAgents)
		g.GET(":agentId", getAgent)
		g.DELETE(":agentId", deleteAgent)
		g.PATCH("/manage/:agentId", manageAgent)
	}
}

var agentsFilePath string

func init() {
	// Initialize the agentsFilePath during package initialization
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error getting home directory: %v", err)
	}
	// Create the "erebrus" folder(SERVICE_CONF_DIR) inside the home directory if it doesn't exist
	erebrusDir := filepath.Join(homeDir, "erebrus")
	err = os.MkdirAll(erebrusDir, os.ModePerm)
	if err != nil {
		log.Fatalf("Error creating erebrus directory: %v", err)
	}

	// Set the path for agents.json inside the erebrus folder
	agentsFilePath = filepath.Join(erebrusDir, "agents.json")

	monitorAndRecoverAgents()

}

// Load agents from file
func loadAgents() ([]model.Agent, error) {
	file, err := os.Open(agentsFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.Agent{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var agents []model.Agent
	if err := json.NewDecoder(file).Decode(&agents); err != nil {
		return nil, err
	}
	return agents, nil
}

// Save agents to file
func saveAgents(newAgent model.Agent) error {
	// Load existing agents
	agents, err := loadAgents()
	if err != nil {
		return err
	}

	agents = append(agents, newAgent)

	file, err := os.Create(agentsFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Encode the updated agents list into the file with indentation
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(agents)
}

// GET /agents
func getAgents(c *gin.Context) {
	agents, err := loadAgents()
	if err != nil {
		log.Printf("Error loading agents: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load agents"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"agents": agents})
}

// GET /agents/:agentId
func getAgent(c *gin.Context) {
	agentID := c.Param("agentId")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Agent ID is required"})
		return
	}

	agents, err := loadAgents()
	if err != nil {
		log.Printf("Error loading agents: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load agents"})
		return
	}

	for _, agent := range agents {
		if strings.EqualFold(agent.ID, agentID) {
			c.JSON(http.StatusOK, gin.H{
				"agent": gin.H{
					"id":      agent.ID,
					"name":    agent.Name,
					"clients": agent.Clients,
					"domain":  agent.Domain,
					"status":  agent.Status,
				},
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
}

// Function to find an available port on the host machine
func getAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// POST /agents
func addAgent(c *gin.Context) {
	log.Println("Received request to add an agent.")

	// Retrieve the file from the request
	file, err := c.FormFile("character_file")
	if err != nil {
		log.Printf("Error retrieving character file: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to retrieve character file"})
		return
	}

	log.Printf("Uploaded file: %s", file.Filename)

	// Read the saved file
	content, err := file.Open()
	if err != nil {
		log.Printf("Error opening file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	// Decode the JSON content
	var character model.CharacterFile

	if err := json.NewDecoder(content).Decode(&character); err != nil {
		log.Printf("Invalid JSON format: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON file"})
		return
	}

	// Ensure the "name" field is present
	if character.Name == "" {
		log.Printf("Missing 'name' field in JSON file")
		c.JSON(http.StatusBadRequest, gin.H{"error": "'name' field is required in the JSON file"})
		return
	}

	agentName := character.Name

	// Ensure the characters directory exists
	if _, err := os.Stat("./characters"); os.IsNotExist(err) {
		log.Println("Characters directory does not exist. Creating...")
		if err := os.Mkdir("./characters", 0755); err != nil {
			log.Printf("Error creating characters directory: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create characters directory"})
			return
		}
	}

	characterFilePath := fmt.Sprintf("./characters/%s/%s", agentName, file.Filename)

	// Save the file to the characters directory
	log.Printf("Saving character file to %s", characterFilePath)
	if err := c.SaveUploadedFile(file, characterFilePath); err != nil {
		log.Printf("Error saving character file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save character file: %s", err.Error())})
		return
	}

	// Ensure the Docker image is present
	dockerImage := os.Getenv("DOCKER_IMAGE_AGENT")
	log.Printf("Checking Docker image: %s", dockerImage)
	pullCmd := exec.Command("docker", "pull", dockerImage)
	if output, err := pullCmd.CombinedOutput(); err != nil {
		log.Printf("Error pulling Docker image: %s", string(output))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to pull Docker image: %s", string(output))})
		return
	}

	// Find an available port
	exposedPort, err := getAvailablePort()
	if err != nil {
		log.Printf("Error finding available port: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find an available port"})
		return
	}

	// Run Docker container
	log.Printf("Starting Docker container for agent: %s on port: %d", agentName, exposedPort)
	dockerCmd := exec.Command(
		"docker", "run", "-d",
		"--name", agentName,
		"-p", fmt.Sprintf("%d:3000", exposedPort),
		"-v", fmt.Sprintf("%s:/app/characters", "./characters"),
		dockerImage,
		"pnpm", "start", fmt.Sprintf("--character=/app/characters/%s/%s", agentName, file.Filename),
	)

	output, err := dockerCmd.CombinedOutput()
	if err != nil {
		log.Printf("Error starting Docker container: %s", string(output))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start Docker container: %s", string(output))})
		return
	}

	log.Printf("Docker container started successfully: %s", string(output))

	time.Sleep(time.Duration(30) * time.Second)

	// Determine the domain
	domain := c.DefaultPostForm("domain", "")
	if domain == "" {
		domain = os.Getenv("EREBRUS_DOMAIN")
	}

	// Call the AddServicesDirect function from the caddy package
	log.Printf("Adding services for domain: %s", domain)
	if err := caddy.AddServicesDirect(domain, agentName, exposedPort); err != nil {
		log.Printf("Error adding services: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add services"})
		return
	}

	// Allow the container to start and stabilize
	agentEndpoint := fmt.Sprintf("http://localhost:%d/agents", exposedPort)
	log.Println("Fetching agents from container at", agentEndpoint)
	resp, err := http.Get(agentEndpoint)
	if err != nil {
		log.Printf("Error fetching agents: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch agents from container"})
		return
	}
	defer resp.Body.Close()

	var agentsResponse struct {
		Agents []model.Agent `json:"agents"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&agentsResponse); err != nil {
		log.Printf("Error parsing agents response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse agents response"})
		return
	}

	// Filter agents by the requested name
	var createdAgent *model.Agent
	for _, agent := range agentsResponse.Agents {
		if strings.EqualFold(agent.Name, agentName) {
			createdAgent = &agent
			break
		}
	}

	if createdAgent == nil {
		log.Printf("Agent creation failed for: %s", agentName)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Agent creation failed"})
		return
	}
	domain = agentName + "." + domain

	createdAgent.Port = exposedPort
	createdAgent.Domain = domain
	createdAgent.Status = "active"

	saveAgents(*createdAgent)

	response := model.AgentResponse{
		ID:      createdAgent.ID,
		Name:    createdAgent.Name,
		Clients: createdAgent.Clients,
		Status:  createdAgent.Status,
	}

	log.Printf("Agent created successfully: %+v", response)
	c.JSON(http.StatusOK, gin.H{"agent": response, "domain": domain})
}

// DELETE /agents/:agentId
func deleteAgent(c *gin.Context) {
	agentID := c.Param("agentId")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Agent ID is required"})
		return
	}

	// Load existing agents
	agents, err := loadAgents()
	if err != nil {
		log.Printf("Error loading agents: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load agents"})
		return
	}

	// Find the agent by ID and remove it
	var indexToDelete int = -1
	for i, agent := range agents {
		if strings.EqualFold(agent.ID, agentID) {
			indexToDelete = i
			break
		}
	}

	// Stop and remove the Docker container for the deleted agent
	dockerCmd := exec.Command("docker", "stop", agents[indexToDelete].Name)
	if output, err := dockerCmd.CombinedOutput(); err != nil {
		log.Printf("Error stopping Docker container: %s", string(output))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to stop Docker container: %s", string(output))})
		return
	}

	dockerRemoveCmd := exec.Command("docker", "rm", agents[indexToDelete].Name)
	if output, err := dockerRemoveCmd.CombinedOutput(); err != nil {
		log.Printf("Error removing Docker container: %s", string(output))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to remove Docker container: %s", string(output))})
		return
	}

	// If the agent is not found, return an error
	if indexToDelete == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	//to delete from caddyfile and caddy.json
	middleware.DeleteService(agents[indexToDelete].Name)

	// Remove the agent from the list
	agents = append(agents[:indexToDelete], agents[indexToDelete+1:]...)

	// Save the updated list of agents
	if err := saveAgentsList(agents); err != nil {
		log.Printf("Error saving agents: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save agents"})
		return
	}

	// Respond with success
	log.Printf("Agent %s deleted successfully", agentID)
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Agent %s deleted successfully", agentID)})
}

// Save the list of agents back to the file after deletion
func saveAgentsList(agents []model.Agent) error {
	// Open or create the file to save the agents list
	file, err := os.Create(agentsFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Encode the updated agents list into the file with indentation
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(agents)
}

func manageAgent(c *gin.Context) {
	agentID := c.Param("agentId")
	if agentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Agent ID is required"})
		return
	}

	action := c.Query("action")
	if action != "pause" && action != "resume" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action. Use 'pause' or 'resume'"})
		return
	}

	// Load existing agents
	agents, err := loadAgents()
	if err != nil {
		log.Printf("Error loading agents: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load agents"})
		return
	}

	var dockerAction string
	if action == "pause" {
		dockerAction = "pause"
	} else {
		dockerAction = "unpause"
	}

	// Find the agent by ID and remove it
	var agentIndex int = -1
	for i, agent := range agents {
		if strings.EqualFold(agent.ID, agentID) {
			agentIndex = i
			if dockerAction == "pause" {
				agents[i].Status = "inactive"
			} else {
				agents[i].Status = "active"
			}
			break
		}
	}

	// If the agent is not found, return an error
	if agentIndex == -1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Agent not found"})
		return
	}

	// Write the updated data back to the file
	updatedData, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		log.Printf("Error marshalling updated JSON: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update agents data"})
		return
	}

	file, err := os.Create(agentsFilePath)
	if err != nil {
		log.Printf("Error creating agents.json: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save updated agents"})
		return
	}

	defer file.Close()

	if _, err := file.Write(updatedData); err != nil {
		log.Printf("Error writing to agents.json: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write agents data"})
		return
	}

	// Execute the pause or resume action
	actionCmd := exec.Command("docker", dockerAction, agents[agentIndex].Name)
	actionOutput, err := actionCmd.CombinedOutput()
	if err != nil {
		log.Printf("Error performing action '%s' on Agent: %s, Output: %s", dockerAction, err, string(actionOutput))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to %s Agent: %s", dockerAction, string(actionOutput))})
		return
	}

	log.Printf("Successfully performed action '%s' on Agent '%s'", dockerAction, agentID)
	c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Agent '%s' %sed successfully", agentID, dockerAction)})
}

func monitorAndRecoverAgents() {
	ticker := time.NewTicker(15 * time.Second)
	go func() {
		for range ticker.C {
			agents, err := loadAgents()
			if err != nil {
				log.Printf("Error loading agents for recovery: %v", err)
				continue
			}

			for _, agent := range agents {
				// Check container status
				cmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", agent.Name)
				output, err := cmd.Output()
				if err != nil {
					log.Printf("Error checking container status for %s: %v", agent.Name, err)
					recreateErr := recreateAgent(agent)
					if recreateErr != nil {
						log.Printf("Failed to recreate agent %s: %v", agent.Name, recreateErr)
					}
					continue
				}

				status := strings.TrimSpace(string(output))
				if status != "true" {
					log.Printf("Agent %s is not running. Attempting to restart...", agent.Name)

					// Restart the container
					restartCmd := exec.Command("docker", "restart", agent.Name)
					if restartOutput, restartErr := restartCmd.CombinedOutput(); restartErr != nil {
						log.Printf("Failed to restart agent %s: %v, Output: %s",
							agent.Name, restartErr, string(restartOutput))

						// If restart fails, try to recreate the container
						recreateErr := recreateAgent(agent)
						if recreateErr != nil {
							log.Printf("Failed to recreate agent %s: %v", agent.Name, recreateErr)
						}

						return
					}

					// check the status of agent and and set it accordingly for container
					if agent.Status == "inactive" {
						actionCmd := exec.Command("docker", "pause", agent.Name)
						actionOutput, err := actionCmd.CombinedOutput()
						if err != nil {
							log.Printf("Error performing action pause on Agent: %s, Output: %s", err, string(actionOutput))
							return
						}
					}

					log.Printf("Agent %s is restored", agent.Name)
				}

			}
		}
	}()
}

func recreateAgent(agent model.Agent) error {
	// Stop and remove existing container if it exists
	stopCmd := exec.Command("docker", "stop", agent.Name)
	stopCmd.Run()

	removeCmd := exec.Command("docker", "rm", agent.Name)
	removeCmd.Run()

	// Recreate the container using the original parameters
	dockerCmd := exec.Command(
		"docker", "run", "-d",
		"--name", agent.Name,
		"-p", fmt.Sprintf("%d:3000", agent.Port),
		"-v", fmt.Sprintf("%s:/app/characters", "./characters"),
		os.Getenv("DOCKER_IMAGE_AGENT"),
		"pnpm", "start", fmt.Sprintf("--character=/app/characters/%s/%s.character.json", agent.Name, agent.Name),
	)

	output, err := dockerCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to recreate container: %v, output: %s", err, string(output))
	}

	if agent.Status == "inactive" {
		actionCmd := exec.Command("docker", "pause", agent.Name)
		actionOutput, err := actionCmd.CombinedOutput()
		if err != nil {
			log.Printf("Error performing action pause on Agent: %s, Output: %s", err, string(actionOutput))
		}
	}

	fmt.Println("Successfully recreated the agent container:", agent.Name)

	return nil
}
