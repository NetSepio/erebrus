package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/NetSepio/erebrus/core"
	"github.com/NetSepio/erebrus/util/pkg/node"
	"github.com/docker/docker/pkg/namesgenerator"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
)

// DiscoveryInterval is how often we search for other peers via the DHT.
const DiscoveryInterval = time.Second * 10

// DiscoveryServiceTag is used in our DHT advertisements to discover
// other peers.
const DiscoveryServiceTag = "erebrus"

var (
	StartTimeStamp int64
	quicManager    *QUICManager
)

func Init() {
	var name string

	if os.Getenv("NODE_NAME") != "" {
		name = os.Getenv("NODE_NAME")
	} else {
		name = namesgenerator.GetRandomName(0)
	}
	StartTimeStamp = time.Now().Unix()
	ctx := context.Background()

	// create a new libp2p Host with enhanced QUIC support
	ha, err := makeBasicHost()
	if err != nil {
		log.Fatal(err)
	}

	// Initialize QUIC manager for enhanced connection management
	quicManager = NewQUICManager(ha)

	// Setup connection event listeners for QUIC optimization
	setupConnectionEventListeners(ha)

	fullAddr := getHostAddress(ha)
	log.Printf("I am %s\n", fullAddr)

	// Use QUIC/UDP instead of TCP for improved performance
	quicPort := os.Getenv("LIBP2P_PORT")
	if quicPort == "" {
		quicPort = "9002" // Default QUIC port
	}
	remoteAddr := "/ip4/" + os.Getenv("HOST_IP") + "/udp/" + quicPort + "/quic/p2p/" + ha.ID().String()
	// Create a new PubSub service using the GossipSub router.
	ps, err := pubsub.NewGossipSub(ctx, ha)
	if err != nil {
		panic(err)
	}

	// Setup DHT with empty discovery peers so this will be a discovery peer for other
	// peers. This peer should run with a public ip address, otherwise change "nil" to
	// a list of peers to bootstrap with.
	bootstrapPeer, err := multiaddr.NewMultiaddr(os.Getenv("GATEWAY_PEERID"))
	if err != nil {
		panic(err)
	}
	dht, err := NewDHT(ctx, ha, []multiaddr.Multiaddr{bootstrapPeer})
	if err != nil {
		panic(err)
	}

	// Setup global peer discovery over DiscoveryServiceTag.
	go Discover(ctx, ha, dht, DiscoveryServiceTag)

	// Topic 1
	topicString := "status" // Change "UniversalPeer" to whatever you want!
	topic, err := ps.Join(DiscoveryServiceTag + "/" + topicString)
	if err != nil {
		panic(err)
	}
	go func() {
		time.Sleep(5 * time.Second)
		fmt.Println("sending status")
		node_data := node.CreateNodeStatus(remoteAddr, ha.ID().String(), StartTimeStamp, name)
		msgBytes, err := json.Marshal(node_data)
		log.Println("node data", node_data)
		if err != nil {
			panic(err)
		}
		if err := topic.Publish(ctx, msgBytes); err != nil {
			panic(err)
		}
	}()
	// Subscribe to the topic.
	sub, err := topic.Subscribe()
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			// Block until we recieve a new message.
			msg, err := sub.Next(ctx)
			if err != nil {
				panic(err)
			}
			if msg.ReceivedFrom == ha.ID() {
				continue
			}
			fmt.Printf("[%s] %s", msg.ReceivedFrom, string(msg.Data))
			fmt.Println()
		}
	}()

	// Topic 2
	ClientTopicString := "client" // Change "UniversalPeer" to whatever you want!
	ClientTopic, err := ps.Join(DiscoveryServiceTag + "/" + ClientTopicString)
	if err != nil {
		panic(err)
	}
	go func() {
		time.Sleep(5 * time.Second)
		fmt.Println("sending clients")
		clients, err := core.ReadClients()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"err": err,
			}).Error("failed to list clients")
			return
		}

		msgBytes, err := json.Marshal(clients)
		if err != nil {
			panic(err)
		}
		if err := topic.Publish(ctx, msgBytes); err != nil {
			panic(err)
		}
	}()
	// Subscribe to the topic.
	ClientSub, err := ClientTopic.Subscribe()
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			// Block until we recieve a new message.
			msg, err := ClientSub.Next(ctx)
			if err != nil {
				panic(err)
			}
			if msg.ReceivedFrom == ha.ID() {
				continue
			}
			fmt.Printf("[%s] %s", msg.ReceivedFrom, string(msg.Data))
			fmt.Println()
		}
	}()

	// Log QUIC-specific statistics periodically
	go logQUICStatistics()
}

type status struct {
	Status string
}

func sendStatusMsg(msg string, topic *pubsub.Topic, ctx context.Context) {
	m := status{
		Status: msg,
	}
	msgBytes, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	if err := topic.Publish(ctx, msgBytes); err != nil {
		panic(err)
	}
}

// setupConnectionEventListeners sets up event listeners for enhanced QUIC management
func setupConnectionEventListeners(host host.Host) {
	host.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			logrus.WithFields(logrus.Fields{
				"peerID":    conn.RemotePeer(),
				"transport": getConnectionTransport(conn),
				"multiaddr": conn.RemoteMultiaddr(),
			}).Info("New connection established")

			// Track QUIC connections specifically
			if isQUICConnection(conn) {
				logrus.WithField("peerID", conn.RemotePeer()).Debug("QUIC connection established")
			}
		},
		DisconnectedF: func(n network.Network, conn network.Conn) {
			logrus.WithFields(logrus.Fields{
				"peerID":    conn.RemotePeer(),
				"transport": getConnectionTransport(conn),
			}).Debug("Connection closed")
		},
	})
}

// isQUICConnection checks if a connection is using QUIC transport
func isQUICConnection(conn network.Conn) bool {
	return isQUICAddress(conn.RemoteMultiaddr())
}

// logQUICStatistics logs QUIC performance statistics periodically
func logQUICStatistics() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		if quicManager != nil {
			metrics := quicManager.GetMetrics()
			logrus.WithFields(logrus.Fields{
				"totalConnections":      metrics.TotalConnections,
				"activeConnections":     metrics.ActiveConnections,
				"failedConnections":     metrics.FailedConnections,
				"totalStreams":          metrics.TotalStreams,
				"bytesTransferred":      metrics.BytesTransferred,
				"avgConnectionDuration": metrics.ConnectionDuration,
			}).Info("QUIC performance statistics")
		}
	}
}

// GetQUICManagerInstance returns the global QUIC manager instance
func GetQUICManagerInstance() *QUICManager {
	return quicManager
}
