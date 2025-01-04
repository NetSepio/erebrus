package caddy

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func isCaddyInstalled() bool {
	_, err := exec.LookPath("caddy")
	return err == nil
}

func isCaddyRunning() bool {
	osType := runtime.GOOS
	var checkCmd *exec.Cmd

	switch osType {
	case "linux", "darwin":
		checkCmd = exec.Command("pgrep", "caddy")
	case "windows":
		checkCmd = exec.Command("tasklist", "/FI", "IMAGENAME eq caddy.exe")
	default:
		return false
	}

	err := checkCmd.Run()
	return err == nil
}

func installCaddy() error {
	osType := runtime.GOOS
	var installCmd *exec.Cmd

	switch osType {
	case "linux":
		installCmd = exec.Command("sh", "-c", `
			sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https && \
			curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg && \
			curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list && \
			sudo apt update && \
			sudo apt install caddy
		`)
	case "darwin":
		installCmd = exec.Command("sh", "-c", `brew install caddy`)
	case "windows":
		installCmd = exec.Command("powershell", "-Command", `Invoke-WebRequest -Uri https://github.com/caddyserver/caddy/releases/latest/download/caddy_windows_amd64.zip -OutFile caddy.zip; \
		Expand-Archive -Path caddy.zip -DestinationPath .; \
		Move-Item -Path .\caddy.exe -Destination $env:ProgramFiles\caddy; \
		$env:Path += \";$env:ProgramFiles\\caddy\"`)
	default:
		return fmt.Errorf("unsupported operating system: %s", osType)
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
		return fmt.Errorf("unsupported operating system: %s", osType)
	}

	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	return startCmd.Run()
}

func InstallCaddy() {

	nodeConfig := os.Getenv("NODE_CONFIG")
	fmt.Println("NODE_CONFIG", nodeConfig)

	if nodeConfig == "standard" || nodeConfig == "hpc" {
		if isCaddyInstalled() {
			fmt.Println("Caddy is already installed removing the older version")
			if err := removeCaddy(); err != nil {
				fmt.Printf("Failed to remove Caddy: %v\n", err)
				return
			} else {
				fmt.Println("Caddy removed successfully!")
				installCaddy()
			}
			return
		}

		fmt.Println("Caddy is not installed. Installing now...")

		if err := installCaddy(); err != nil {
			fmt.Printf("Failed to install Caddy: %v\n", err)
			return
		}

		fmt.Println("Caddy installed and started successfully!")
	}

}

// func removeCaddy() error {
// 	switch runtime.GOOS {
// 	case "linux", "darwin": // Linux and macOS
// 		fmt.Println("Removing Caddy for", runtime.GOOS)
// 		// Remove the Caddy binary
// 		cmd := exec.Command("sudo", "rm", "-f", "/usr/bin/caddy") // Linux and macOS use similar paths
// 		if runtime.GOOS == "darwin" {
// 			cmd = exec.Command("sudo", "rm", "-f", "/usr/local/bin/caddy") // macOS-specific path
// 		}
// 		return cmd.Run()
// 	case "windows":
// 		fmt.Println("Removing Caddy for Windows")
// 		// Remove the Caddy executable for Windows
// 		cmd := exec.Command("cmd", "/C", "del", "C:\\Caddy\\caddy.exe") // Adjust path if needed
// 		return cmd.Run()
// 	default:
// 		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
// 	}
// }

func removeCaddy() error {
	switch runtime.GOOS {
	case "linux":
		fmt.Println("Removing Caddy for Linux")
		// Remove the Caddy binary
		cmd := exec.Command("sudo", "rm", "-f", "/usr/bin/caddy")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case "darwin":
		fmt.Println("Removing Caddy for macOS")
		// Use Homebrew to uninstall Caddy
		cmd := exec.Command("brew", "uninstall", "caddy")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	case "windows":
		fmt.Println("Removing Caddy for Windows")
		// Remove the Caddy executable for Windows
		cmd := exec.Command("cmd", "/C", "del", "%ProgramFiles%\\caddy\\caddy.exe") // Adjust path if needed
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()

	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}
