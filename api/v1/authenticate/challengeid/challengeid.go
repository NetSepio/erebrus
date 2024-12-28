package challengeid

import (
	"encoding/hex"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/NetSepio/erebrus/core"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type FlowId struct {
	WalletAddress string
	FlowId        string `gorm:"primary_key"`
}
type MemoryDB struct {
	WalletAddress string
	ChainName     string
	Timestamp     time.Time
}

var Data map[string]MemoryDB

// Get walletAddress, chain and return eula, challengeId
func GetChallengeId(c *gin.Context) {
	walletAddress := c.Query("walletAddress")
	chainName := c.Query("chainName")

	if walletAddress == "" {
		log.WithFields(log.Fields{
			"err": "empty Wallet Address",
		}).Error("failed to create client")

		response := core.MakeErrorResponse(403, "Empty Wallet Address", nil, nil, nil)
		c.JSON(http.StatusForbidden, response)
		return
	}

	if chainName == "" {
		log.WithFields(log.Fields{
			"err": "empty Chain name",
		}).Error("failed to create client")

		response := core.MakeErrorResponse(403, "Empty Wallet Address", nil, nil, nil)
		c.JSON(http.StatusForbidden, response)
		return
	}

	if err := ValidateAddress(chainName, walletAddress); err != nil {

		info := "chain name = " + chainName + "; please pass chain name between solana, peaq, aptos, sui, eclipse, ethereum"

		switch err {
		case ErrInvalidChain:
			log.WithFields(log.Fields{"err": ErrInvalidChain}).Error("failed to create client")
			response := core.MakeErrorResponse(http.StatusNotAcceptable, ErrInvalidChain.Error()+info, nil, nil, nil)
			c.JSON(http.StatusNotAcceptable, response)
			return
		case ErrInvalidAddress:
			log.WithFields(log.Fields{"err": ErrInvalidAddress}).Error("failed to create client")
			response := core.MakeErrorResponse(http.StatusNotAcceptable, ErrInvalidAddress.Error(), nil, nil, nil)
			c.JSON(http.StatusNotAcceptable, response)
			return
		}
		return
	}

	challengeId, err := GenerateChallengeId(walletAddress, chainName)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("failed to create FlowId")
		response := core.MakeErrorResponse(500, err.Error(), nil, nil, nil)
		c.JSON(http.StatusInternalServerError, response)
		return
	}
	userAuthEULA := os.Getenv("AUTH_EULA")
	payload := GetChallengeIdPayload{
		ChallengeId: challengeId,
		Eula:        userAuthEULA,
	}
	c.JSON(200, payload)
}

func GenerateChallengeId(walletAddress string, chainName string) (string, error) {
	challengeId := uuid.NewString()
	var dbdata MemoryDB
	dbdata.WalletAddress = walletAddress
	dbdata.Timestamp = time.Now()
	dbdata.ChainName = chainName
	Data = map[string]MemoryDB{
		challengeId: dbdata,
	}
	return challengeId, nil
}

// ValidateAddress validates a wallet address for the specified blockchain
func ValidateAddress(chain, address string) error {
	// Convert chain name to lowercase for case-insensitive comparison

	switch chain {
	case "ethereum":
		if !ValidateAddressEtherium(address) {
			return ErrInvalidAddress
		}
	case "solana", "eclipse":
		if !ValidateSolanaAddress(address) {
			return ErrInvalidAddress
		}

	case "peaq":
		if !ValidatePeaqAddress(address) {
			return ErrInvalidAddress
		}

	case "aptos":
		if !ValidateAptosAddress(address) {
			return ErrInvalidAddress
		}

	case "sui":
		if !ValidateSuiAddress(address) {
			return ErrInvalidAddress
		}

	default:
		return ErrInvalidChain
	}

	return nil
}

// ValidateSolanaAddress checks if the given string is a valid Solana wallet address
func ValidateSolanaAddress(address string) bool {
	if len(address) < 32 || len(address) > 44 {
		return false
	}

	// Solana addresses only contain base58 characters
	matched, _ := regexp.MatchString("^[1-9A-HJ-NP-Za-km-z]+$", address)
	return matched
}

// ValidatePeaqAddress checks if the given string is a valid Peaq wallet address
func ValidatePeaqAddress(address string) bool {
	if len(address) != 48 || !strings.HasPrefix(address, "5") {
		return false
	}

	// Peaq addresses only contain base58 characters
	matched, _ := regexp.MatchString("^[1-9A-HJ-NP-Za-km-z]+$", address)
	return matched
}

// ValidateAptosAddress checks if the given string is a valid Aptos wallet address
func ValidateAptosAddress(address string) bool {
	if len(address) != 66 || !strings.HasPrefix(address, "0x") {
		return false
	}

	// Remove "0x" prefix and check if remaining string is valid hex
	address = strings.TrimPrefix(address, "0x")
	_, err := hex.DecodeString(address)
	return err == nil
}

// ValidateSuiAddress checks if the given string is a valid Sui wallet address
func ValidateSuiAddress(address string) bool {
	if len(address) != 42 || !strings.HasPrefix(address, "0x") {
		return false
	}

	// Remove "0x" prefix and check if remaining string is valid hex
	address = strings.TrimPrefix(address, "0x")
	_, err := hex.DecodeString(address)
	return err == nil
}

func ValidateAddressEtherium(address string) bool {
	if len(address) != 42 || !strings.HasPrefix(address, "0x") {
		return false
	}
	_, isValid := big.NewInt(0).SetString(address[2:], 16)
	return isValid
}
