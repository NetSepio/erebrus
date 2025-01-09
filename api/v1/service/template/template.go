package template

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/NetSepio/erebrus/core"
	"github.com/NetSepio/erebrus/model"
)

var (
	caddyTpl = `
# {{.Name}}, {{.Port}}, {{.CreatedAt}}
{{.Name}}.{{.Domain}} {
	reverse_proxy / {{.IpAddress}}{{.Port}}
	log {
		output file /var/log/caddy/{{.Name}}.{{.Domain}}.access.log {
			roll_size 3MiB
			roll_keep 5
			roll_keep_for 48h
		}
		format console
	}
	encode gzip zstd

	tls support@netsepio.com {
		protocols tls1.2 tls1.3
	}
}
`
	nginxTpl = `
# {{.Name}}, {{.Port}}, {{.CreatedAt}}
server {
	listen 6000;
	server_name {{.Name}}.{{.Domain}};
	
	location / {
		proxy_pass http://127.0.0.1:{{.Port}}$request_uri;
		proxy_set_header X-Real-IP $remote_addr;
		proxy_set_header Host $host;
		proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
		proxy_buffering off;
		proxy_redirect off;
	}
}
`
)

// Caddy configuration file template
func CaddyConfigTempl(tunnel model.Service) ([]byte, error) {
	t, err := template.New("config").Parse(caddyTpl)
	if err != nil {
		return nil, err
	}

	var tplBuff bytes.Buffer
	err = t.Execute(&tplBuff, tunnel)
	if err != nil {
		return nil, err
	}

	// Get the directory path and ensure it exists
	configDir := os.Getenv("CADDY_CONF_DIR")
	if configDir == "" {
		return nil, fmt.Errorf("CADDY_CONF_DIR environment variable is not set")
	}

	configDir += os.Getenv("CADDY_INTERFACE_NAME")

	// Ensure the directory exists
	err = os.MkdirAll(configDir, 0755) // 0755 for read/write/execute permissions
	if err != nil {
		return nil, fmt.Errorf("error creating directory %s: %w", configDir, err)
	}

	// Write the file
	configFilePath := filepath.Join(configDir, os.Getenv("CADDY_INTERFACE_NAME"))
	err = core.Writefile(configFilePath, tplBuff.Bytes())
	if err != nil {
		return nil, fmt.Errorf("error writing file %s: %w", configFilePath, err)
	}

	return tplBuff.Bytes(), nil
}

// Nginx configuration file template
func NginxConfigTempl(tunnel model.Service) ([]byte, error) {
	t, err := template.New("config").Parse(nginxTpl)
	if err != nil {
		return nil, err
	}

	var tplBuff bytes.Buffer
	err = t.Execute(&tplBuff, tunnel)
	if err != nil {
		return nil, err
	}

	err = core.Writefile(filepath.Join(os.Getenv("NGINX_CONF_DIR"), os.Getenv("NGINX_INTERFACE_NAME")), tplBuff.Bytes())
	if err != nil {
		return nil, err
	}

	return tplBuff.Bytes(), nil
}
