package agents

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
)

func EnsureDockerAndCaddy() {
	// Check and install Docker
	if !isCommandAvailable("docker") {
		log.Println("Docker is not installed. Installing Docker...")
		err := installDocker()
		if err != nil {
			log.Fatalf("Failed to install Docker: %v", err)
		}
		log.Println("Docker installed successfully.")
	} else {
		log.Println("Docker is already installed.")
	}

	// Test Docker functionality
	log.Println("Testing Docker functionality...")
	testDocker()

	// Check and install Caddy
	if !isCommandAvailable("caddy") {
		log.Println("Caddy is not installed. Installing Caddy...")
		err := installCaddy()
		if err != nil {
			log.Fatalf("Failed to install Caddy: %v", err)
		}
		log.Println("Caddy installed successfully.")
	} else {
		log.Println("Caddy is already installed.")
	}

	// Start Caddy
	log.Println("Starting Caddy")
	if err := runCommand(exec.Command("systemctl", "restart", "caddy")); err != nil {
		log.Fatalf("Failed to restart caddy: %v", err)
	} else {
		log.Println("Caddy Started Successfully")
	}

}

// Check if a command is available
func isCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// Install Docker
func installDocker() error {
	cmd := exec.Command("sh", "-c", `
		curl -fsSL https://get.docker.com | sh
	`)
	if err := runCommand(cmd); err != nil {
		return err
	}

	// Enable and start Docker service
	log.Println("Enabling and starting Docker service...")
	enableCmd := exec.Command("systemctl", "enable", "--now", "docker")
	if err := runCommand(enableCmd); err != nil {
		return fmt.Errorf("failed to enable/start Docker: %w", err)
	}

	log.Println("Docker service enabled and started successfully.")
	return nil
}

// Install Caddy
func installCaddy() error {
	cmd := exec.Command("sh", "-c", `
		apt-get update -qq &&
		apt-get install -y debian-keyring debian-archive-keyring apt-transport-https &&
		curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/gpg.key | gpg --dearmor -o /usr/share/keyrings/caddy-archive-keyring.gpg &&
		echo "deb [signed-by=/usr/share/keyrings/caddy-archive-keyring.gpg] https://dl.cloudsmith.io/public/caddy/stable/deb/debian all main" > /etc/apt/sources.list.d/caddy-stable.list &&
		apt-get update -qq &&
		apt-get install -y caddy
	`)
	return runCommand(cmd)
}

// Test Docker functionality
func testDocker() {
	log.Println("Pulling Alpine image...")
	if err := runCommand(exec.Command("systemctl", "enable", "docker")); err != nil {
		log.Fatalf("Failed to enable docker: %v", err)
	}
	if err := runCommand(exec.Command("systemctl", "restart", "docker")); err != nil {
		log.Fatalf("Failed to restart docker: %v", err)
	}
	log.Println("Successfully restarted Docker")
	if err := runCommand(exec.Command("docker", "pull", "alpine")); err != nil {
		log.Fatalf("Failed to pull Alpine image: %v", err)
	}
	log.Println("Successfully pulled Alpine image.")

	log.Println("Running Alpine container...")
	if err := runCommand(exec.Command("docker", "run", "--name", "alpine-test", "-d", "alpine", "sleep", "10")); err != nil {
		log.Fatalf("Failed to run Alpine container: %v", err)
	}
	log.Println("Successfully ran Alpine container.")

	log.Println("Deleting Alpine container...")
	if err := runCommand(exec.Command("docker", "rm", "-f", "alpine-test")); err != nil {
		log.Fatalf("Failed to delete Alpine container: %v", err)
	}
	log.Println("Successfully deleted Alpine container.")
}

// Helper function to run commands and capture output
func runCommand(cmd *exec.Cmd) error {
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("Command failed: %s\nOutput: %s\nError: %s", cmd.String(), out.String(), stderr.String())
		return err
	}
	log.Printf("Command succeeded: %s\nOutput: %s", cmd.String(), out.String())
	return nil
}
