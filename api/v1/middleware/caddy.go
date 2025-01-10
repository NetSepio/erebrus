package middleware

import (
	"encoding/json"
	"errors"
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
func IsValidWeb(name string, port int) (int, string, error) {
	// Check if the name is empty
	fmt.Printf("Checking service name: %s, port: %d\n", name, port)
	if name == "" {
		fmt.Println("Service name is empty")
		return -1, "Services Name is required", nil
	}

	// Check the name field length
	fmt.Printf("Service name length: %d\n", len(name))
	if len(name) < 4 || len(name) > 12 {
		fmt.Println("Service name length is invalid")
		return -1, "Services Name field must be between 4-12 chars", nil
	}

	// Read existing services
	fmt.Println("Reading web services...")
	Services, err := ReadWebServices()
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
				return -1, "Services Already exists", nil
			} else if service.Port == strconv.Itoa(port) {
				fmt.Println("Port is already in use")
				return -1, "Port Already in use", nil
			}
		}
	}

	// Validate the format of the name
	if !util.IsLetter(name) {
		fmt.Println("Service name is not alphanumeric")
		return -1, "Services Name should be Alphanumeric", nil
	}

	fmt.Println("Service name and port are valid")
	return 1, "", nil
}


// ReadWebTunnels fetches all the Web Tunnel
func ReadWebServices() (*model.Services, error) {

	// Get the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Unable to get home directory: %v\n", err)
		return nil, err
	}
	fmt.Printf("Home directory: %s\n", homeDir)

	// Construct the file path
	filePath := filepath.Join(homeDir, os.Getenv("SERVICE_CONF_DIR"), "caddy.json")
	fmt.Printf("File path: %s\n", filePath)

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		fmt.Println("File does not exist, creating a new file")
		// Create the file if it doesn't exist
		file, err := os.Create(filePath)
		if err != nil {
			fmt.Printf("File creation error: %v\n", err)
			return nil, err
		}
		fmt.Println("File created successfully")
		defer file.Close() // Ensure the file is closed after creation
	}

	// Open the file
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("File open error: %v\n", err)
		return nil, err
	}
	defer file.Close() // Ensure the file is closed in any case
	fmt.Println("File opened successfully")

	// Read the file content
	b, err := io.ReadAll(file)
	if err != nil {
		fmt.Printf("File read error: %v\n", err)
		return nil, err
	}
	fmt.Printf("File content (bytes): %v\n", b)
	fmt.Printf("File content (string): %s\n", string(b))

	// Check if the file is empty
	if len(b) == 0 {
		fmt.Println("Caddy file is empty while reading file")
		return nil, errors.New("caddy file is empty while reading file")
	}

	// Parse the file content
	var Services model.Services
	fmt.Println("Unmarshalling JSON content into Services struct...")
	err = json.Unmarshal(b, &Services.Services)
	if err != nil {
		fmt.Printf("JSON unmarshal error: %v\n", err)
		return nil, err
	}
	fmt.Printf("Unmarshalled Services struct: %+v\n", Services)

	return &Services, nil
}

// ReadWebTunnel fetches a Web Tunnel
func ReadWebService(tunnelName string) (*model.Service, error) {
	Services, err := ReadWebServices()
	if err != nil {
		return nil, err
	}

	var data model.Service
	for _, Services := range Services.Services {
		if Services.Name == tunnelName {
			data.Name = Services.Name
			data.Port = Services.Port
			data.CreatedAt = Services.CreatedAt
			data.Domain = Services.Domain
			data.Status = Services.Status
			break
		}
	}

	return &data, nil
}

// AddWebServices creates a Web Services
func AddWebServices(Services model.Service) error {
	servicesList, err := ReadWebServices()
	if err != nil {
		if err.Error() == "caddy file is empty while reading file" {
			util.LogError("Caddy file is empty, proceeding to create a new Services", nil)
			servicesList = &model.Services{Services: []model.Service{}} // Initialize an empty Services struct
		} else {
			return err
		}
	}

	if servicesList == nil || servicesList.Services == nil {
		servicesList = &model.Services{Services: []model.Service{}} // Ensure Services is initialized
	}

	// Prepare updated Services list
	updatedServices := append(servicesList.Services, Services)

	// Marshal the updated Services to JSON
	inter, err := json.MarshalIndent(updatedServices, "", "   ")
	if err != nil {
		util.LogError("JSON Marshal error: ", err)
		return err
	}

	// Write the updated configuration to the file
	caddyConfigPath := filepath.Join(os.Getenv("SERVICE_CONF_DIR"), "caddy.json")
	err = util.WriteFile(caddyConfigPath, inter)
	if err != nil {
		util.LogError("File write error: ", err)
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

// DeleteWebServices deletes a Web Services
func DeleteWebServices(ServicesName string) error {
	Services, err := ReadWebServices()
	if err != nil {
		return err
	}

	var updatedServices []model.Service
	for _, Service := range Services.Services {
		if Service.Name == ServicesName {
			continue
		}
		updatedServices = append(updatedServices, Service)
	}

	inter, err := json.MarshalIndent(updatedServices, "", "   ")
	if err != nil {
		util.LogError("JSON Marshal error: ", err)
		return err
	}

	err = util.WriteFile(filepath.Join(os.Getenv("SERVICE_CONF_DIR"), "caddy.json"), inter)
	if err != nil {
		util.LogError("File write error: ", err)
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
	Services, err := ReadWebServices()
	if err != nil {
		return err
	}

	path := filepath.Join(os.Getenv("WG_CONF_DIR"), os.Getenv("CADDY_INTERFACE_NAME"))
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
