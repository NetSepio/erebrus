package core

import (
	// "crypto/rand"
	"errors"
	// "fmt"
	// "math/big"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/NetSepio/erebrus/model"
	"github.com/NetSepio/erebrus/storage"
	"github.com/NetSepio/erebrus/template"
	"github.com/NetSepio/erebrus/util"
	"github.com/NetSepio/erebrus/util/pkg/stats"
	uuid "github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// RegisterClient client with all necessary data
func RegisterClient(client *model.Client) (*model.Client, error) {
	// check if client is valid
	errs := client.IsValid()
	if len(errs) != 0 {
		for _, err := range errs {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("client validation error")
		}
		return nil, errors.New("failed to validate client")
	}

	u, err := uuid.NewRandom()
	client.UUID = u.String()

	presharedKey, err := wgtypes.GenerateKey()
	if err != nil {
		return nil, err
	}
	client.PresharedKey = presharedKey.String()

	reserverIps, err := GetAllReservedIps()
	if err != nil {
		return nil, err
	}

	ips := make([]string, 0)
	for _, network := range client.Address {
		ip, err := util.GetAvailableIP(network, reserverIps)
		if err != nil {
			return nil, err
		}
		if util.IsIPv6(ip) {
			ip = ip + "/128"
		} else {
			ip = ip + "/32"
		}
		ips = append(ips, ip)
	}
	client.Address = ips
	client.CreatedAt = timestamppb.Now().AsTime().UnixMilli()

	client.UpdatedAt = client.CreatedAt

	err = storage.Serialize(client.UUID, client)
	if err != nil {
		return nil, err
	}

	v, err := storage.Deserialize(client.UUID)
	if err != nil {
		return nil, err
	}
	client = v.(*model.Client)

	// data modified, dump new config
	return client, UpdateServerConfigWg()
}

// ReadClient client by id
func ReadClient(id string) (*model.Client, error) {
	v, err := storage.Deserialize(id)
	if err != nil {
		return nil, err
	}
	client := v.(*model.Client)
	pkey := client.PublicKey
	clientStats, err := stats.GetWireGuardStatsForPeer(pkey)
	if err == nil {
		client.ReceiveBytes = clientStats.ReceivedBytes
		client.TransmitBytes = clientStats.TransmittedBytes
	}

	return client, nil
}

// UpdateClient preserve keys
func UpdateClient(UUID string, client *model.Client) (*model.Client, error) {
	v, err := storage.Deserialize(UUID)
	if err != nil {
		return nil, err
	}
	current := v.(*model.Client)

	if current.UUID != client.UUID {
		return nil, errors.New("records UUID mismatch")
	}

	// check if client is valid
	errs := client.IsValid()
	if len(errs) != 0 {
		for _, err := range errs {
			log.WithFields(log.Fields{
				"err": err,
			}).Error("client validation error")
		}
		return nil, errors.New("failed to validate client")
	}

	// Keep Keys
	client.PublicKey = current.PublicKey
	client.PresharedKey = current.PresharedKey
	client.UpdatedAt = timestamppb.Now().AsTime().UnixMilli()

	err = storage.Serialize(client.UUID, client)
	if err != nil {
		return nil, err
	}

	v, err = storage.Deserialize(UUID)
	if err != nil {
		return nil, err
	}
	client = v.(*model.Client)

	// data modified, dump new config
	return client, UpdateServerConfigWg()
}

// DeleteClient from disk
func DeleteClient(id string) error {
	path := filepath.Join(os.Getenv("WG_CLIENTS_DIR"), id)
	err := os.Remove(path)
	if err != nil {
		return err
	}

	// data modified, dump new config
	return UpdateServerConfigWg()
}

// ReadClients all clients
func ReadClients() ([]*model.Client, error) {
	clients := make([]*model.Client, 0)

	files, err := os.ReadDir(filepath.Join(os.Getenv("WG_CLIENTS_DIR")))
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		// clients file name is an uuid
		_, err := uuid.Parse(f.Name())
		if err == nil {
			c, err := storage.Deserialize(f.Name())
			if err != nil {
				log.WithFields(log.Fields{
					"err":  err,
					"path": f.Name(),
				}).Error("failed to deserialize client")
			} else {
				cl := c.(*model.Client)
				pkey := cl.PublicKey
				clientStats, err := stats.GetWireGuardStatsForPeer(pkey)
				if err == nil {
					cl.ReceiveBytes = clientStats.ReceivedBytes
					cl.TransmitBytes = clientStats.TransmittedBytes
				}

				clients = append(clients, cl)
			}
		}
	}

	sort.Slice(clients, func(i, j int) bool {
		return clients[i].CreatedAt < (clients[j].CreatedAt)
	})

	return clients, nil
}

func ReadClientConfig(id string) ([]byte, error) {
	client, err := ReadClient(id)
	if err != nil {
		return nil, err
	}

	server, err := ReadServer()
	if err != nil {
		return nil, err
	}

	configDataWg, err := template.DumpClientWg(client, server)
	if err != nil {
		return nil, err
	}

	return configDataWg, nil
}

// LENGTH 16
// func GeneratePeaqDID(length int) (string, string, error) {
// 	if length <= 0 {
// 		length = 55
// 	}

// 	const validChars = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
// 	result := make([]byte, length)

// 	for i := 0; i < length; i++ {
// 		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(validChars))))
// 		if err != nil {
// 			return "", "", fmt.Errorf("failed to generate random number: %v", err)
// 		}
// 		result[i] = validChars[randomIndex.Int64()]
// 		fmt.Println("result : ", result)
// 	}

// 	return fmt.Sprintf("did:peaq:%s", string(result)), string(result), nil
// }

func IsValidPeaqDID(did string) bool {
	// Check if the DID starts with "did:peaq:"
	if !strings.HasPrefix(did, "did:peaq:") {
		return false
	}

	// Extract the id-string part
	idString := strings.TrimPrefix(did, "did:peaq:")

	// Check if the id-string is not empty
	if len(idString) == 0 {
		return false
	}

	// Define the allowed characters for idchar
	idcharRegex := regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]+$`)

	// Check if the id-string contains only valid idchar characters
	return idcharRegex.MatchString(idString)
}
