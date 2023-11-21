package bmsclient_test

import (
	"net"
	"time"

	bmsclient "github.com/botnet-monitoring/grpc-client"
)

// An advanced example that creates the session separately from the client.
func Example_advanced() {
	// First, create a client
	client, err := bmsclient.NewClient(
		"your-bms-server.local:8083",
		"some-monitor-id",
		"AAAA/some+example+token/AAAAAAAAAAAAAAAAAAA=",
	)
	if err != nil {
		panic(err)
	}

	// Then, add a new session to the client
	session, err := client.NewSession("some-botnet-id")
	if err != nil {
		panic(nil)
	}

	// Now you can send data on the created session
	// For example synchronously in batches
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
	_, err = session.SendBotReplyBatch(&batch) // this blocks
	if err != nil {
		panic(err)
	}

	// Or add it to the client as you gather it and flush it at once
	session.AddBotReply(time.Now(), net.ParseIP("192.0.2.3"), 10003, "", nil)
	session.AddBotReply(time.Now(), net.ParseIP("192.0.2.4"), 10004, "", nil)
	err = session.Flush() // this blocks
	if err != nil {
		panic(err)
	}

	// If you're done, you can end the session
	err = session.End()
	if err != nil {
		panic(err)
	}
}
