package authenticate

import (
	"fmt"
	"net/http"
	"os"

	"github.com/NetSepio/erebrus/api/v1/authenticate/challengeid"
	"github.com/NetSepio/erebrus/util/pkg/auth"
	"github.com/NetSepio/erebrus/util/pkg/claims"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// ApplyRoutes applies router to gin Router
func ApplyRoutes(r *gin.RouterGroup) {
	g := r.Group("/authenticate")
	{
		g.GET("", challengeid.GetChallengeId)
		g.POST("", authenticate)

	}
}

func authenticate(c *gin.Context) {

	var req AuthenticateRequest
	err := c.BindJSON(&req)
	if err != nil {
		log.WithFields(log.Fields{
			"err": err,
		}).Error("Invalid request payload")

		errResponse := ErrAuthenticate(err.Error())
		c.JSON(http.StatusForbidden, errResponse)
		return
	}
	userAuthEULA := os.Getenv("AUTH_EULA")
	message := userAuthEULA + req.ChallengeId

	var (
		isCorrect     bool
		walletAddress string
	)

	switch req.ChainName {
	case "ethereum", "peaq":
		userAuthEULA := userAuthEULA
		message := userAuthEULA + req.ChallengeId
		walletAddress, isCorrect, err = CheckSignEthereum(req.Signature, req.ChallengeId, message)

		if err == ErrChallengeIdNotFound {

			log.WithFields(log.Fields{"err": err}).Errorf("Challenge Id not found")

			c.JSON(http.StatusNotFound, ErrAuthenticate("Challenge Id not found"))

			return
		}

		if err != nil {
			fmt.Println("error", err)

			log.WithFields(log.Fields{"err": err}).Errorf("failed to CheckSignature, error %v", err.Error())

			c.JSON(http.StatusNotFound, ErrAuthenticate("failed to CheckSignature, error :"+err.Error()))
			return
		}

	case "aptos":
		userAuthEULA := userAuthEULA
		message := fmt.Sprintf("APTOS\nmessage: %v\nnonce: %v", userAuthEULA, req.ChallengeId)
		walletAddress, isCorrect, err = CheckSignAptos(req.Signature, req.ChallengeId, message, req.PubKey)

		if err == ErrChallengeIdNotFound {
			log.WithFields(log.Fields{"err": err}).Errorf("Challenge Id not found")

			c.JSON(http.StatusNotFound, ErrAuthenticate("Challenge Id not found"))

			return
		}

		if err != nil {

			log.WithFields(log.Fields{"err": err}).Errorf("failed to CheckSignature, error %v", err.Error())

			c.JSON(http.StatusNotFound, ErrAuthenticate("failed to CheckSignature, error :"+err.Error()))
			return
		}

	case "sui":
		walletAddress, isCorrect, err = CheckSignSui(req.Signature, req.ChallengeId)

		if err == ErrChallengeIdNotFound {

			log.WithFields(log.Fields{"err": err}).Errorf("Challenge Id not found")

			c.JSON(http.StatusNotFound, ErrAuthenticate("Challenge Id not found"))
			return
		}

		if err != nil {
			log.WithFields(log.Fields{"err": err}).Errorf("failed to CheckSignature, error %v", err.Error())

			c.JSON(http.StatusNotFound, ErrAuthenticate("failed to CheckSignature, error : "+err.Error()))
			return
		}

	case "solana":
		walletAddress, isCorrect, err = CheckSignSolana(req.Signature, req.ChallengeId, message, req.PubKey)

		if err == ErrChallengeIdNotFound {
			log.WithFields(log.Fields{"err": err}).Errorf("Challenge Id not found")
			c.JSON(http.StatusNotFound, ErrAuthenticate("Challenge Id not found"))
			return
		}

		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Errorf("failed to CheckSignature, error : %v", err.Error())
			errResponse := ErrAuthenticate("failed to CheckSignature, error :" + err.Error())
			c.JSON(http.StatusInternalServerError, errResponse)
			return

		}

	default:
		info := "chain name must be between solana, peaq, aptos, sui, eclipse, ethereum"
		log.WithFields(log.Fields{
			"err": err,
		}).Errorf("Invalid chain name, INFO : %s\n", info)
		errResponse := ErrAuthenticate("failed to CheckSignature, error :" + "Invalid chain name, INFO : " + info)
		c.JSON(http.StatusInternalServerError, errResponse)
		return
	}
	if isCorrect {
		customClaims := claims.New(walletAddress)
		pasetoToken, err := auth.GenerateTokenPaseto(customClaims)
		if err != nil {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("failed to generate token")
			errResponse := ErrAuthenticate(err.Error())
			c.JSON(http.StatusInternalServerError, errResponse)
			return
		}
		delete(challengeid.Data, req.ChallengeId)
		payload := AuthenticatePayload{
			Status:  200,
			Success: true,
			Message: "Successfully Authenticated",
			Token:   pasetoToken,
		}
		c.JSON(http.StatusAccepted, payload)
	} else {
		errResponse := ErrAuthenticate("Forbidden")
		c.JSON(http.StatusForbidden, errResponse)
		return
	}
}

func ErrAuthenticate(errvalue string) AuthenticatePayload {
	var payload AuthenticatePayload
	payload.Success = false
	payload.Status = 401
	payload.Message = errvalue
	return payload
}
