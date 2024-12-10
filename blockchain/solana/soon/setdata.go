package soon_solana

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/joho/godotenv"
)

type NodeDetails struct {
	PrivateKey string `json:"private_key"` // Private key in base58 format
	Did        string `json:"peaq_did"`    // DID (Decentralized Identifier)
	NodeName   string `json:"nodename"`    // Name of the node
	IPAddress  string `json:"ipaddress"`   // IP address of the node
	ISPInfo    string `json:"ispinfo"`     // Information about the ISP
	Region     string `json:"region"`      // Region of the node
	Location   string `json:"location"`    // Geographical location of the node
}

func isNodeInstalled() bool {
	fmt.Println("checking node is installed ")

	// Check if Node.js is installed
	_, err := exec.LookPath("node")
	return err == nil
}

func isCommandAvailable(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

func installNodeAndNpm() error {
	fmt.Println("Node.js or npm is not installed. Installing...")
	switch runtime.GOOS {
	case "linux":
		// Install Node.js and npm using apt
		cmd := exec.Command("bash", "-c", "apt-get update && apt-get install -y nodejs npm")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "darwin":
		// For macOS, use Homebrew
		cmd := exec.Command("bash", "-c", "brew install node")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "windows":
		fmt.Println("Please install Node.js manually from https://nodejs.org/")
		return fmt.Errorf("manual installation required on Windows")
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func installBuildEssentials() error {
	fmt.Println("Build essentials are not installed. Installing...")

	switch runtime.GOOS {
	case "windows":
		// Redirect user to install MinGW
		fmt.Println("Please install MinGW (Minimalist GNU for Windows) manually from https://sourceforge.net/projects/mingw/ or use a package manager like Scoop or Chocolatey.")
		return fmt.Errorf("manual installation required on Windows")
	case "linux":
		// Install build-essential using apt-get (Debian/Ubuntu)
		cmd := exec.Command("bash", "-c", "sudo apt-get update && sudo apt-get install -y build-essential")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "darwin":
		// Install Xcode Command Line Tools on macOS
		cmd := exec.Command("bash", "-c", "xcode-select --install")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func installNode() error {
	fmt.Println("Node.js is not installed. Installing...")

	switch runtime.GOOS {
	case "windows":
		// Redirect user to download Node.js
		fmt.Println("Please download Node.js from https://nodejs.org/ and install it manually.")
		return fmt.Errorf("manual installation required on Windows")
	case "linux":
		// Install Node.js using apt-get (Debian/Ubuntu)
		cmd := exec.Command("bash", "-c", "sudo apt-get update && sudo apt-get install -y nodejs")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "darwin":
		// Install Node.js using Homebrew on macOS
		cmd := exec.Command("bash", "-c", "brew install node")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func areBuildEssentialsInstalled() bool {
	fmt.Println("Checking if build essentials are installed...")

	switch runtime.GOOS {
	case "windows":
		// Check if MinGW is installed
		_, err := exec.LookPath("gcc")
		if err != nil {
			fmt.Println("gcc (MinGW) is not found.")
			return false
		}
		return true
	case "linux":
		// Check if gcc is installed on Linux
		_, err := exec.LookPath("gcc")
		if err != nil {
			fmt.Println("gcc is not found.")
			return false
		}
		return true
	case "darwin":
		// Check if gcc or clang is installed on macOS
		_, err := exec.LookPath("clang")
		if err != nil {
			fmt.Println("clang is not found.")
			return false
		}
		return true
	default:
		fmt.Printf("Unsupported operating system: %s\n", runtime.GOOS)
		return false
	}
}

func SoonNodeBlockchainCall(data NodeDetails) {

	// load env
	err := godotenv.Load()
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
		// Continue execution even if .env file is not found
	}

	if !areBuildEssentialsInstalled() {
		if err := installBuildEssentials(); err != nil {
			fmt.Println(err)
			return
		}

	}

	if !isNodeInstalled() {
		if err := installNode(); err != nil {
			fmt.Println(err)
			return
		}
	}

	fmt.Println("Checking if Node.js and npm are installed...")

	// Check if Node.js and npm are installed
	if !isCommandAvailable("node") || !isCommandAvailable("npm") {
		if err := installNodeAndNpm(); err != nil {
			log.Fatalf("Error installing Node.js or npm: %v", err)
		}
		fmt.Println("Node.js and npm installed successfully.")
	} else {
		fmt.Println("Node.js and npm are already installed.")
	}

	// Path to the JavaScript file
	// jsFilePath := "setdata.js"
	projectDir := "."

	// Node details to pass to JavaScript file (dynamic values)

	fmt.Println("************************************Printing the data**************************")
	fmt.Printf("%+v\n", data.ISPInfo)
	fmt.Println("************************************Printing the data**************************")

	peaqDid := data.Did
	nodename := data.NodeName
	ipaddress := data.IPAddress
	ispinfo := data.ISPInfo
	region := data.Region
	location := "Finance, India (20.2724,85.8338)"

	// Step 1: Check if `@solana/web3.js` is installed
	fmt.Println("Checking if @solana/web3.js is installed...")
	checkPackage := exec.Command("npm", "list", "@solana/web3.js")
	checkPackage.Dir = projectDir
	if err := checkPackage.Run(); err != nil {
		fmt.Println("@solana/web3.js is not installed. Installing now...")
		installPackage := exec.Command("npm", "install", "@solana/web3.js")
		installPackage.Dir = projectDir
		if err := installPackage.Run(); err != nil {
			fmt.Printf("Error installing @solana/web3.js: %s\n", err.Error())
			return
		}
		fmt.Println("Successfully installed @solana/web3.js.")
	} else {
		fmt.Println("@solana/web3.js is already installed.")
	}

	// Step 2: Check if `@project-serum/anchor` is installed
	fmt.Println("Checking if @project-serum/anchor is installed...")
	checkAnchorPackage := exec.Command("npm", "list", "@project-serum/anchor")
	checkAnchorPackage.Dir = projectDir
	if err := checkAnchorPackage.Run(); err != nil {
		fmt.Println("@project-serum/anchor is not installed. Installing now...")
		installAnchorPackage := exec.Command("npm", "install", "@project-serum/anchor")
		installAnchorPackage.Dir = projectDir
		if err := installAnchorPackage.Run(); err != nil {
			fmt.Printf("Error installing @project-serum/anchor: %s\n", err.Error())
			return
		}
		fmt.Println("Successfully installed @project-serum/anchor.")
	} else {
		fmt.Println("@project-serum/anchor is already installed.")
	}

	// Step 2: Run npm install
	fmt.Println("Running npm install to ensure all dependencies are installed...")
	npmInstall := exec.Command("npm", "install")
	npmInstall.Dir = projectDir
	if err := npmInstall.Run(); err != nil {
		fmt.Printf("Error running npm install: %s\n", err.Error())
		return
	}
	fmt.Println("All dependencies are installed.")

	executeJS(peaqDid, nodename, ipaddress, ispinfo, region, location, os.Getenv("SOLANA_PRIVATE_KEY"))
}

func executeJS(peaqDid, nodeName, ipAddress, ispInfo, region, location, privateKey string) {

	fmt.Println("printing the insertion : ", peaqDid, nodeName, ipAddress, ispInfo, region, location, privateKey)

	// Create the command to run setdata.js
	cmd := exec.Command("node", "blockchain/solana/soon/setdata.js")

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("NODE_DID=%s", peaqDid),
		fmt.Sprintf("NODE_NAME=%s", nodeName),
		fmt.Sprintf("IP_ADDRESS=%s", ipAddress),
		fmt.Sprintf("ISP_INFO=%s", ispInfo),
		fmt.Sprintf("REGION=%s", region),
		fmt.Sprintf("LOCATION=%s", location),
		`SOLANA_PRIVATE_KEY=`+privateKey+``,
		"PURE_JS=true",
	)

	// Set output to use the current process's stdout and stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error running setdata.js: %v\n", err)
		os.Exit(1)
	}
}
