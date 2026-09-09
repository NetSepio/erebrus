package gatewayclient

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHelloAckPublishesHeartbeatUpdate(t *testing.T) {
	client := New("https://gateway.example", "node", "token", nil, nil, nil)
	data, err := json.Marshal(Envelope{
		Type: TypeHelloAck,
		Data: json.RawMessage(`{"heartbeat_interval_sec":12}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.handleFrame(context.Background(), (*websocket.Conn)(nil), data); err != nil {
		t.Fatal(err)
	}
	select {
	case interval := <-client.heartbeatUpdates:
		if interval != 12*time.Second {
			t.Fatalf("interval interval = %s", interval)
		}
	default:
		t.Fatal("heartbeat update was not published")
	}
}
