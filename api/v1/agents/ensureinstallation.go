package agents

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
)

func EnsureDockerAndCaddy() {
	// Check and install Docker
	// if !isCommandAvailable("docker") {
	// 	log.Println("Docker is not installed. Installing Docker...")
	// 	err := installDocker()
	// 	if err != nil {
	// 		log.Fatalf("Failed to install Docker: %v", err)
	// 	}
	// 	log.Println("Docker installed successfully.")
	// } else {
	// 	log.Println("Docker is already installed.")
	// }

	// // Test Docker functionality
	// log.Println("Testing Docker functionality...")
	// testDocker()

	// Check and install Caddy
	if !isCommandAvailable("caddy") {
		log.Println("Caddy is not installed. Installing Caddy...")
		if err := installCaddy(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		log.Println("Caddy installed successfully.")
	} else {
		log.Println("Caddy is already installed.")
	}

	// Start Caddy
	log.Println("Starting Caddy server...")
	startCaddy()
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
// installCaddy installs Caddy on a Linux system
func installCaddy() error {
	fmt.Println("Installing Caddy manually...")

	// Download the latest Caddy binary
	fmt.Println("Downloading Caddy binary...")
	cmd := exec.Command("curl", "-fsSL", "https://caddyserver.com/api/download?os=linux&arch=amd64", "-o", "caddy.tar.gz")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to download Caddy: %v", err)
	}

	// Extract the binary
	fmt.Println("Extracting Caddy binary...")
	cmd = exec.Command("tar", "-xzf", "caddy.tar.gz")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to extract Caddy: %v", err)
	}

	// Move Caddy binary to /usr/local/bin
	fmt.Println("Moving Caddy binary to /usr/local/bin...")
	cmd = exec.Command("sudo", "mv", "caddy", "/usr/local/bin/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to move Caddy binary: %v", err)
	}

	// Give execution permission
	fmt.Println("Giving execution permission to Caddy binary...")
	cmd = exec.Command("sudo", "chmod", "+x", "/usr/local/bin/caddy")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set execute permission: %v", err)
	}

	// Verify installation
	fmt.Println("Verifying Caddy installation...")
	cmd = exec.Command("caddy", "version")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Caddy installation verification failed: %v", err)
	}

	fmt.Println("Caddy installed successfully!")
	return nil
}

// Test Docker functionality
func testDocker() {
	log.Println("Pulling Alpine image...")
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

// Start Caddy
func startCaddy() {
	cmd := exec.Command("caddy", "run")
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start Caddy: %v", err)
	}
	log.Println("Caddy server started successfully.")
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
