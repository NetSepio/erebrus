package challengeid

import "errors"

type GetChallengeIdPayload struct {
	Eula        string `json:"eula,omitempty"`
	ChallengeId string `json:"challangeId"`
}

var (
	ErrInvalidChain   = errors.New("unsupported blockchain")
	ErrInvalidAddress = errors.New("invalid address format")
)
