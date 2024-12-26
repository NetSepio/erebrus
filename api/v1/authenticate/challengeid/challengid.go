package challengeid

import (
	"net/http"
	"os"
	"time"

	"github.com/NetSepio/erebrus/core"
	"github.com/NetSepio/erebrus/util"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

//	type User struct {
//		Name          string   `json:"name,omitempty"`
//		WalletAddress string   `gorm:"primary_key" json:"walletAddress"`
//		FlowIds       []FlowId `gorm:"foreignkey:WalletAddress" json:"-"`
//	}
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

	// TODO: verify wallet address depending on chainName: ethereum, solana, peaq, aptos, sui, eclipse...
	_, err := hexutil.Decode(walletAddress)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Wallet address (walletAddress) is not valid")

		response := core.MakeErrorResponse(400, err.Error(), nil, nil, nil)
		c.JSON(http.StatusBadRequest, response)
		return
	}
	if !util.RegexpWalletEth.MatchString(walletAddress) {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Wallet address (walletAddress) is not valid")
		response := core.MakeErrorResponse(400, err.Error(), nil, nil, nil)
		c.JSON(http.StatusBadRequest, response)
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
	Data = map[string]MemoryDB{
		challengeId: dbdata,
	}
	return challengeId, nil
}
