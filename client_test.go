package bmsclient_test

import bmsclient "github.com/botnet-monitoring/grpc-client"

func ExampleNewClient() {
	client, err := bmsclient.NewClient("your-bms-server.local:8083", "some-monitor-id", "AAAA/some+example+token/AAAAAAAAAAAAAAAAAAA=")
	if err != nil {
		panic(err)
	}

	// Do something with the client
	_ = client
}
