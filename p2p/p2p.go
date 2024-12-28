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
	"github.com/multiformats/go-multiaddr"
	"github.com/sirupsen/logrus"
)

// DiscoveryInterval is how often we search for other peers via the DHT.
const DiscoveryInterval = time.Second * 10

// DiscoveryServiceTag is used in our DHT advertisements to discover
// other peers.
const DiscoveryServiceTag = "erebrus"

var StartTimeStamp int64

func Init() {

	var name string

	if os.Getenv("NODE_NAME") != "" {
		name = os.Getenv("NODE_NAME")
	} else {
		name = namesgenerator.GetRandomName(0) 

	}
	StartTimeStamp = time.Now().Unix()
	ctx := context.Background()

	// create a new libp2p Host
	ha, err := makeBasicHost()
	if err != nil {
		log.Fatal(err)
	}

	fullAddr := getHostAddress(ha)
	log.Printf("I am %s\n", fullAddr)

	remoteAddr := "/ip4/" + os.Getenv("HOST_IP") + "/tcp/" + os.Getenv("LIBP2P_PORT") + "/p2p/" + ha.ID().String()
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

	//Topic 1
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
	//Subscribe to the topic.
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

	//Topic 2
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
	//Subscribe to the topic.
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
