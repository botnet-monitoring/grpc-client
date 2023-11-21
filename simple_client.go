package bmsclient

import (
	"net"
	"time"
)

// SimpleClient is a simple to use BMS client.
//
// It creates a client internally and initiates a session.
// If you need multiple sessions in the same client, use the [Client].
//
// To create a SimpleClient, use [NewSimpleClient].
type SimpleClient struct {
	client  *Client
	session *Session

	clientOptions  []ClientOption
	sessionOptions []SessionOption
}

// Functions which implement SimpleClientOption can be passed to [NewSimpleClient] as additional options.
type SimpleClientOption func(*SimpleClient)

// NewSimpleClient creates a simple to use BMS client based on the given parameters.
//
// The simple client does not distinguish between client and sessions and just creates a session for you internally.
// If you're done sending, you can call [SimpleClient.End] (and similar methods) to make sure all data is sent.
//
// While you don't have to do anything with the session token (will be attached internally to all batches), you have to ensure that the session stays active by not letting the session timeout elapse.
// You can get the session timeout via [SimpleClient.GetSessionTimeout].
//
// The passed auth token must be base64-encoded and exactly 32 bytes long.
func NewSimpleClient(bmsServer string, monitorID string, authToken string, botnetID string, options ...SimpleClientOption) (*SimpleClient, error) {
	simpleClient := &SimpleClient{}

	for _, option := range options {
		option(simpleClient)
	}

	client, err := NewClient(bmsServer, monitorID, authToken, simpleClient.clientOptions...)
	if err != nil {
		return nil, err
	}
	session, err := client.NewSession(botnetID, simpleClient.sessionOptions...)
	if err != nil {
		return nil, err
	}

	// Embed client and session into simple client
	simpleClient.client = client
	simpleClient.session = session

	// Since all the options are now embedded via client or session, we do not need these anymore
	simpleClient.clientOptions = nil
	simpleClient.sessionOptions = nil

	return simpleClient, nil
}

// WithClientOptions can be used to pass one or multiple instances of [ClientOption] to [NewSimpleClient] to configure its internal client.
//
// As there currently are no implementations for [ClientOption], this option is rather useless.
func WithClientOptions(options ...ClientOption) SimpleClientOption {
	return func(client *SimpleClient) {
		client.clientOptions = options
	}
}

// WithSessionOptions can be used to pass one or multiple instances of [SessionOption] to [NewSimpleClient] to configure its internal session.
func WithSessionOptions(options ...SessionOption) SimpleClientOption {
	return func(client *SimpleClient) {
		client.sessionOptions = options
	}
}

// GetSessionToken is a wrapper around [Session.GetSessionToken].
func (s *SimpleClient) GetSessionToken() [4]byte {
	return s.session.GetSessionToken()
}

// GetSessionTimeout is a wrapper around [Session.GetSessionTimeout].
func (s *SimpleClient) GetSessionTimeout() time.Duration {
	return s.session.GetSessionTimeout()
}

// GetLastBotReplyTime is a wrapper around [Session.GetLastBotReplyTime].
func (s *SimpleClient) GetLastBotReplyTime() time.Time {
	return s.session.GetLastBotReplyTime()
}

// GetLastFailedTryTime is a wrapper around [Session.GetLastFailedTryTime].
func (s *SimpleClient) GetLastFailedTryTime() time.Time {
	return s.session.GetLastFailedTryTime()
}

// GetLastEdgeTime is a wrapper around [Session.GetLastEdgeTime].
func (s *SimpleClient) GetLastEdgeTime() time.Time {
	return s.session.GetLastEdgeTime()
}

// End is a wrapper around [Session.End].
func (s *SimpleClient) End() error {
	return s.session.End()
}

// EndWithReason is a wrapper around [Session.EndWithReason].
func (s *SimpleClient) EndWithReason(reason DisconnectReason) error {
	return s.session.EndWithReason(reason)
}

// SendBotReplyBatch is a wrapper around [Session.SendBotReplyBatch].
func (s *SimpleClient) SendBotReplyBatch(botReplyBatch *BotReplyBatch) (uint32, error) {
	return s.session.SendBotReplyBatch(botReplyBatch)
}

// SendFailedTryBatch is a wrapper around [Session.SendFailedTryBatch].
func (s *SimpleClient) SendFailedTryBatch(failedTryBatch *FailedTryBatch) (uint32, error) {
	return s.session.SendFailedTryBatch(failedTryBatch)
}

// SendEdgeBatch is a wrapper around [Session.SendEdgeBatch].
func (s *SimpleClient) SendEdgeBatch(edgeBatch *EdgeBatch) (uint32, error) {
	return s.session.SendEdgeBatch(edgeBatch)
}

// AddBotReply is a wrapper around [Session.AddBotReply].
func (s *SimpleClient) AddBotReply(timestamp time.Time, ip net.IP, port uint16, botID string, otherData interface{}) {
	s.session.AddBotReply(timestamp, ip, port, botID, otherData)
}

// AddFailedTry is a wrapper around [Session.AddFailedTry].
func (s *SimpleClient) AddFailedTry(timestamp time.Time, ip net.IP, port uint16, botID string, reason string, otherData interface{}) {
	s.session.AddFailedTry(timestamp, ip, port, botID, reason, otherData)
}

// AddEdge is a wrapper around [Session.AddEdge].
func (s *SimpleClient) AddEdge(timestamp time.Time, srcIP net.IP, srcPort uint16, srcBotID string, dstIP net.IP, dstPort uint16, dstBotID string) {
	s.session.AddEdge(timestamp, srcIP, srcPort, srcBotID, dstIP, dstPort, dstBotID)
}

// Flush is a wrapper around [Session.Flush].
func (s *SimpleClient) Flush() error {
	return s.session.Flush()
}

// FlushBotReplies is a wrapper around [Session.FlushBotReplies].
func (s *SimpleClient) FlushBotReplies() error {
	return s.session.FlushBotReplies()
}

// FlushFailedTries is a wrapper around [Session.FlushFailedTries].
func (s *SimpleClient) FlushFailedTries() error {
	return s.session.FlushFailedTries()
}

// FlushEdges is a wrapper around [Session.FlushEdges].
func (s *SimpleClient) FlushEdges() error {
	return s.session.FlushEdges()
}
