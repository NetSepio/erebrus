// package main

// import (
// 	"bytes"
// 	"fmt"
// 	"os"
// 	"os/exec"
// )

// func main() {
// 	// Path to the JavaScript file
// 	jsFilePath := "setdata.js"
// 	projectDir := "."

// 	// Your private key in base58 format (replace with your actual private key)
// 	privateKey := "kRVZ7yFvAZFufaUJheFecbp8gESdEFB6hzwRXzzdJGZdnH6hTvdhhZf65bCfG2fAyb9AxFuw3JTvXDPUPXwA5rG"

// 	// Step 1: Check if `@solana/web3.js` is installed
// 	fmt.Println("Checking if @solana/web3.js is installed...")
// 	checkPackage := exec.Command("npm", "list", "@solana/web3.js")
// 	checkPackage.Dir = projectDir
// 	if err := checkPackage.Run(); err != nil {
// 		fmt.Println("@solana/web3.js is not installed. Installing now...")
// 		installPackage := exec.Command("npm", "install", "@solana/web3.js")
// 		installPackage.Dir = projectDir
// 		if err := installPackage.Run(); err != nil {
// 			fmt.Printf("Error installing @solana/web3.js: %s\n", err.Error())
// 			return
// 		}
// 		fmt.Println("Successfully installed @solana/web3.js.")
// 	} else {
// 		fmt.Println("@solana/web3.js is already installed.")
// 	}

// 	// Step 2: Run npm install
// 	fmt.Println("Running npm install to ensure all dependencies are installed...")
// 	npmInstall := exec.Command("npm", "install")
// 	npmInstall.Dir = projectDir
// 	if err := npmInstall.Run(); err != nil {
// 		fmt.Printf("Error running npm install: %s\n", err.Error())
// 		return
// 	}
// 	fmt.Println("All dependencies are installed.")

// 	// Step 3: Execute the JavaScript file
// 	fmt.Println("Executing JavaScript file:", jsFilePath)
// 	cmd := exec.Command("node", jsFilePath)
// 	cmd.Dir = projectDir

// 	// Set the private key as an environment variable
// 	cmd.Env = append(os.Environ(), fmt.Sprintf("SOLANA_PRIVATE_KEY=%s", privateKey))

// 	// Buffers to capture the output and errors
// 	var stdout, stderr bytes.Buffer
// 	cmd.Stdout = &stdout
// 	cmd.Stderr = &stderr

// 	// Run the command
// 	err := cmd.Run()

// 	// Handle errors and output
// 	if err != nil {
// 		fmt.Printf("Error executing JavaScript: %s\n", err.Error())
// 		fmt.Printf("Stderr: %s\n", stderr.String())
// 		return
// 	}

//		fmt.Println("JavaScript Output:")
//		fmt.Println(stdout.String())
//	}
package soon_solana

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"text/template"
)

// func main() {
// 	SoonNodeCreation()
// }

type NodeDetails struct {
	PrivateKey string `json:"private_key"` // Private key in base58 format
	PeaqDid    string `json:"peaq_did"`    // DID (Decentralized Identifier)
	NodeName   string `json:"nodename"`    // Name of the node
	IPAddress  string `json:"ipaddress"`   // IP address of the node
	ISPInfo    string `json:"ispinfo"`     // Information about the ISP
	Region     string `json:"region"`      // Region of the node
	Location   string `json:"location"`    // Geographical location of the node
}

func SoonNodeCreation(data NodeDetails) {
	// Path to the JavaScript file
	// jsFilePath := "setdata.js"
	jsFilePath := "blockchain/solana/soon/setdata.js"

	//
	projectDir := "blockchain/solana/soon/."

	// // Your private key in base58 format (replace with your actual private key)
	// privateKey := "kRVZ7yFvAZFufaUJheFecbp8gESdEFB6hzwRXzzdJGZdnH6hTvdhhZf65bCfG2fAyb9AxFuw3JTvXDPUPXwA5rG"

	// Set environment variable to pass to JS file
	os.Setenv("SOLANA_PRIVATE_KEY", data.PrivateKey)

	// Node details to pass to JavaScript file (dynamic values)
	peaqDid := data.PeaqDid
	nodename := data.NodeName
	ipaddress := data.IPAddress
	ispinfo := data.IPAddress
	region := data.Region
	location := data.Location

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

	// Step 3: Inject dynamic data into setdata.js
	err := injectDataIntoJS(jsFilePath, peaqDid, nodename, ipaddress, ispinfo, region, location)
	if err != nil {
		fmt.Printf("Error injecting data into JS file: %s\n", err.Error())
		return
	}

	// Step 4: Execute the modified JavaScript file
	fmt.Println("Executing data file:", jsFilePath)
	cmd := exec.Command("node", jsFilePath)
	cmd.Dir = projectDir

	// Buffers to capture the output and errors
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()

	// Handle errors and output
	if err != nil {
		fmt.Printf("Error executing JavaScript: %s\n", err.Error())
		fmt.Printf("Stderr: %s\n", stderr.String())
		return
	}

	fmt.Println("Data Output:")
	fmt.Println(stdout.String())
}

// Function to inject dynamic data into the JS file
func injectDataIntoJS(jsFilePath, peaqDid, nodename, ipaddress, ispinfo, region, location string) error {
	// Read the template JS file
	jsTemplate, err := ioutil.ReadFile(jsFilePath)
	if err != nil {
		return fmt.Errorf("failed to read JS file: %w", err)
	}

	// Define a template to inject the dynamic values
	jsTemplateStr := string(jsTemplate)
	tmpl, err := template.New("js").Parse(jsTemplateStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create a struct to hold dynamic data
	data := struct {
		PeaqDid  string
		NodeName string
		IPAddr   string
		ISPInfo  string
		Region   string
		Location string
	}{
		PeaqDid:  peaqDid,
		NodeName: nodename,
		IPAddr:   ipaddress,
		ISPInfo:  ispinfo,
		Region:   region,
		Location: location,
	}

	// Create a temporary JS file with dynamic data injected
	tempJsFilePath := jsFilePath + ".temp"
	tempFile, err := os.Create(tempJsFilePath)
	if err != nil {
		return fmt.Errorf("failed to create temporary JS file: %w", err)
	}
	defer tempFile.Close()

	// Execute the template with dynamic data and write to the temp file
	err = tmpl.Execute(tempFile, data)
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Replace the original JS file with the modified one
	err = os.Rename(tempJsFilePath, jsFilePath)
	if err != nil {
		return fmt.Errorf("failed to rename temp JS file: %w", err)
	}

	return nil
}
