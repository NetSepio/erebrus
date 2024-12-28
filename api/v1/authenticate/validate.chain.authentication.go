package authenticate

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/NetSepio/erebrus/api/v1/authenticate/challengeid"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/minio/blake2b-simd"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/nacl/sign"
	"golang.org/x/crypto/sha3"
)

var ErrChallengeIdNotFound = errors.New("challenge id not found")

func CheckSignAptos(signature string, challangeId string, message string, pubKey string) (string, bool, error) {
	signatureInBytes, err := hexutil.Decode(signature)
	if err != nil {
		return "", false, err
	}

	sha3_i := sha3.New256()
	signatureInBytes = append(signatureInBytes, []byte(message)...)
	pubBytes, err := hexutil.Decode(pubKey)
	if err != nil {
		return "", false, err
	}
	sha3_i.Write(pubBytes)
	sha3_i.Write([]byte{0})
	hash := sha3_i.Sum(nil)
	addr := hexutil.Encode(hash)

	dbData, exists := challengeid.Data[challangeId]
	if !exists {
		return "", false, ErrChallengeIdNotFound
	}

	if !strings.EqualFold(addr, dbData.WalletAddress) {
		return "", false, err
	}

	msgGot, matches := sign.Open(nil, signatureInBytes, (*[32]byte)(pubBytes))
	if !matches || string(msgGot) != message {
		return "", false, err
	}
	return dbData.WalletAddress, true, nil

}

func CheckSignEthereum(signature string, flowId string, message string) (string, bool, error) {

	newMsg := fmt.Sprintf("\x19Ethereum Signed Message:\n%v%v", len(message), message)

	// fmt.Println("newMsg : ", newMsg)

	newMsgHash := crypto.Keccak256Hash([]byte(newMsg))
	signatureInBytes, err := hexutil.Decode(signature)
	if err != nil {
		return "", false, err
	}
	// check if the signature is in the [R || S || V] format
	if len(signatureInBytes) != 65 {
		return "", false, errors.New("invalid signature length")
	}
	if signatureInBytes[64] == 27 || signatureInBytes[64] == 28 {
		signatureInBytes[64] -= 27
	}
	pubKey, err := crypto.SigToPub(newMsgHash.Bytes(), signatureInBytes)

	if err != nil {
		return "", false, err
	}

	//Get address from public key
	walletAddress := crypto.PubkeyToAddress(*pubKey)

	flowIdData := challengeid.Data[flowId]
	if (challengeid.MemoryDB{}) == flowIdData {
		return "", false, ErrChallengeIdNotFound
	}
	if strings.EqualFold(flowIdData.WalletAddress, walletAddress.String()) {
		return flowIdData.WalletAddress, true, nil
	} else {
		return "", false, errors.New("mismatch wallet_address")
	}
}

func CheckSignSui(signature string, challangeId string) (string, bool, error) {
	// Decode signature
	signatureBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return "", false, err
	}

	// Assuming ED25519 signature format
	size := 32

	publicKey := signatureBytes[len(signatureBytes)-size:]
	pubKey := &ecdsa.PublicKey{
		Curve: elliptic.P256(),                       // Curve is not used in serialization
		X:     new(big.Int).SetBytes(publicKey[:]),   // Set X coordinate
		Y:     new(big.Int).SetBytes(publicKey[32:]), // Set Y coordinate
	}
	if pubKey.X == nil || pubKey.Y == nil {
		return "", false, err
	}
	// Serialize the public key into bytes
	pubKeyBytes := pubKey.X.Bytes()

	// Pad X coordinate bytes to ensure they are the same length as the curve's bit size
	paddingLen := (pubKey.Curve.Params().BitSize + 7) / 8
	pubKeyBytes = append(make([]byte, paddingLen-len(pubKeyBytes)), pubKeyBytes...)

	// Concatenate the signature scheme flag (0x00 for Ed25519) with the serialized public key bytes
	concatenatedBytes := append([]byte{0x00}, pubKeyBytes...)

	// Compute the BLAKE2b hash
	hash := blake2b.Sum256(concatenatedBytes)

	// The resulting hash is the Sui address
	suiAddress := "0x" + hex.EncodeToString(hash[:])

	dbData, exists := challengeid.Data[challangeId]
	if !exists {
		return "", false, ErrChallengeIdNotFound
	}

	if !strings.EqualFold(suiAddress, dbData.WalletAddress) {
		return "", false, err
	}

	return dbData.WalletAddress, true, nil
}

func CheckSignSolana(signature string, challangeId string, message string, pubKey string) (string, bool, error) {

	bytes, err := base58.Decode(pubKey)
	if err != nil {
		return "", false, err
	}
	messageAsBytes := []byte(message)

	signedMessageAsBytes, err := hex.DecodeString(signature)

	if err != nil {

		return "", false, err
	}

	dbData, exists := challengeid.Data[challangeId]
	if !exists {
		return "", false, ErrChallengeIdNotFound
	}

	ed25519.Verify(bytes, messageAsBytes, signedMessageAsBytes)

	return dbData.WalletAddress, true, nil

}
