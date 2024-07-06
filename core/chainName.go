package core

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

var ChainName string

// Function to load the chain name from the environment and save it to the global variable
func LoadChainName() {
	// Load environment variables from the .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// Get the CHAIN_NAME variable from the environment
	ChainName = os.Getenv("CHAIN_NAME")
	if ChainName == "" {
		log.Fatalf("CHAIN_NAME environment variable is not set")
	}
	fmt.Printf("Chain Name: %s\n", ChainName)
}
