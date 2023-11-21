package bmsclient_test

import (
	"context"
	"net"
	"sync"
	"time"

	bmsclient "github.com/botnet-monitoring/grpc-client"
)

func ExampleClient_NewSession() {
	client, err := bmsclient.NewClient("your-bms-server.local:8083", "some-monitor-id", "AAAA/some+example+token/AAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		panic(err)
	}

	session, err := client.NewSession("some-botnet-id")
	if err != nil {
		panic(err)
	}

	// Do something with the session
	_ = session
}

func ExampleClient_NewSession_withOptions() {
	client, err := bmsclient.NewClient(
		"your-bms-server.local:8083",
		"some-monitor-id",
		"AAAA/some+example+token/AAAAAAAAAAAAAAAAAAA=",
	)
	if err != nil {
		panic(err)
	}

	otherData := struct {
		Some string `json:"some_field"`
	}{
		Some: "value",
	}

	session, err := client.NewSession(
		"some-botnet-id",
		bmsclient.WithCampaign("some-campaign"),
		bmsclient.WithFixedFrequency(30),
		bmsclient.WithAdditionalConfig(otherData),
		bmsclient.WithIP(net.ParseIP("198.51.100.1")),
		bmsclient.WithPort(10101),
	)
	if err != nil {
		panic(err)
	}

	// Do something with the session
	_ = session
}

func ExampleSession_SendBotReplyBatch() {
	// Create a client
	client, err := bmsclient.NewClient("your-bms-server.local:8083", "some-monitor-id", "AAAA/some+example+token/AAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		panic(err)
	}

	// Create a session
	session, err := client.NewSession("some-botnet-id")
	if err != nil {
		panic(err)
	}

	// Create a batch
	batch := &bmsclient.BotReplyBatch{
		&bmsclient.BotReply{
			Timestamp: time.Now(),
		},
	}

	// Send the batch
	_, err = session.SendBotReplyBatch(batch)
	if err != nil {
		panic(err)
	}

	// End the session because we're done
	err = session.End()
	if err != nil {
		panic(err)
	}
}

func ExampleSession_SendBotReplyBatchWithContext() {
	// Create a client
	client, err := bmsclient.NewClient("your-bms-server.local:8083", "some-monitor-id", "AAAA/some+example+token/AAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		panic(err)
	}

	// Create a session
	session, err := client.NewSession("some-botnet-id")
	if err != nil {
		panic(err)
	}

	// Create a context that times out after 500ms
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		// Add the Go routine below to the wait group
		wg.Add(1)

		// Create a batch to send
		batch := &bmsclient.BotReplyBatch{
			&bmsclient.BotReply{
				Timestamp: time.Now(),
				IP:        net.ParseIP("192.0.2.1"),
				Port:      uint16(i), // Use the index variable here to make sure the bot reply is in fact different
			},
		}

		// Send the batch in the background
		go func() {
			// Mark this Go routine as finished when it returns
			defer wg.Done()

			// Send the batch
			_, err = session.SendBotReplyBatchWithContext(ctx, batch)
			if err != nil {
				panic(err)
			}
		}()
	}

	// Wait for all Go routines to finish
	wg.Wait()

	// End the session because we're done
	err = session.End()
	if err != nil {
		panic(err)
	}

	// Note that we use a wait group here, although End by default is configured to wait for all sent batches to be acknowledged.
	// This is necessary because (depending on how the Go routine is scheduled) it can happen that End locks the sending stream before Send began sending.
	// If using this example without the wait group, it can happen that not all batches will be sent (and it depends on your use case if that's ok for you).
}
