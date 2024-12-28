package authenticate

import (
	"fmt"
	"net/http"
	"os"

	"github.com/NetSepio/erebrus/api/v1/authenticate/challengeid"
	"github.com/NetSepio/erebrus/util/pkg/auth"
	"github.com/NetSepio/erebrus/util/pkg/claims"
	"github.com/NetSepio/gateway/util/pkg/logwrapper"
	"github.com/TheLazarusNetwork/go-helpers/httpo"

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
	// walletAddress, isCorrect, err := cryptosign.CheckSign(req.Signature, req.ChallengeId, message)

	// if err == cryptosign.ErrChallangeIdNotFound {
	// 	log.WithFields(log.Fields{
	// 		"err": err,
	// 	}).Error("FlowId Not Found")
	// 	errResponse := ErrAuthenticate(err.Error())
	// 	c.JSON(http.StatusNotFound, errResponse)
	// 	return
	// }

	// if err != nil {
	// 	log.WithFields(log.Fields{
	// 		"err": err,
	// 	}).Error("failed to CheckSignature")
	// 	errResponse := ErrAuthenticate(err.Error())
	// 	c.JSON(http.StatusInternalServerError, errResponse)
	// 	return
	// }

	var (
		isCorrect bool
		// userId     string
		walletAddr string
	)

	switch req.ChainName {
	case "EVM", "PEAQ":
		userAuthEULA := userAuthEULA
		message := userAuthEULA + req.ChallengeId
		walletAddr, isCorrect, err = CheckSignEth(req.Signature, req.ChallengeId, message)

		if err == ErrChallangeIdNotFound {
			httpo.NewErrorResponse(http.StatusNotFound, "Challange Id not found")
			return
		}

		if err != nil {
			logwrapper.Errorf("failed to CheckSignature, error %v", err.Error())
			httpo.NewErrorResponse(http.StatusInternalServerError, "Unexpected error occurred").SendD(c)
			return
		}

	case "APTOS":
		userAuthEULA := userAuthEULA
		message := fmt.Sprintf("APTOS\nmessage: %v\nnonce: %v", userAuthEULA, req.ChallengeId)
		walletAddr, isCorrect, err = CheckSign(req.Signature, req.ChallengeId, message, req.PubKey)

		if err == ErrChallangeIdNotFound {
			httpo.NewErrorResponse(http.StatusNotFound, "Challange Id not found")
			return
		}

		if err != nil {
			logwrapper.Errorf("failed to CheckSignature, error %v", err.Error())
			httpo.NewErrorResponse(http.StatusInternalServerError, "Unexpected error occurred").SendD(c)
			return
		}

	case "SUI":
		walletAddr, isCorrect, err = CheckSignSui(req.Signature, req.ChallengeId)

		if err == ErrChallangeIdNotFound {
			httpo.NewErrorResponse(http.StatusNotFound, "Challange Id not found")
			return
		}

		if err != nil {
			logwrapper.Errorf("failed to CheckSignature, error %v", err.Error())
			httpo.NewErrorResponse(http.StatusInternalServerError, "Unexpected error occurred").SendD(c)
			return
		}

	case "SOLANA":
		walletAddr, isCorrect, err = CheckSignSol(req.Signature, req.ChallengeId, message, req.PubKey)

		if err == ErrChallangeIdNotFound {
			httpo.NewErrorResponse(http.StatusNotFound, "Challange Id not found")
			return
		}

		if err != nil {
			logwrapper.Errorf("failed to CheckSignature, error %v", err.Error())
			httpo.NewErrorResponse(http.StatusInternalServerError, "Unexpected error occurred").SendD(c)
			return
		}
	}
	if isCorrect {
		customClaims := claims.New(walletAddr)
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
