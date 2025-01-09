package middleware

import (
	"encoding/json"
	"errors"
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
	// check if the name is empty
	if name == "" {
		return -1, "Services Name is required", nil
	}

	// check the name field is between 3 to 40 chars
	if len(name) < 4 || len(name) > 12 {
		return -1, "Services Name field must be between 4-12 chars", nil
	}

	// check if name or port is already in use
	Services, err := ReadWebServices()
	if err != nil {
		if err.Error() == "caddy file is empty while reading file" {
			util.LogError("Caddy file is empty, proceeding to create a new Services", nil)
		} else {
			return -1, "", err
		}
	}

	if Services != nil {
		for _, Services := range Services.Services {
			if Services.Name == name {
				return -1, "Services Already exists", nil
			} else if Services.Port == strconv.Itoa(port) {
				return -1, "Port Already in use", nil
			}
		}
	}

	// check the format of name
	if !util.IsLetter(name) {
		return -1, "Services Name should be Aplhanumeric", nil
	}

	return 1, "", nil
}

// ReadWebTunnels fetches all the Web Tunnel
func ReadWebServices() (*model.Services, error) {

	// Get the home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		util.LogError("Unable to get home directory: ", err)
		return nil, err
	}

	filePath := filepath.Join(homeDir, os.Getenv("SERVICE_CONF_DIR"), "caddy.json")

	// file, err := os.OpenFile(filepath.Join(os.Getenv("SERVICE_CONF_DIR"), "caddy.json"), os.O_RDWR|os.O_APPEND, 0666)
	// if err != nil {
	// 	util.LogError("File Open error: ", err)
	// 	return nil, err
	// }

	// Check if the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Create the file if it doesn't exist
		file, err := os.Create(filePath)
		if err != nil {
			util.LogError("File creation error: ", err)
			return nil, err
		}
		defer file.Close() // Ensure the file is closed after creation
	}

	// Open the file
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		util.LogError("File Open error: ", err)
		return nil, err
	}
	defer file.Close() // Ensure the file is closed in any case

	b, err := io.ReadAll(file)
	if err != nil {
		util.LogError("File Read error: ", err)
		return nil, err
	}

	// Check if the file is empty
	if len(b) == 0 {
		util.LogError("Caddy file is empty", err)
		return nil, errors.New("caddy file is empty while reading file")
	}

	var Services model.Services
	err = json.Unmarshal(b, &Services.Services)
	if err != nil {
		util.LogError("Unmarshal json error: ", err)
		return nil, err
	}

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
