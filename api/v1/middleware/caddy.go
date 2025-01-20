package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/NetSepio/erebrus/api/v1/service/template"
	"github.com/NetSepio/erebrus/api/v1/service/util"
	"github.com/NetSepio/erebrus/model"
)

// IsValid check if model is valid
func IsValidService(name string, port int, ipAddress string) (int, string, error) {
	// Check if the name is empty
	fmt.Printf("Checking service name: %s, port: %d\n", name, port)
	if name == "" {
		fmt.Println("Service name is empty")
		return -1, "Services Name is required", nil
	}

	// Check the name field length
	fmt.Printf("Service name length: %d\n", len(name))
	if len(name) < 4 || len(name) > 50 {
		fmt.Println("Service name length is invalid")
		return -1, "Services Name field must be between 4-12 chars", nil
	}

	// Read existing services
	fmt.Println("Reading web services...")
	Services, err := ReadServices()
	if err != nil {
		if err.Error() == "caddy file is empty while reading file" {
			fmt.Println("Caddy file is empty, proceeding to create a new Services")
		} else {
			fmt.Printf("Error reading web services: %v\n", err)
			return -1, "", err
		}
	} else {
		fmt.Printf("Read web services successfully: %+v\n", Services)
	}

	// Check if the name or port is already in use
	if Services != nil {
		for _, service := range Services.Services {
			fmt.Printf("Checking service: %+v\n", service)
			if service.Name == name {
				fmt.Println("Service name already exists")
				return -1, "Service Already exists", nil
			} else if service.IpAddress == ipAddress && service.Port == strconv.Itoa(port) {
				fmt.Println("Port and IP address combination is already in use")
				return -1, "Port and IP address combination already in use", nil
			}
		}
	}

	// Validate the format of the name
	// if !util.IsLetter(name) {
	// 	fmt.Println("Service name is not alphanumeric")
	// 	return -1, "Services Name should be Alphanumeric", nil
	// }

	fmt.Println("Service name and port are valid")
	return 1, "", nil
}

// ReadServices fetches all the Web Tunnel services
func ReadServices() (*model.ServicesList, error) {
	// Get the CADDY_CONF_DIR environment variable
	caddyConfDir := os.Getenv("CADDY_CONF_DIR")
	if caddyConfDir == "" {
		return nil, fmt.Errorf("CADDY_CONF_DIR environment variable is not set")
	}

	// Ensure the directory exists
	if _, err := os.Stat(caddyConfDir); os.IsNotExist(err) {
		err = os.MkdirAll(caddyConfDir, 0755) // Create the directory with proper permissions
		if err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", caddyConfDir, err)
		}
	}

	// Construct the file path
	filePath := filepath.Join(caddyConfDir, "caddy.json")
	fmt.Println("filePath : ", filePath)

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Create the file at the specified location
		file, err := os.Create(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file at %s: %w", filePath, err)
		}
		defer file.Close()

		// Initialize with empty JSON structure
		if _, writeErr := file.WriteString(`{"services": []}`); writeErr != nil {
			return nil, fmt.Errorf("failed to write initial JSON to file: %w", writeErr)
		}
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read the file contents
	b, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Handle empty file
	if len(b) == 0 {
		fmt.Println("Caddy file is empty while reading file", &model.ServicesList{Services: []model.Service{}})
		return &model.ServicesList{Services: []model.Service{}}, nil
	}

	// Parse the JSON contents
	var Services model.ServicesList
	err = json.Unmarshal(b, &Services)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &Services, nil
}

// ReadWebTunnel fetches a Web Tunnel
func ReadService(tunnelName string) (*model.Service, error) {
	Services, err := ReadServices()
	if err != nil {
		return nil, err
	}

	var data model.Service
	for _, Service := range Services.Services {
		// print all the services
		if Service.Name == tunnelName {
			data.Name = Service.Name
			data.Port = Service.Port
			data.CreatedAt = Service.CreatedAt
			data.Domain = Service.Domain
			data.Status = Service.Status
			break
		}
	}

	return &data, nil
}

func AddServices(newService model.Service) error {
	// Read existing services
	servicesList, err := ReadServices()
	if err != nil {
		if err.Error() == "caddy file is empty while reading file" {
			util.LogError("Caddy file is empty, proceeding to create a new Services", nil)
			servicesList = &model.ServicesList{Services: []model.Service{}} // Initialize an empty Services struct
		} else {
			return err
		}
	}

	// Ensure the services list is initialized
	if servicesList == nil || servicesList.Services == nil {
		servicesList = &model.ServicesList{Services: []model.Service{}}
	}

	// Append the new service
	servicesList.Services = append(servicesList.Services, newService)

	// Marshal the updated services list to JSON
	updatedJSON, err := json.MarshalIndent(servicesList, "", "   ")
	if err != nil {
		util.LogError("JSON Marshal error: ", err)
		return err
	}

	//to save/update in /etc/caddy and service_conf_dir
	err = SaveToFile(updatedJSON)
	if err != nil {
		util.LogError("failed to save/update data in config files: ", err)
		return err
	}

	// Update the Caddy configuration
	err = UpdateCaddyConfig()
	if err != nil {
		util.LogError("Caddy configuration update error: ", err)
		return err
	}

	return nil
}

func DeleteService(serviceName string) error {
	services, err := ReadServices()
	if err != nil {
		return err
	}

	var updatedServices []model.Service
	for _, service := range services.Services {
		if service.Name == serviceName {
			continue
		}
		updatedServices = append(updatedServices, service)
	}

	newServices := &model.ServicesList{
		Services: updatedServices,
	}

	jsonData, err := json.MarshalIndent(newServices, "", "   ")
	if err != nil {
		util.LogError("JSON Marshal error: ", err)
		return err
	}

	err = SaveToFile(jsonData)
	if err != nil {
		util.LogError("failed to save/update data in config files: ", err)
		return err
	}

	err = UpdateCaddyConfig()
	if err != nil {
		return err
	}

	return nil
}

// UpdateCaddyConfig updates Caddyfile
func UpdateCaddyConfig() error {
	Services, err := ReadServices()
	if err != nil {
		return err
	}

	path := filepath.Join(os.Getenv("CADDY_CONF_DIR"), os.Getenv("CADDY_INTERFACE_NAME"))
	if util.FileExists(path) {
		os.Remove(path)
	}

	for _, Services := range Services.Services {
		_, err := template.CaddyConfigTempl(Services)
		if err != nil {
			util.LogError("Caddy update error: ", err)
			return err
		}
	}

	return nil
}

func SaveToFile(updatedJSON []byte) error {
	// Write the updated configuration back to the file
	caddyConfigPath := filepath.Join(os.Getenv("CADDY_CONF_DIR"), "caddy.json")

	fmt.Println("caddyConfigPath : ", caddyConfigPath)
	err := util.WriteFile(caddyConfigPath, updatedJSON)
	if err != nil {
		util.LogError("File write error: ", err)
		return err
	}

	// Write the updated configuration to the SERVICE_CONF_DIR in $HOME
	homeDir, err := os.UserHomeDir()
	if err != nil {
		util.LogError("Error getting home directory: ", err)
		return err
	}

	serviceConfDir := filepath.Join(homeDir, os.Getenv("SERVICE_CONF_DIR"))
	err = os.MkdirAll(serviceConfDir, 0755) // Ensure the directory exists
	if err != nil {
		util.LogError("Error creating SERVICE_CONF_DIR: ", err)
		return err
	}

	serviceConfigPath := filepath.Join(serviceConfDir, "caddy.json")
	fmt.Println("serviceConfigPath: ", serviceConfigPath)
	err = util.WriteFile(serviceConfigPath, updatedJSON)
	if err != nil {
		util.LogError("File write error for SERVICE_CONF_DIR: ", err)
		return err
	}
	return nil
}
