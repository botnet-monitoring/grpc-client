// This example is used in the README.
// If you change it, make sure to update the README too.

package bmsclient_test

import (
	"net"
	"time"

	bmsclient "github.com/botnet-monitoring/grpc-client"
)

// A simple example that uses the simple client.
// The simple client internally creates a session and thereby combines a client with exactly one session for easier usage.
func Example_simple() {
	// Create a simple client (which is a client that automatically starts a session)
	client, err := bmsclient.NewSimpleClient(
		"your-bms-server.local:8083",
		"some-monitor-id",
		"AAAA/some+example+token/AAAAAAAAAAAAAAAAAAA=",
		"some-botnet-id",
	)
	if err != nil {
		panic(err)
	}

	// You can send data in batches and send it synchronously
	batch := bmsclient.BotReplyBatch{
		&bmsclient.BotReply{
			Timestamp: time.Now(),
			IP:        net.ParseIP("192.0.2.1"),
			Port:      10001,
		},
		&bmsclient.BotReply{
			Timestamp: time.Now(),
			IP:        net.ParseIP("192.0.2.2"),
			Port:      10002,
		},
	}
	_, err = client.SendBotReplyBatch(&batch) // this blocks
	if err != nil {
		panic(err)
	}

	// Or add it to the client as you gather it and flush it at once
	client.AddBotReply(time.Now(), net.ParseIP("192.0.2.3"), 10003, "", nil)
	client.AddBotReply(time.Now(), net.ParseIP("192.0.2.4"), 10004, "", nil)
	err = client.Flush() // this blocks
	if err != nil {
		panic(err)
	}

	// If you're done, you can end the session
	err = client.End()
	if err != nil {
		panic(err)
	}
}
