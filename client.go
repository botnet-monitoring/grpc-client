package bmsclient

import (
	"encoding/base64"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	bmsapi "github.com/botnet-monitoring/grpc-api/gen"
)

// The Client struct represents a BMS client.
// A Client can have multiple sessions that can send data to BMS.
//
// To create a Client, use [NewClient].
type Client struct {
	grpcStorageClient bmsapi.BMSStorageServiceClient

	monitorID string
	authToken [32]byte
}

// Functions which implement ClientOption can be passed to [NewClient] as additional options.
//
// Currently there are no implementations of it.
type ClientOption func(*Client)

// NewClient creates a new BMS client based on the given parameters.
//
// It will dial the gRPC connection (to check if a compatible server is there), but not start to authenticate until you start a session with [Client.NewSession].
// Dialing the gRPC connection may block up to 20 seconds (the default connect timeout).
//
// In theory you could pass additional configuration via options, but there are currently no implementations of [ClientOption].
//
// The passed auth token must be base64-encoded and exactly 32 bytes long.
func NewClient(bmsServer string, monitorID string, authToken string, options ...ClientOption) (*Client, error) {
	authTokenSlice, err := base64.StdEncoding.DecodeString(string(authToken))
	if err != nil {
		return nil, err
	}

	if len(authTokenSlice) != 32 {
		return nil, errors.New("parameter authToken needs to be exactly 32 bytes long")
	}
	var authTokenArray [32]byte
	copy(authTokenArray[:], authTokenSlice)

	client := &Client{
		monitorID: monitorID,
		authToken: authTokenArray,
	}

	for _, option := range options {
		option(client)
	}

	conn, err := grpc.Dial(
		bmsServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	client.grpcStorageClient = bmsapi.NewBMSStorageServiceClient(conn)

	return client, nil
}
