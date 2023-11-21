package bmsclient_test

import bmsclient "github.com/botnet-monitoring/grpc-client"

func ExampleNewSimpleClient() {
	client, err := bmsclient.NewSimpleClient("your-bms-server.local:8083", "some-monitor-id", "AAAA/some+example+token/AAAAAAAAAAAAAAAAAAA=", "some-botnet-id")
	if err != nil {
		panic(err)
	}

	// Do something with the client
	_ = client
}

func ExampleWithSessionOptions() {
	client, err := bmsclient.NewSimpleClient(
		"your-bms-server.local:8083",
		"some-monitor-id",
		"AAAA/some+example+token/AAAAAAAAAAAAAAAAAAA=",
		"some-botnet-id",

		// Pass it to NewSimpleClient on creation and wrap one or multiple session options in it
		bmsclient.WithSessionOptions(
			bmsclient.WithCampaign("some-campaign"),
		),
	)
	if err != nil {
		panic(err)
	}

	// Do something with the client
	_ = client
}
