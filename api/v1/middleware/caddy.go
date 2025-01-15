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
	if len(name) < 4 || len(name) > 12 {
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
	if !util.IsLetter(name) {
		fmt.Println("Service name is not alphanumeric")
		return -1, "Services Name should be Alphanumeric", nil
	}

	fmt.Println("Service name and port are valid")
	return 1, "", nil
}

// ReadWebTunnels fetches all the Web Tunnel
func ReadServices() (*model.ServicesList, error) {

	filePath := filepath.Join(os.Getenv("CADDY_CONF_DIR"), "caddy.json")

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		file, err := os.Create(filePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		file.WriteString(`{"services": []}`) // Initialize with empty JSON structure
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		fmt.Println("Caddy file is empty while reading file", &model.ServicesList{Services: []model.Service{}})
		return &model.ServicesList{Services: []model.Service{}}, nil
	}

	var Services model.ServicesList

	err = json.Unmarshal(b, &Services)
	if err != nil {
		return nil, err
	}

	return &Services, nil
}

// ReadWebTunnel fetches a Web Tunnel
func ReadWebService(tunnelName string) (*model.Service, error) {
	Services, err := ReadServices()
	if err != nil {
		return nil, err
	}

	// print all the services
	fmt.Println()
	fmt.Println("Services: ")
	fmt.Printf("%+v\n", Services)
	fmt.Println()

	var data model.Service
	for _, Service := range Services.Services {
		// print all the services
		fmt.Println()
		fmt.Println("Services: ")
		fmt.Printf("%+v\n", Service)
		fmt.Println()
		fmt.Println("tunnel Name: ", tunnelName)
		fmt.Println()
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

func AddWebServices(newService model.Service) error {
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

	// Write the updated configuration back to the file
	caddyConfigPath := filepath.Join(os.Getenv("CADDY_CONF_DIR"), "caddy.json")

	err = util.WriteFile(caddyConfigPath, updatedJSON)
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

func DeleteWebService(serviceName string) error {
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

	err = util.WriteFile(filepath.Join(os.Getenv("CADDY_CONF_DIR"), "caddy.json"), jsonData)
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
	Services, err := ReadServices()
	if err != nil {
		return err
	}

	path := filepath.Join(os.Getenv("CADDY_HOME"), os.Getenv("CADDY_CONF_DIR"), os.Getenv("CADDY_INTERFACE_NAME"))
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
