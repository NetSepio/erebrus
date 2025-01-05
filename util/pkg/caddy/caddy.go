package caddy

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
)

func isCaddyInstalled() bool {
	_, err := exec.LookPath("caddy")
	return err == nil
}

func LinuxBasicInstallation() {

	// Check if 'sh' is already installed
	cmdCheck := exec.Command("sh", "--version")
	err := cmdCheck.Run()

	if err == nil {
		// If no error, 'sh' is already installed
		fmt.Println(" ✅ 'sh' is already installed! ✅")
		return
	}

	// If 'sh' is not installed, install 'dash' using apt package manager
	cmdInstall := exec.Command("sudo", "apt-get", "install", "-y", "dash")

	// Run the command
	err = cmdInstall.Run()
	if err != nil {
		// Return the error with the custom message format
		log.Fatalf(" ❌ Failed to install 'sh': %v ❌", err)
		return
	}

	// Success message in the desired format
	fmt.Println(" ✅ 'sh' installation complete! ✅")

}

func installCaddy() error {
	osType := runtime.GOOS
	var installCmd *exec.Cmd

	switch osType {
	case "linux":
		installCmd = exec.Command("sh", "-c", `
			 🐧 Installing Caddy on Linux... Please wait... ⏳  && \
			sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https && \
			curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg && \
			curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list && \
			sudo apt update && \
			sudo apt install caddy
		`)
	case "darwin":
		// installCmd = exec.Command("sh", "-c", `brew install caddy`)
		fmt.Println(" 🍏 Installing Caddy on macOS... Please wait. ⏳ ")

		// Install Homebrew if not installed
		_, err := exec.LookPath("brew")
		if err != nil {
			log.Println(" ❌ Homebrew not found. Installing Homebrew... 🍺 ")
			if err := exec.Command("sh", "-c", `/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"`).Run(); err != nil {
				return fmt.Errorf(" ❌ Failed to install Homebrew: %w ❌ ", err)
			}
		}

		// Install Caddy using Homebrew
		if err := exec.Command("sh", "-c", "brew install caddy").Run(); err != nil {
			return fmt.Errorf(" ❌ Failed to install Caddy: %w ❌ ", err)
		} else {
			fmt.Println(" 🎉 Caddy installed successfully! 🎉 ")
			if err := exec.Command("sh", "-c", "brew services start caddy").Run(); err != nil {
				return fmt.Errorf("failed to start Caddy service: %w", err)
			} else {
				fmt.Println(" 🚀 Caddy service started successfully! 🚀 ")
			}
		}

		return nil
	case "windows":
		installCmd = exec.Command("powershell", "-Command", `Invoke-WebRequest -Uri https://github.com/caddyserver/caddy/releases/latest/download/caddy_windows_amd64.zip -OutFile caddy.zip; \
		Expand-Archive -Path caddy.zip -DestinationPath .; \
		Move-Item -Path .\caddy.exe -Destination $env:ProgramFiles\caddy; \
		$env:Path += \";$env:ProgramFiles\\caddy\"`)
	default:
		return fmt.Errorf(" ❌ Unsupported operating system: %s ❌ ", osType)
	}

	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return err
	}

	return startCaddyService()
}

func startCaddyService() error {
	osType := runtime.GOOS
	var startCmd *exec.Cmd

	switch osType {
	case "linux":
		startCmd = exec.Command("sh", "-c", `sudo systemctl start caddy && sudo systemctl enable caddy`)
	case "darwin":
		startCmd = exec.Command("sh", "-c", `brew services start caddy`)
	case "windows":
		startCmd = exec.Command("powershell", "-Command", `Start-Process -FilePath caddy.exe -NoNewWindow`)
	default:
		return fmt.Errorf(" ❌ Unsupported operating system: %s ❌ ", osType)
	}

	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	return startCmd.Run()
}

func InstallCaddy() {
	nodeConfig := os.Getenv("NODE_CONFIG")
	fmt.Println(" NODE_CONFIG:", nodeConfig, " ")

	if nodeConfig == "standard" || nodeConfig == "hpc" {
		if isCaddyInstalled() {
			fmt.Println(" 🔄 Caddy is already installed. Removing the older version... 🔄 ")
			if err := removeCaddy(); err != nil {
				fmt.Printf(" ❌ Failed to remove Caddy: %v ❌ \n", err)
				return
			} else {
				fmt.Println(" ✅ Caddy removed successfully. Installing the latest version now... ⚡ ")
				if err := installCaddy(); err != nil {
					fmt.Printf(" ❌ Failed to install Caddy: %v ❌ \n", err)
					return
				}
			}
			return
		} else {
			fmt.Println(" 🚫 Caddy is not installed. Installing now... Please wait. ⏳ ")

			if err := installCaddy(); err != nil {
				fmt.Printf(" ❌ Failed to install Caddy: %v ❌ \n", err)
				return
			}
		}

		fmt.Println(" 🚀 Caddy has been successfully installed and started! 🎉 ")
	}
}

func removeCaddy() error {
	switch runtime.GOOS {
	case "linux":
		return uninstallCaddyInUbuntu()

	case "darwin":
		fmt.Println(" 🍏 Removing Caddy for macOS... 🧹 ")
		// Use Homebrew to uninstall Caddy
		return removeCaddyByBrew()

	case "windows":
		fmt.Println(" 💻 Removing Caddy for Windows... 🧹 ")
		// Remove the Caddy executable for Windows
		cmd := exec.Command("cmd", "/C", "del", "%ProgramFiles%\\caddy\\caddy.exe") // Adjust path if needed
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	default:
		return fmt.Errorf(" ❌ Unsupported OS: %s ❌ ", runtime.GOOS)
	}
}

// executeCommand runs the given shell command and prints the output or error.
func executeCommand(cmd string, args ...string) error {
	// Prepare the command
	command := exec.Command(cmd, args...)
	_, err := command.CombinedOutput()
	if err != nil {
		log.Fatalf(" ❌ Error executing command %s %v: %s ❌ \n", cmd, args, err)
		return err
	}
	// fmt.Printf(" Output: %s \n", output)
	return nil
}

// stopAndUninstallCaddy executes the commands to stop and uninstall Caddy.
func removeCaddyByBrew() error {
	var err error
	err = executeCommand("brew", "services", "stop", "caddy")
	if err != nil {
		log.Fatalf(" ❌ Error stopping Caddy service: %v ❌ \n", err)
	} else {
		fmt.Println(" ✅ Caddy service stopped successfully. ✅ ")
	}
	err = executeCommand("brew", "uninstall", "--force", "caddy")
	if err != nil {
		log.Fatalf(" ❌ Error uninstalling Caddy: %v ❌ \n", err)
	} else {
		fmt.Println(" ✅ Caddy uninstalled successfully. ✅ ")
	}
	err = executeCommand("rm", "-rf", "/usr/local/etc/caddy")
	if err != nil {
		log.Fatalf(" ❌ Error removing Caddy configuration: %v ❌ \n", err)
	} else {
		fmt.Println(" ✅ Caddy configuration removed successfully. ✅ ")
	}
	err = executeCommand("rm", "-rf", "/usr/local/var/log/caddy")
	if err != nil {
		log.Fatalf(" ❌ Error removing Caddy logs: %v ❌ \n", err)
	} else {
		fmt.Println(" ✅ Caddy logs removed successfully. ✅ ")
	}
	return nil
}

// runCommand executes a system command and prints output or error.
func runCommand(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// uninstallCaddy performs the uninstallation of Caddy from the system.
func uninstallCaddyInUbuntu() error {
	// Step 1: Stop Caddy service
	fmt.Println(" 🛑 Stopping Caddy service... Please wait. ⏳")
	err := runCommand("sudo", "systemctl", "stop", "caddy")
	if err != nil {
		log.Fatalf(" ❌ Failed to stop Caddy service: %v ❌\n", err)
		return err
	} else {
		fmt.Println(" ✅ Caddy service stopped successfully. ✅ ")
	}

	// Step 2: Disable Caddy service from starting on boot
	fmt.Println(" 📴 Disabling Caddy service from starting on boot... ⏳")
	err = runCommand("sudo", "systemctl", "disable", "caddy")
	if err != nil {
		log.Fatalf(" ❌ Failed to disable Caddy service: %v ❌\n", err)
		return err
	} else {
		fmt.Println(" ✅ Caddy service disabled successfully. ✅ ")
	}

	// Step 3: Remove Caddy binary
	fmt.Println(" 🗑️ Removing Caddy binary... ⏳")
	err = runCommand("sudo", "rm", "/usr/local/bin/caddy")
	if err != nil {
		log.Fatalf(" ❌ Failed to remove Caddy binary: %v ❌\n", err)
		return err
	} else {
		fmt.Println(" ✅ Caddy binary removed successfully. ✅ ")
	}

	// Step 4: Remove Caddy systemd service file
	fmt.Println(" 🗑️ Removing Caddy service file... ⏳")
	err = runCommand("sudo", "rm", "/etc/systemd/system/caddy.service")
	if err != nil {
		log.Fatalf(" ❌ Failed to remove Caddy service file: %v ❌\n", err)
		return err
	} else {
		fmt.Println(" ✅ Caddy service file removed successfully. ✅ ")
	}

	// Step 5: Remove Caddy configuration and data files
	fmt.Println(" 🗑️ Removing Caddy configuration and data files... ⏳")
	err = runCommand("sudo", "rm", "-rf", "/etc/caddy", "/var/lib/caddy")
	if err != nil {
		log.Fatalf(" ❌ Failed to remove Caddy configuration and data files: %v ❌\n", err)
		return err
	} else {
		fmt.Println(" ✅ Caddy configuration and data files removed successfully. ✅ ")
	}

	// Step 6: Remove Caddy logs
	fmt.Println(" 🗑️ Removing Caddy logs... ⏳")
	err = runCommand("sudo", "rm", "-rf", "/var/log/caddy")
	if err != nil {
		log.Fatalf(" ❌ Failed to remove Caddy logs: %v ❌\n", err)
		return err
	} else {
		fmt.Println(" ✅ Caddy logs removed successfully. ✅ ")
	}

	// Step 7: Optional cleanup of unnecessary dependencies
	fmt.Println(" 🧹 Running system cleanup... ⏳")
	err = runCommand("sudo", "apt-get", "autoremove", "-y")
	if err != nil {
		log.Fatalf(" ❌ Failed to autoremove dependencies: %v ❌\n", err)
		return err
	} else {
		fmt.Println(" ✅ System cleanup completed successfully. ✅ ")
	}

	// Final confirmation
	fmt.Println(" 🎉 Caddy has been successfully uninstalled from your system! 🎉")
	return nil
}
