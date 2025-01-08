package middleware

import (
	"encoding/json"
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
		return -1, "", err
	} else {
		for _, tunnel := range tunnels.Tunnels {
			if tunnel.Name == name {
				return -1, "Tunnel Already exists", err
			} else if tunnel.Port == strconv.Itoa(port) {
				return -1, "Port Already in use", err
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
	file, err := os.OpenFile(filepath.Join(os.Getenv("SEVICE_CONF_DIR"), "caddy.json"), os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		util.LogError("File Open error: ", err)
		return nil, err
	}

	b, err := io.ReadAll(file)
	if err != nil {
		util.LogError("File Read error: ", err)
		return nil, err
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
		return err
	}

	var updatedTunnels []model.Tunnel
	updatedTunnels = append(updatedTunnels, tunnels.Tunnels...)
	updatedTunnels = append(updatedTunnels, tunnel)

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
