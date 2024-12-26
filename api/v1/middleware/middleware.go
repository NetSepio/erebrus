package middleware

import (
	"os"

	log "github.com/sirupsen/logrus"
)

func CheckGatewayAccess(decryptedWalletAddress any) bool {
	AllowedWalletAddress := os.Getenv("GATEWAY_WALLET")
	if AllowedWalletAddress == "*" {
		return true
	}
	if decryptedWalletAddress != AllowedWalletAddress {
		log.WithFields(log.Fields{
			"err": "Updates Not Allowed for the Given Wallet Address",
		}).Error("Updates Not Allowed for the Given Wallet Address")
		return false
	}
	return true

}
