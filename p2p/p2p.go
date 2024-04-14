package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/NetSepio/erebrus/util/pkg/node"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
)

// DiscoveryInterval is how often we search for other peers via the DHT.
const DiscoveryInterval = time.Second * 10

// DiscoveryServiceTag is used in our DHT advertisements to discover
// other peers.
const DiscoveryServiceTag = "erebrus"

var StartTimeStamp int64

func Init() {
	StartTimeStamp = time.Now().Unix()
	ctx := context.Background()

	// create a new libp2p Host
	ha, err := makeBasicHost()
	if err != nil {
		log.Fatal(err)
	}

	fullAddr := getHostAddress(ha)
	log.Printf("I am %s\n", fullAddr)

	remoteAddr := "/ip4/" + os.Getenv("HOST_IP") + "/tcp/9002/p2p/" + ha.ID().String()
	// Create a new PubSub service using the GossipSub router.
	ps, err := pubsub.NewGossipSub(ctx, ha)
	if err != nil {
		panic(err)
	}

	// Setup DHT with empty discovery peers so this will be a discovery peer for other
	// peers. This peer should run with a public ip address, otherwise change "nil" to
	// a list of peers to bootstrap with.
	bootstrapPeer, err := multiaddr.NewMultiaddr(os.Getenv("MASTERNODE_PEERID"))
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
		fmt.Println("sending")
		node_data := node.CreateNodeStatus(remoteAddr, ha.ID().String(), StartTimeStamp)
		msgBytes, err := json.Marshal(node_data)
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

	// //Topic 2
	// topicString2 := "client" // Change "UniversalPeer" to whatever you want!
	// topic2, err := ps.Join(DiscoveryServiceTag + "/" + topicString2)
	// if err != nil {
	// 	panic(err)
	// }
	// // if err := topic2.Publish(ctx, []byte("sending data over client topic")); err != nil {
	// // 	panic(err)
	// // }
	// sub2, err := topic2.Subscribe()
	// if err != nil {
	// 	panic(err)
	// }

	// go func() {
	// 	for {
	// 		// Block until we recieve a new message.
	// 		msg, err := sub2.Next(ctx)

	// 		if err != nil {
	// 			panic(err)
	// 		}
	// 		// if msg.ReceivedFrom == ha.ID() {
	// 		// 	continue
	// 		// }
	// 		fmt.Printf("[%s] %s", msg.ReceivedFrom, string(msg.Data))
	// 		fmt.Println()
	// 	}
	// }()

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