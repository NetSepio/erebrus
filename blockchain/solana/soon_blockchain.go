package soon_blockchain

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"

	soon_solana "github.com/NetSepio/erebrus/blockchain/solana/soon"
	"github.com/NetSepio/erebrus/core"
	"github.com/NetSepio/erebrus/util/pkg/node"
	"github.com/joho/godotenv"
)

func SoonBlockchain(name string) {
	if os.Getenv("CHAIN_NAME") == "SOON" {

		if len(os.Getenv("SOLANA_PRIVATE_KEY")) == 0 {
			// Load the .env file
			err := godotenv.Load()
			if err != nil {
				log.Fatal("Error loading .env file")
				return
			} else {
				if len(os.Getenv("SOLANA_PRIVATE_KEY")) == 0 {
					log.Fatal("Error: SOLANA_PRIVATE_KEY is not set in .env file")
					return
				}
			}
		}

		peaqDid, _, _ := GenerateSoonDID(23)

		IpGeoAddress := node.IpGeoAddress{IpInfoIP: core.GlobalIPInfo.IP,
			IpInfoCity:     core.GlobalIPInfo.City,
			IpInfoCountry:  core.GlobalIPInfo.Country,
			IpInfoLocation: core.GlobalIPInfo.Location,
			IpInfoOrg:      core.GlobalIPInfo.Org,
			IpInfoPostal:   core.GlobalIPInfo.Postal,
			IpInfoTimezone: core.GlobalIPInfo.Timezone}
		fmt.Println("Ip Geo : ")
		fmt.Printf("%+v\n", IpGeoAddress)
		fmt.Println("name : ", name)

		soon_solana.SoonNodeBlockchainCall(soon_solana.NodeDetails{
			PrivateKey: os.Getenv("SOON_PRIVATE_KEY"),
			Did:        peaqDid,
			NodeName:   name,
			IPAddress:  node.ToJSON(node.GetOSInfo()),
			ISPInfo:    node.ToJSON(node.GetIPInfo().IPv4Addresses),
			Region:     core.GlobalIPInfo.Country,
			Location:   node.ToJSON(IpGeoAddress),
		})

	}

}

func GenerateSoonDID(length int) (string, string, error) {
	if length <= 0 {
		length = 55
	}

	const validChars = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	result := make([]byte, length)

	for i := 0; i < length; i++ {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(validChars))))
		if err != nil {
			return "", "", fmt.Errorf("failed to generate random number: %v", err)
		}
		result[i] = validChars[randomIndex.Int64()]
		fmt.Println("result : ", result)
	}

	return fmt.Sprintf("did:soon:%s", string(result)), string(result), nil
}
