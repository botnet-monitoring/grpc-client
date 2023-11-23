package bmsclient

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	bmsapi "github.com/botnet-monitoring/grpc-api/gen"

	"github.com/botnet-monitoring/grpc-client/hooks"
	bmsencoding "github.com/botnet-monitoring/grpc-client/internal/encoding"
)

// The Session struct represents a session of a BMS client.
//
// To create a Session, use [Client.NewSession].
type Session struct {
	client Client

	botnetID         string
	campaignIDPtr    *string
	frequencyPtr     *uint32
	publicIPPtr      *net.IP
	monitorPortPtr   *uint16
	additionalConfig interface{}

	ignoreServerAcks bool

	sessionToken   [4]byte
	sessionTimeout time.Duration

	lastBotReplyTime  time.Time
	lastFailedTryTime time.Time
	lastEdgeTime      time.Time

	cancelStreams context.CancelFunc

	botReplyBatchCounter  uint32
	failedTryBatchCounter uint32
	edgeBatchCounter      uint32

	botReplyStreamSendMutex  semaphore.Weighted
	failedTryStreamSendMutex semaphore.Weighted
	edgeStreamSendMutex      semaphore.Weighted

	botReplyStream  bmsapi.BMSStorageService_StoreDatedBotReplyBatchClient
	failedTryStream bmsapi.BMSStorageService_StoreDatedFailedTryBatchClient
	edgeStream      bmsapi.BMSStorageService_StoreDatedEdgeBatchClient

	receivedBotReplyBatchIDs  sync.Map
	receivedFailedTryBatchIDs sync.Map
	receivedEdgeBatchIDs      sync.Map

	unsentBotReplyBatchMutex  sync.Mutex
	unsentFailedTryBatchMutex sync.Mutex
	unsentEdgeBatchMutex      sync.Mutex

	unsentBotReplyBatch  BotReplyBatch
	unsentFailedTryBatch FailedTryBatch
	unsentEdgeBatch      EdgeBatch

	botReplyBatchResponseSuccessHooks  []hooks.BotReplyBatchResponseSuccessHook
	failedTryBatchResponseSuccessHooks []hooks.FailedTryBatchResponseSuccessHook
	edgeBatchResponseSuccessHooks      []hooks.EdgeBatchResponseSuccessHook
	botReplyBatchResponseFailureHooks  []hooks.BotReplyBatchResponseFailureHook
	failedTryBatchResponseFailureHooks []hooks.FailedTryBatchResponseFailureHook
	edgeBatchResponseFailureHooks      []hooks.EdgeBatchResponseFailureHook
}

// Functions which implement SessionOption can be passed to [Client.NewSession] as additional options.
type SessionOption func(*Session)

// Possible reasons a client wants to end the session with.
type DisconnectReason bmsapi.DisconnectReason

const (
	// The client wants to specify no reason to end the session.
	DisconnectReasonUnspecified DisconnectReason = 0
	// The client has done its purpose and thereby wants to end the session.
	DisconnectReasonFinished DisconnectReason = 1
	// The clients wants to end the session in order to reconnect soon.
	DisconnectReasonBeRightBack DisconnectReason = 2
	// The clients wants to end the session in order to reconnect soon with a new configuration.
	DisconnectReasonBeRightBackWithNewConfig DisconnectReason = 3
	// The client had some error and thereby wants to the the session.
	DisconnectReasonClientError DisconnectReason = 4
	// The client wants to end the session because of some other reason.
	DisconnectReasonOther DisconnectReason = 5
)

// NewSession creates a new session for the BMS client it was called on.
//
// Calling NewSession will register the client's session with the BMS server.
// The client tells the server its configuration, the server checks whether the configuration is valid (and conforms with the campaign if the client has set one), then the server will respond with a session token and a session timeout.
//
// While a session is in progress, all settings are fixed.
// If the client wants to change its configuration (e.g. switching the botnet it monitors), it has to end the current session and start a new one.
// You can end a session with [Session.End] (and similar methods).
//
// While you don't have to do anything with the session token (will be attached internally to all batches), you have to ensure that the session stays active by not letting the session timeout elapse.
// You can get the session timeout via [Session.GetSessionTimeout].
func (c *Client) NewSession(botnetID string, options ...SessionOption) (*Session, error) {
	session := &Session{
		client:   *c,
		botnetID: botnetID,

		botReplyStreamSendMutex:  *semaphore.NewWeighted(1),
		edgeStreamSendMutex:      *semaphore.NewWeighted(1),
		failedTryStreamSendMutex: *semaphore.NewWeighted(1),
	}

	for _, option := range options {
		option(session)
	}

	sessionToken, sessionTimeout, lastBotReplyTime, lastFailedTryTime, lastEdgeTime, err := session.registerSessionViaGRPC()
	if err != nil {
		return nil, err
	}
	session.sessionToken = sessionToken
	session.sessionTimeout = sessionTimeout
	if lastBotReplyTime == nil {
		session.lastBotReplyTime = time.Time{}
	} else {
		session.lastBotReplyTime = *lastBotReplyTime
	}
	if lastFailedTryTime == nil {
		session.lastFailedTryTime = time.Time{}
	} else {
		session.lastFailedTryTime = *lastFailedTryTime
	}
	if lastEdgeTime == nil {
		session.lastEdgeTime = time.Time{}
	} else {
		session.lastEdgeTime = *lastEdgeTime
	}

	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, "session-token-bin", string(sessionToken[:]))
	ctx, cancel := context.WithCancel(ctx)
	session.cancelStreams = cancel

	botReplyStream, err := c.grpcStorageClient.StoreDatedBotReplyBatch(ctx)
	if err != nil {
		return nil, err
	}
	session.botReplyStream = botReplyStream

	edgeStream, err := c.grpcStorageClient.StoreDatedEdgeBatch(ctx)
	if err != nil {
		return nil, err
	}
	session.edgeStream = edgeStream

	failedTryStream, err := c.grpcStorageClient.StoreDatedFailedTryBatch(ctx)
	if err != nil {
		return nil, err
	}
	session.failedTryStream = failedTryStream

	// These will be cancelled automatically when closing the connection or cancelling their context
	go session.botReplyResponseHandler()
	go session.edgeResponseHandler()
	go session.failedTryResponseHandler()

	return session, nil
}

func (s *Session) botReplyResponseHandler() {
	for {
		response, err := s.botReplyStream.Recv()

		if err == nil {
			for _, hook := range s.botReplyBatchResponseSuccessHooks {
				hook(response)
			}

			if !s.ignoreServerAcks {
				s.receivedBotReplyBatchIDs.Store(response.BatchId, true)
			}
		} else {
			if status.Code(err) == codes.Canceled {
				// We do not consider this an error, see https://pkg.go.dev/google.golang.org/grpc/codes#Canceled
				break
			}

			for _, hook := range s.botReplyBatchResponseFailureHooks {
				hook(err)
			}
			break
		}
	}
}

func (s *Session) edgeResponseHandler() {
	for {
		response, err := s.edgeStream.Recv()

		if err == nil {
			for _, hook := range s.edgeBatchResponseSuccessHooks {
				hook(response)
			}

			if !s.ignoreServerAcks {
				s.receivedEdgeBatchIDs.Store(response.BatchId, true)
			}
		} else {
			if status.Code(err) == codes.Canceled {
				// We do not consider this an error, see https://pkg.go.dev/google.golang.org/grpc/codes#Canceled
				break
			}

			for _, hook := range s.edgeBatchResponseFailureHooks {
				hook(err)
			}
			break
		}
	}
}

func (s *Session) failedTryResponseHandler() {
	for {
		response, err := s.failedTryStream.Recv()

		if err == nil {
			for _, hook := range s.failedTryBatchResponseSuccessHooks {
				hook(response)
			}

			if !s.ignoreServerAcks {
				s.receivedFailedTryBatchIDs.Store(response.BatchId, true)
			}
		} else {
			if status.Code(err) == codes.Canceled {
				// We do not consider this an error, see https://pkg.go.dev/google.golang.org/grpc/codes#Canceled
				break
			}

			for _, hook := range s.failedTryBatchResponseFailureHooks {
				hook(err)
			}
			break
		}
	}
}

// WithCampaign can be used to set a campaign for the session.
//
// Note that when being part of a campaign, the server will check whether the session configuration conforms with the campaign configuration (e.g. monitoring the correct botnet, using the correct frequency).
func WithCampaign(campaignID string) SessionOption {
	return func(session *Session) {
		session.campaignIDPtr = &campaignID
	}
}

// WithFixedFrequency can be used to set the frequency for the session.
//
// This is mostly useful for crawlers (as sensors don't actively contact bots).
// The frequency is passed as seconds that the crawler waits between contacting (or trying to contact) the same bot again.
func WithFixedFrequency(frequency uint32) SessionOption {
	return func(session *Session) {
		session.frequencyPtr = &frequency
	}
}

// WithIP can be used to set the public IP address of the monitor for the session.
//
// This is for example useful for sensors that just listen for incoming traffic (as the IP address might influence the measurements or can be used to identify the sensor in the peerlists of other bots).
func WithIP(publicIP net.IP) SessionOption {
	return func(session *Session) {
		session.publicIPPtr = &publicIP
	}
}

// WithPort can be used to set the port that a monitor listens on for incoming bot traffic.
//
// This is for example useful for monitors that listen for incoming traffic on a fixed port.
// It can also be used to distinguish between several monitors that run on the same host (and therefore share the same IP address) but use different fixed ports.
func WithPort(monitorPort uint16) SessionOption {
	return func(session *Session) {
		session.monitorPortPtr = &monitorPort
	}
}

// WithAdditionalConfig can be used to set arbitrary additional config for the session.
//
// Please also note that this goes through [encoding/json.Marshal], so fields that should go into the database should be exported.
// This also means you can use field tags like `json:"some_field_name"`.
func WithAdditionalConfig(config interface{}) SessionOption {
	return func(session *Session) {
		session.additionalConfig = config
	}
}

// WithIgnoreServerAcks can be used to ignore the acknowledgements of sent batches that the server received.
//
// By default, sending batches will wait for the server to acknowledge the reception of a batch.
// While the send method waits for the server to acknowledge, it will block.
// Please note that this might take long (or in case of an error even forever), so it is recommended to pass a context with timeout (e.g. by using [Session.SendBotReplyBatchWithContext]).
//
// If you ignore server acks, it might happen that a batch isn't sent before the program exits.
// This is because the underlying [google.golang.org/grpc.ClientStream.SendMsg] returns as soon as the message is scheduled to sent (but not actually sent yet).
// See their documentation for more information.
func WithIgnoreServerAcks(ignoreServerAcks bool) SessionOption {
	return func(session *Session) {
		session.ignoreServerAcks = ignoreServerAcks
	}
}

// WithBotReplyBatchResponseSuccessHook can be used to pass a function that will be executed when a server ack for a bot reply batch is received.
func WithBotReplyBatchResponseSuccessHook(hook hooks.BotReplyBatchResponseSuccessHook) SessionOption {
	return func(session *Session) {
		session.botReplyBatchResponseSuccessHooks = append(session.botReplyBatchResponseSuccessHooks, hook)
	}
}

// WithEdgeBatchResponseSuccessHook can be used to pass a function that will be executed when a server ack for an edge batch is received.
func WithEdgeBatchResponseSuccessHook(hook hooks.EdgeBatchResponseSuccessHook) SessionOption {
	return func(session *Session) {
		session.edgeBatchResponseSuccessHooks = append(session.edgeBatchResponseSuccessHooks, hook)
	}
}

// WithFailedTryBatchResponseSuccessHook can be used to pass a function that will be executed when a server ack for failed try batch is received.
func WithFailedTryBatchResponseSuccessHook(hook hooks.FailedTryBatchResponseSuccessHook) SessionOption {
	return func(session *Session) {
		session.failedTryBatchResponseSuccessHooks = append(session.failedTryBatchResponseSuccessHooks, hook)
	}
}

// WithBotReplyBatchResponseFailureHook can be used to pass a function that will be executed when a server error on the bot reply batch stream is received.
//
// Please note that the stream will be closed after the first error.
// So the hook will be executed only once.
func WithBotReplyBatchResponseFailureHook(hook hooks.BotReplyBatchResponseFailureHook) SessionOption {
	return func(session *Session) {
		session.botReplyBatchResponseFailureHooks = append(session.botReplyBatchResponseFailureHooks, hook)
	}
}

// WithEdgeBatchResponseFailureHook can be used to pass a function that will be executed when a server error on the edge batch stream is received.
//
// Please note that the stream will be closed after the first error.
// So the hook will be executed only once.
func WithEdgeBatchResponseFailureHook(hook hooks.EdgeBatchResponseFailureHook) SessionOption {
	return func(session *Session) {
		session.edgeBatchResponseFailureHooks = append(session.edgeBatchResponseFailureHooks, hook)
	}
}

// WithFailedTryBatchResponseFailureHook can be used to pass a function that will be executed when a server error on the failed try batch stream is received.
//
// Please note that the stream will be closed after the first error.
// So the hook will be executed only once.
func WithFailedTryBatchResponseFailureHook(hook hooks.FailedTryBatchResponseFailureHook) SessionOption {
	return func(session *Session) {
		session.failedTryBatchResponseFailureHooks = append(session.failedTryBatchResponseFailureHooks, hook)
	}
}

func (s *Session) registerSessionViaGRPC() (sessionToken [4]byte, sessionTimeout time.Duration, lastBotReplyTime *time.Time, lastFailedTryTime *time.Time, lastEdgeTime *time.Time, err error) {
	publicIP := bmsencoding.OptionalIPToWrapper(s.publicIPPtr)
	monitorPort := bmsencoding.OptionalPortToWrapper(s.monitorPortPtr)
	configJSON, err := bmsencoding.StructToJSONWrapper(s.additionalConfig)
	if err != nil {
		return [4]byte{}, time.Duration(0), nil, nil, nil, err
	}

	registerRequest := &bmsapi.RegisterSessionRequest{
		BotnetId:    s.botnetID,
		CampaignId:  s.campaignIDPtr,
		Frequency:   s.frequencyPtr,
		PublicIp:    publicIP,
		MonitorPort: monitorPort,
		ConfigJson:  configJSON,
	}

	ctx := context.Background()
	ctxWithAuthCredentials := metadata.AppendToOutgoingContext(ctx, "auth-monitor-id", s.client.monitorID, "auth-token-bin", string(s.client.authToken[:]))

	registerResponse, err := s.client.grpcStorageClient.RegisterSession(ctxWithAuthCredentials, registerRequest)
	if err != nil {
		return [4]byte{}, time.Duration(0), nil, nil, nil, err
	}

	copy(sessionToken[:], registerResponse.SessionToken.Token)

	sessionTimeout = time.Duration(registerResponse.SessionTimeout) * time.Second

	lastBotReplyTime = bmsencoding.OptionalTimestampToTime(registerResponse.LastInsertedBotReply)
	lastFailedTryTime = bmsencoding.OptionalTimestampToTime(registerResponse.LastInsertedFailedTry)
	lastEdgeTime = bmsencoding.OptionalTimestampToTime(registerResponse.LastInsertedEdge)

	return sessionToken, sessionTimeout, lastBotReplyTime, lastFailedTryTime, lastEdgeTime, nil
}

// GetSessionToken returns the session token that the server set for the session.
//
// It should not be needed to do anything with the session token, as the client internally attaches it to its messages to the server.
func (s *Session) GetSessionToken() [4]byte {
	return s.sessionToken
}

// GetSessionTimeout returns the timeout that the server set for the session.
//
// After inactivity longer than the timeout the server may close the session.
// This doesn't mean that the server closes the session right after the timeout elapses, but it's guaranteed to not be closed before that.
// Sending a batch (even an empty one) every time right before the timeout elapses is enough to renew the session.
func (s *Session) GetSessionTimeout() time.Duration {
	return s.sessionTimeout
}

// GetLastBotReplyTime returns the time the last bot reply batch was successfully received by the server (if any).
//
// This method can be used to implement local caching when the server goes offline temporarily.
func (s *Session) GetLastBotReplyTime() time.Time {
	return s.lastBotReplyTime
}

// GetLastEdgeTime returns the time the last edge batch was successfully received by the server (if any).
//
// This method can be used to implement local caching when the server goes offline temporarily.
func (s *Session) GetLastEdgeTime() time.Time {
	return s.lastEdgeTime
}

// GetLastFailedTryTime returns the time the last failed try batch was successfully received by the server (if any).
//
// This method can be used to implement local caching when the server goes offline temporarily.
func (s *Session) GetLastFailedTryTime() time.Time {
	return s.lastFailedTryTime
}

// End ends a session.
//
// It will flush all unset batches (e.g. bot replies added via [Session.AddBotReply]) and then communicate to the server that we're not intending to send any further data.
// Additionally, it will wait for all sent batches to be acknowledged.
// While flushing and waiting, this method blocks (use [Session.EndWithContext] if you want to set a timeout).
func (s *Session) End() error {
	return s.EndWithReason(DisconnectReasonUnspecified)
}

// EndWithContext is like [Session.End] but takes a context which for example can be used to cancel it after some timeout.
func (s *Session) EndWithContext(ctx context.Context) error {
	return s.EndWithContextAndReason(ctx, DisconnectReasonUnspecified)
}

// EndWithReason is like [Session.End] but takes a reason why the session should end.
func (s *Session) EndWithReason(reason DisconnectReason) error {
	return s.EndWithContextAndReason(context.Background(), reason)
}

// EndWithContextAndReason is a mix out of [Session.EndWithContext] and [Session.EndWithReason].
func (s *Session) EndWithContextAndReason(ctx context.Context, reason DisconnectReason) error {
	// Make sure we're eventually canceling the streams to not leak any Go routines
	defer s.cancelStreams()

	// Calling with keepLocked=true will keep the send and receive locks (so that nobody can add data while we're trying to flush one last time)
	errReplies := s.flushBotRepliesWithContext(ctx, true)
	errEdges := s.flushEdgesWithContext(ctx, true)
	errTries := s.flushFailedTriesWithContext(ctx, true)

	// Now also lock send and receive mutexes to keep other threads from trying to send more batches
	err := s.botReplyStreamSendMutex.Acquire(ctx, 1)
	if err != nil {
		// Context was canceled or exceeded deadline
		return err
	}
	err = s.edgeStreamSendMutex.Acquire(ctx, 1)
	if err != nil {
		return err
	}
	err = s.failedTryStreamSendMutex.Acquire(ctx, 1)
	if err != nil {
		return err
	}

	// Check errors when tried to flush all, just to make it a little bit more likely that data is flushed in case of errors
	if errReplies != nil {
		return errReplies
	}
	if errEdges != nil {
		return errEdges
	}
	if errTries != nil {
		return errTries
	}

	// If we wait for server acks, check that all batches are acknowledged before continuing
	if !s.ignoreServerAcks {
		for {
			var isEmpty bool = true

			s.receivedBotReplyBatchIDs.Range(func(key, value any) bool {
				isEmpty = false
				return false
			})

			if isEmpty {
				break
			}

			if ctx.Err() != nil {
				return err
			}

			time.Sleep(500 * time.Microsecond)
		}
	}

	ctxWithSessionCredentials := metadata.AppendToOutgoingContext(ctx, "session-token-bin", string(s.sessionToken[:]))

	request := &bmsapi.DisconnectRequest{
		Reason: convertToBMSAPIEnum(reason),
	}

	_, err = s.client.grpcStorageClient.Disconnect(ctxWithSessionCredentials, request)
	if err != nil {
		return err
	}

	s.botnetID = ""
	s.sessionTimeout = 0
	s.sessionToken = [4]byte{}

	return nil
}

// SendBotReplyBatch sends the given bot reply batch synchronously.
//
// By default (see [WithIgnoreServerAcks]) it will wait for the server to acknowledge sent batches.
// While it waits, the method will block.
// In case of a server error, it might even wait forever.
// Therefore you likely want to use [Session.SendBotReplyBatchWithContext] to set a timeout.
func (s *Session) SendBotReplyBatch(botReplyBatch *BotReplyBatch) (uint32, error) {
	return s.SendBotReplyBatchWithContext(context.Background(), botReplyBatch)
}

// SendBotReplyBatchWithContext is like [Session.SendBotReplyBatch] but takes an additional context (e.g. to be able to cancel it when executing as a Go routine).
func (s *Session) SendBotReplyBatchWithContext(ctx context.Context, botReplyBatch *BotReplyBatch) (uint32, error) {
	var request bmsapi.StoreDatedBotReplyBatchRequest
	var batch []*bmsapi.DatedBotReply

	batchID := atomic.AddUint32(&s.botReplyBatchCounter, 1)
	request.BatchId = batchID

	var otherData *bmsapi.JSON
	var err error
	for _, given := range *botReplyBatch {
		var own bmsapi.DatedBotReply

		own.Timestamp = bmsencoding.TimeToWrapper(given.Timestamp)
		own.BotId = bmsencoding.StringToStringPtr(given.BotID)
		own.Ip = bmsencoding.IPToWrapper(given.IP)
		own.Port = bmsencoding.PortToWrapper(given.Port)
		otherData, err = bmsencoding.StructToJSONWrapper(given.OtherData)
		if err != nil {
			return 0, err
		}
		own.OtherData = otherData

		batch = append(batch, &own)

		// Context was canceled or deadline is exceeded
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
	}

	request.Replies = batch

	err = s.botReplyStreamSendMutex.Acquire(ctx, 1)
	if err != nil {
		// Context was canceled or exceeded deadline
		return 0, err
	}
	err = s.botReplyStream.Send(&request)
	if err != nil {
		return 0, err
	}
	s.botReplyStreamSendMutex.Release(1)

	if !s.ignoreServerAcks {
		var deleted bool
		for {
			deleted = s.receivedBotReplyBatchIDs.CompareAndDelete(batchID, true)
			if deleted {
				break
			}
			if ctx.Err() != nil {
				return 0, err
			}
			time.Sleep(500 * time.Microsecond)
		}
	}

	return batchID, nil
}

// SendFailedTryBatch sends the given failed try batch synchronously.
//
// By default (see [WithIgnoreServerAcks]) it will wait for the server to acknowledge sent batches.
// While it waits, the method will block.
// In case of a server error, it might even wait forever.
// Therefore you likely want to use [Session.SendFailedTryBatchWithContext] to set a timeout.
func (s *Session) SendFailedTryBatch(failedTryBatch *FailedTryBatch) (uint32, error) {
	return s.SendFailedTryBatchWithContext(context.Background(), failedTryBatch)
}

// SendFailedTryBatchWithContext is like [Session.SendFailedTryBatch] but takes an additional context (e.g. to be able to cancel it when executing as a Go routine).
func (s *Session) SendFailedTryBatchWithContext(ctx context.Context, failedTryBatch *FailedTryBatch) (uint32, error) {
	var request bmsapi.StoreDatedFailedTryBatchRequest
	var batch []*bmsapi.DatedFailedTry

	batchID := atomic.AddUint32(&s.failedTryBatchCounter, 1)
	request.BatchId = batchID

	var otherData *bmsapi.JSON
	var err error
	for _, given := range *failedTryBatch {
		var own bmsapi.DatedFailedTry

		own.Timestamp = bmsencoding.TimeToWrapper(given.Timestamp)
		own.BotId = bmsencoding.StringToStringPtr(given.BotID)
		own.Ip = bmsencoding.IPToWrapper(given.IP)
		own.Port = bmsencoding.PortToWrapper(given.Port)
		own.Reason = bmsencoding.StringToStringPtr(given.Reason)
		otherData, err = bmsencoding.StructToJSONWrapper(given.OtherData)
		if err != nil {
			return 0, err
		}
		own.OtherData = otherData

		batch = append(batch, &own)

		// Context was canceled or deadline is exceeded
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
	}

	request.Tries = batch

	err = s.failedTryStreamSendMutex.Acquire(ctx, 1)
	if err != nil {
		// Context was canceled or exceeded deadline
		return 0, err
	}
	err = s.failedTryStream.Send(&request)
	if err != nil {
		return 0, err
	}
	s.failedTryStreamSendMutex.Release(1)

	if !s.ignoreServerAcks {
		var deleted bool
		for {
			deleted = s.receivedFailedTryBatchIDs.CompareAndDelete(batchID, true)
			if deleted {
				break
			}
			if ctx.Err() != nil {
				return 0, err
			}
			time.Sleep(500 * time.Microsecond)
		}
	}

	return batchID, nil
}

// SendEdgeBatch sends the given edge batch synchronously.
//
// By default (see [WithIgnoreServerAcks]) it will wait for the server to acknowledge sent batches.
// While it waits, the method will block.
// In case of a server error, it might even wait forever.
// Therefore you likely want to use [Session.SendEdgeBatchWithContext] to set a timeout.
func (s *Session) SendEdgeBatch(edgeBatch *EdgeBatch) (uint32, error) {
	return s.SendEdgeBatchWithContext(context.Background(), edgeBatch)
}

// SendEdgeBatchWithContext is like [Session.SendEdgeBatch] but takes an additional context (e.g. to be able to cancel it when executing as a Go routine).
func (s *Session) SendEdgeBatchWithContext(ctx context.Context, edgeBatch *EdgeBatch) (uint32, error) {
	var request bmsapi.StoreDatedEdgeBatchRequest
	var batch []*bmsapi.DatedEdge

	batchID := atomic.AddUint32(&s.edgeBatchCounter, 1)
	request.BatchId = batchID

	var err error
	for _, given := range *edgeBatch {
		var own bmsapi.DatedEdge

		own.Timestamp = bmsencoding.TimeToWrapper(given.Timestamp)
		own.SrcBotId = bmsencoding.StringToStringPtr(given.SrcBotID)
		own.SrcIp = bmsencoding.IPToWrapper(given.SrcIP)
		own.SrcPort = bmsencoding.PortToWrapper(given.SrcPort)
		own.DstBotId = bmsencoding.StringToStringPtr(given.DstBotID)
		own.DstIp = bmsencoding.IPToWrapper(given.DstIP)
		own.DstPort = bmsencoding.PortToWrapper(given.DstPort)

		batch = append(batch, &own)

		// Context was canceled or deadline is exceeded
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
	}

	request.Edges = batch

	err = s.edgeStreamSendMutex.Acquire(ctx, 1)
	if err != nil {
		// Context was canceled or exceeded deadline
		return 0, err
	}
	err = s.edgeStream.Send(&request)
	if err != nil {
		return 0, err
	}
	s.edgeStreamSendMutex.Release(1)

	if !s.ignoreServerAcks {
		var deleted bool
		for {
			deleted = s.receivedEdgeBatchIDs.CompareAndDelete(batchID, true)
			if deleted {
				break
			}
			if ctx.Err() != nil {
				return 0, err
			}
			time.Sleep(500 * time.Microsecond)
		}
	}

	return batchID, nil
}

// AddBotReply adds a bot reply to the session's internal storage that can be flushed with [Session.Flush] or [Session.FlushBotReplies].
//
// An empty string passed as bot ID will be interpreted as a non-existing ID.
// An empty struct or nil passed as additional data will be interpreted as no additional data.
//
// Please also note that the additional data go through [encoding/json.Marshal], so fields that should go into the database should be exported.
// This also means you can use field tags like `json:"some_field_name"`.
func (s *Session) AddBotReply(timestamp time.Time, ip net.IP, port uint16, botID string, otherData interface{}) {
	s.unsentBotReplyBatchMutex.Lock()
	s.unsentBotReplyBatch = append(s.unsentBotReplyBatch, &BotReply{
		Timestamp: timestamp,
		BotID:     botID,
		IP:        ip,
		Port:      port,
		OtherData: otherData,
	})
	s.unsentBotReplyBatchMutex.Unlock()
}

// AddBotReplyStruct is like [Session.AddBotReply] but takes the bot reply as a struct.
func (s *Session) AddBotReplyStruct(botReply *BotReply) {
	s.unsentBotReplyBatchMutex.Lock()
	s.unsentBotReplyBatch = append(s.unsentBotReplyBatch, botReply)
	s.unsentBotReplyBatchMutex.Unlock()
}

// AddFailedTry adds a failed try to the session's internal storage that can be flushed with [Session.Flush] or [Session.FlushFailedTries].
//
// An empty string passed as bot ID / reason will be interpreted as a non-existing ID / reason.
// An empty struct or nil passed as additional data will be interpreted as no additional data.
//
// Please also note that the additional data go through [encoding/json.Marshal], so fields that should go into the database should be exported.
// This also means you can use field tags like `json:"some_field_name"`.
func (s *Session) AddFailedTry(timestamp time.Time, ip net.IP, port uint16, botID string, reason string, otherData interface{}) {
	s.unsentFailedTryBatchMutex.Lock()
	s.unsentFailedTryBatch = append(s.unsentFailedTryBatch, &FailedTry{
		Timestamp: timestamp,
		BotID:     botID,
		IP:        ip,
		Port:      port,
		Reason:    reason,
		OtherData: otherData,
	})
	s.unsentFailedTryBatchMutex.Unlock()
}

// AddFailedTryStruct is like [Session.AddFailedTry] but takes the failed try as a struct.
func (s *Session) AddFailedTryStruct(failedTry *FailedTry) {
	s.unsentFailedTryBatchMutex.Lock()
	s.unsentFailedTryBatch = append(s.unsentFailedTryBatch, failedTry)
	s.unsentFailedTryBatchMutex.Unlock()
}

// AddEdge adds an edge to the session's internal storage that can be flushed with [Session.Flush] or [Session.FlushEdges].
//
// An empty string passed as bot ID will be interpreted as a non-existing ID.
func (s *Session) AddEdge(timestamp time.Time, srcIP net.IP, srcPort uint16, srcBotID string, dstIP net.IP, dstPort uint16, dstBotID string) {
	s.unsentEdgeBatchMutex.Lock()
	s.unsentEdgeBatch = append(s.unsentEdgeBatch, &Edge{
		Timestamp: timestamp,
		SrcBotID:  srcBotID,
		SrcIP:     srcIP,
		SrcPort:   srcPort,
		DstBotID:  dstBotID,
		DstIP:     dstIP,
		DstPort:   dstPort,
	})
	s.unsentEdgeBatchMutex.Unlock()
}

// AddEdgeStruct is like [Session.AddEdge] but takes the edge as a struct.
func (s *Session) AddEdgeStruct(edge *Edge) {
	s.unsentEdgeBatchMutex.Lock()
	s.unsentEdgeBatch = append(s.unsentEdgeBatch, edge)
	s.unsentEdgeBatchMutex.Unlock()
}

// FlushBotReplies is like [Session.Flush] but only flushes bot replies.
func (s *Session) FlushBotReplies() error {
	return s.FlushBotRepliesWithContext(context.Background())
}

// FlushEdges is like [Session.Flush] but only flushes edges.
func (s *Session) FlushEdges() error {
	return s.FlushEdgesWithContext(context.Background())
}

// FlushFailedTries is like [Session.Flush] but only flushes failed tries.
func (s *Session) FlushFailedTries() error {
	return s.FlushFailedTriesWithContext(context.Background())
}

// Flush flushes the bot replies, edges and failed tries added to the internal storage via the Add* methods.
//
// It uses the also exported Send* methods for this, so it will block while sending.
// If you don't want it to wait for the server ack, you can use [WithIgnoreServerAcks].
// If you want to cancel it while it waits for the ack (which might take long), you can use [Session.FlushWithContext].
//
// Flush first flushes bot replies, then edges, then failed tries.
// It will stop on the first error it encounters, which might result in unflushed failed tries, although it maybe would have been possible to flush them.
// As sending errors very likely are due to the underlying connections having problems, it's also likely that sending anything further would have failed either way.
func (s *Session) Flush() error {
	return s.FlushWithContext(context.Background())
}

// FlushWithContext is like [Session.Flush] but takes a context which for example can be used to cancel it after some timeout.
func (s *Session) FlushWithContext(ctx context.Context) error {
	var err error

	err = s.FlushBotRepliesWithContext(ctx)
	if err != nil {
		return err
	}

	err = s.FlushEdgesWithContext(ctx)
	if err != nil {
		return err
	}

	err = s.FlushFailedTriesWithContext(ctx)
	if err != nil {
		return err
	}

	return nil
}

// FlushBotRepliesWithContext is like [Session.FlushBotReplies] but takes a context which for example can be used to cancel it after some timeout.
func (s *Session) FlushBotRepliesWithContext(ctx context.Context) error {
	return s.flushBotRepliesWithContext(ctx, false)
}

func (s *Session) flushBotRepliesWithContext(ctx context.Context, keepLocked bool) error {
	ctxWithSessionCredentials := metadata.AppendToOutgoingContext(ctx, "session-token-bin", string(s.sessionToken[:]))

	var botReplyBatchToSend BotReplyBatch
	var err error

	s.unsentBotReplyBatchMutex.Lock()
	botReplyBatchToSend = append(botReplyBatchToSend, s.unsentBotReplyBatch...)
	s.unsentBotReplyBatch = nil
	if !keepLocked {
		s.unsentBotReplyBatchMutex.Unlock()
	}

	// Ignoring batch ID here, returning it would expose the internally used Send method
	// If users want to self check the acknowledged batch IDs, they should use the Send methods directly
	_, err = s.SendBotReplyBatchWithContext(ctxWithSessionCredentials, &botReplyBatchToSend)
	if err != nil {
		// If keepLock, we're in the final sending, if there's an error, we can't do anything to send the batch later
		// Also makes sure that the lock will be available a few lines down from here
		if keepLocked {
			return err
		}

		// Otherwise re-add the unsuccessfully sent batch before returning the error
		s.unsentBotReplyBatchMutex.Lock()
		s.unsentBotReplyBatch = append(s.unsentBotReplyBatch, botReplyBatchToSend...)
		s.unsentBotReplyBatchMutex.Unlock()

		return err
	}

	return nil
}

// FlushEdgesWithContext is like [Session.FlushEdges] but takes a context which for example can be used to cancel it after some timeout.
func (s *Session) FlushEdgesWithContext(ctx context.Context) error {
	return s.flushEdgesWithContext(ctx, false)
}

func (s *Session) flushEdgesWithContext(ctx context.Context, keepLocked bool) error {
	ctxWithSessionCredentials := metadata.AppendToOutgoingContext(ctx, "session-token-bin", string(s.sessionToken[:]))

	var edgeBatchToSend EdgeBatch
	var err error

	s.unsentEdgeBatchMutex.Lock()
	edgeBatchToSend = append(edgeBatchToSend, s.unsentEdgeBatch...)
	s.unsentEdgeBatch = nil
	if !keepLocked {
		s.unsentEdgeBatchMutex.Unlock()
	}

	// Ignoring batch ID here, returning it would expose the internally used Send method
	// If users want to self check the acknowledged batch IDs, they should use the Send methods directly
	_, err = s.SendEdgeBatchWithContext(ctxWithSessionCredentials, &edgeBatchToSend)
	if err != nil {
		// If keepLock, we're in the final sending, if there's an error, we can't do anything to send the batch later
		// Also makes sure that the lock will be available a few lines down from here
		if keepLocked {
			return err
		}

		// Otherwise re-add the unsuccessfully sent batch before returning the error
		s.unsentEdgeBatchMutex.Lock()
		s.unsentEdgeBatch = append(s.unsentEdgeBatch, edgeBatchToSend...)
		s.unsentEdgeBatchMutex.Unlock()

		return err
	}

	return nil
}

// FlushFailedTriesWithContext is like [Session.FlushFailedTries] but takes a context which for example can be used to cancel it after some timeout.
func (s *Session) FlushFailedTriesWithContext(ctx context.Context) error {
	return s.flushFailedTriesWithContext(ctx, false)
}

func (s *Session) flushFailedTriesWithContext(ctx context.Context, keepLocked bool) error {
	ctxWithSessionCredentials := metadata.AppendToOutgoingContext(ctx, "session-token-bin", string(s.sessionToken[:]))

	var failedTryBatchToSend FailedTryBatch
	var err error

	s.unsentFailedTryBatchMutex.Lock()
	failedTryBatchToSend = append(failedTryBatchToSend, s.unsentFailedTryBatch...)
	s.unsentFailedTryBatch = nil
	if !keepLocked {
		s.unsentFailedTryBatchMutex.Unlock()
	}

	// Ignoring batch ID here, returning it would expose the internally used Send method
	// If users want to self check the acknowledged batch IDs, they should use the Send methods directly
	_, err = s.SendFailedTryBatchWithContext(ctxWithSessionCredentials, &failedTryBatchToSend)
	if err != nil {
		// If keepLock, we're in the final sending, if there's an error, we can't do anything to send the batch later
		// Also makes sure that the lock will be available a few lines down from here
		if keepLocked {
			return err
		}

		// Otherwise re-add the unsuccessfully sent batch before returning the error
		s.unsentFailedTryBatchMutex.Lock()
		s.unsentFailedTryBatch = append(s.unsentFailedTryBatch, failedTryBatchToSend...)
		s.unsentFailedTryBatchMutex.Unlock()

		return err
	}

	return nil
}

func convertToBMSAPIEnum(reason DisconnectReason) bmsapi.DisconnectReason {
	// I currently don't know a better way (except maybe proper reflection)
	// to convert the bmsapi enum to our enum (which should be exactly the same).

	// Note: This method panics if it encounters an enum entry it doesn't know.

	var bmsReason bmsapi.DisconnectReason

	switch reason {
	case DisconnectReasonUnspecified:
		bmsReason = bmsapi.DisconnectReason_DISCONNECT_REASON_UNSPECIFIED
	case DisconnectReasonFinished:
		bmsReason = bmsapi.DisconnectReason_DISCONNECT_REASON_FINISHED
	case DisconnectReasonBeRightBack:
		bmsReason = bmsapi.DisconnectReason_DISCONNECT_REASON_BE_RIGHT_BACK
	case DisconnectReasonBeRightBackWithNewConfig:
		bmsReason = bmsapi.DisconnectReason_DISCONNECT_REASON_BE_RIGHT_BACK_WITH_NEW_CONFIG
	case DisconnectReasonClientError:
		bmsReason = bmsapi.DisconnectReason_DISCONNECT_REASON_CLIENT_ERROR
	case DisconnectReasonOther:
		bmsReason = bmsapi.DisconnectReason_DISCONNECT_REASON_OTHER
	default:
		panic("Got an unknown reason, this shouldn't have happened")
	}

	return bmsReason
}
