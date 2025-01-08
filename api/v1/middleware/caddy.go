package middleware

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/NetSepio/erebrus/api/v1/tunnel/template"
	"github.com/NetSepio/erebrus/api/v1/tunnel/util"
	"github.com/NetSepio/erebrus/model"
)

// IsValid check if model is valid
func IsValidWeb(name string, port int) (int, string, error) {
	// check if the name is empty
	if name == "" {
		return -1, "Tunnel Name is required", nil
	}

	// check the name field is between 3 to 40 chars
	if len(name) < 4 || len(name) > 12 {
		return -1, "Tunnel Name field must be between 4-12 chars", nil
	}

	// check if name or port is already in use
	tunnels, err := ReadWebTunnels()
	if err != nil {
		if err.Error() == "caddy file is empty while reading file" {
			util.LogError("Caddy file is empty, proceeding to create a new tunnel", nil)
		} else {
			return -1, "", err
		}
	}

	if tunnels != nil {
		for _, tunnel := range tunnels.Tunnels {
			if tunnel.Name == name {
				return -1, "Tunnel Already exists", nil
			} else if tunnel.Port == strconv.Itoa(port) {
				return -1, "Port Already in use", nil
			}
		}
	}

	// check the format of name
	if !util.IsLetter(name) {
		return -1, "Tunnel Name should be Aplhanumeric", nil
	}

	return 1, "", nil
}

// ReadWebTunnels fetches all the Web Tunnel
func ReadWebTunnels() (*model.Tunnels, error) {

	// filePath := filepath.Join(os.Getenv("SERVICE_CONF_DIR"), "caddy.json")

	file, err := os.OpenFile(filepath.Join(os.Getenv("SEVICE_CONF_DIR"), "caddy.json"), os.O_RDWR|os.O_APPEND, 0666)
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

	var tunnels model.Tunnels
	err = json.Unmarshal(b, &tunnels.Tunnels)
	if err != nil {
		util.LogError("Unmarshal json error: ", err)
		return nil, err
	}

	return &tunnels, nil
}

// ReadWebTunnel fetches a Web Tunnel
func ReadWebTunnel(tunnelName string) (*model.Tunnel, error) {
	tunnels, err := ReadWebTunnels()
	if err != nil {
		return nil, err
	}

	var data model.Tunnel
	for _, tunnel := range tunnels.Tunnels {
		if tunnel.Name == tunnelName {
			data.Name = tunnel.Name
			data.Port = tunnel.Port
			data.CreatedAt = tunnel.CreatedAt
			data.Domain = tunnel.Domain
			data.Status = tunnel.Status
			break
		}
	}

	return &data, nil
}

// AddWebTunnel creates a Web Tunnel
func AddWebTunnel(tunnel model.Tunnel) error {
	tunnels, err := ReadWebTunnels()
	if err != nil {
		if err.Error() == "caddy file is empty while reading file" {
			util.LogError("Caddy file is empty, proceeding to create a new tunnel", nil)
			tunnels = &model.Tunnels{Tunnels: []model.Tunnel{}} // Initialize an empty Tunnels struct
		} else {
			return err
		}
	}

	if tunnels == nil || tunnels.Tunnels == nil {
		tunnels = &model.Tunnels{Tunnels: []model.Tunnel{}} // Ensure tunnels is initialized
	}

	// Prepare updated tunnels list
	updatedTunnels := append(tunnels.Tunnels, tunnel)

	// Marshal the updated tunnels to JSON
	inter, err := json.MarshalIndent(updatedTunnels, "", "   ")
	if err != nil {
		util.LogError("JSON Marshal error: ", err)
		return err
	}

	// Write the updated configuration to the file
	caddyConfigPath := filepath.Join(os.Getenv("SEVICE_CONF_DIR"), "caddy.json")
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

// DeleteWebTunnel deletes a Web Tunnel
func DeleteWebTunnel(tunnelName string) error {
	tunnels, err := ReadWebTunnels()
	if err != nil {
		return err
	}

	var updatedTunnels []model.Tunnel
	for _, tunnel := range tunnels.Tunnels {
		if tunnel.Name == tunnelName {
			continue
		}
		updatedTunnels = append(updatedTunnels, tunnel)
	}

	inter, err := json.MarshalIndent(updatedTunnels, "", "   ")
	if err != nil {
		util.LogError("JSON Marshal error: ", err)
		return err
	}

	err = util.WriteFile(filepath.Join(os.Getenv("SEVICE_CONF_DIR"), "caddy.json"), inter)
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
	tunnels, err := ReadWebTunnels()
	if err != nil {
		return err
	}

	path := filepath.Join(os.Getenv("WG_CONF_DIR"), os.Getenv("CADDY_INTERFACE_NAME"))
	if util.FileExists(path) {
		os.Remove(path)
	}

	for _, tunnel := range tunnels.Tunnels {
		_, err := template.CaddyConfigTempl(tunnel)
		if err != nil {
			util.LogError("Caddy update error: ", err)
			return err
		}
	}

	return nil
}
