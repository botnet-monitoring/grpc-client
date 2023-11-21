package bmsclient

import (
	"context"
	"net"
	"time"
)

// A BotReply struct represents the data collected when observing a bot.
type BotReply struct {
	// The timestamp the bot was observed
	Timestamp time.Time

	// An ID which the bot can be identified with (optional, as not available for all botnets)
	// Set an empty string when not available
	BotID string

	// The IP address of the bot
	IP net.IP

	// The port the bot responded on
	Port uint16

	// Potential other data that was observed
	// Set an empty struct or nil when not available
	//
	// Please also note that this goes through [encoding/json.Marshal], so fields that should go into the database should be exported
	// This also means you can use field tags like `json:"some_field_name"`
	OtherData interface{}
}

// BotReplyBatch can contain multiple/many instances of [BotReply].
//
// Since it's a slice of pointers, it should be alright to make this quite big (a few hundred still works fine).
type BotReplyBatch []*BotReply

// A FailedTry struct represents the data collected when not observing a bot (but it should have worked).
// E.g. a crawler tried to reach a bot, but the bot did not respond before the timeout was reached.
//
// Note that not all monitors can measure this.
// For example a crawler that uses a stateless loop for requesting peers and a stateless loop for receiving responses (e.g. by listening on a fixed port) cannot know with certainty whether a response relates to the last request or whether it came late and was meant for an earlier request.
type FailedTry struct {
	// The timestamp the bot observation failed
	Timestamp time.Time

	// An ID which the bot can be identified with (optional, as not available for all botnets)
	// Set an empty string when not available
	BotID string

	// The IP address that was tried to reach
	IP net.IP

	// The port that was tried to reach
	Port uint16

	// The reason the observation failed (e.g. "timeout", "connection refused")
	Reason string

	// Potential other data that was observed
	// Set an empty struct or nil when not available
	//
	// Please also note that this goes through [encoding/json.Marshal], so fields that should go into the database should be exported
	// This also means you can use field tags like `json:"some_field_name"`
	OtherData interface{}
}

// FailedTryBatch can contain multiple/many instances of [FailedTry].
//
// Since it's a slice of pointers, it should be alright to make this quite big (a few hundred still works fine).
type FailedTryBatch []*FailedTry

// An Edge struct represents a connection from one bot to another.
// It is mostly relevant for P2P botnets where bots maintain peer lists of other bots.
//
// Most of the time observing a bot reply also coincides with observing one (or multiple) edges.
// For example when requesting peers from a bot in a P2P botnet, the bot responds with multiple other bots.
// These bots will be the destination bots of the edge, while the bots which responded will be the source bot.
// Please also note that the returned bots are not necessarily online and actually don't even need to be meaningful.
type Edge struct {
	// The timestamp the edge was observed
	Timestamp time.Time

	// An ID which the source bot can be identified with (optional, as not available for all botnets)
	// Set an empty string when not available
	SrcBotID string

	// The IP address of the source bot
	SrcIP net.IP

	// The port the source bot responded on
	SrcPort uint16

	// An ID which the destination bot can be identified with (optional, as not available for all botnets)
	// Set an empty string when not available
	DstBotID string

	// The IP address of the destination bot
	DstIP net.IP

	// The port the destination bot is expected to be reached
	DstPort uint16
}

// EdgeBatch can contain multiple/many instances of [Edge].
//
// Since it's a slice of pointers, it should be alright to make this quite big (a few hundred still works fine).
type EdgeBatch []*Edge

// SessionInfo is an interface that is implemented by [Session] and [SimpleClient].
//
// It makes it easier for you to switch from [SimpleClient] to [Client] and [Client.NewSession].
type SessionInfo interface {
	GetSessionToken() [4]byte
	GetSessionTimeout() time.Duration
	GetLastBotReplyTime() time.Time
	GetLastFailedTryTime() time.Time
	GetLastEdgeTime() time.Time
}

// Make sure Session implements SessionInfo
var _ SessionInfo = (*Session)(nil)

// Make sure SimpleClient implements SessionInfo
var _ SessionInfo = (*SimpleClient)(nil)

// GRPCDataIngestion is an interface that [Session] implements.
// It contains a collection of methods to send data to BMS.
//
// This interface can also be implemented by external packages, so that crawlers can use the same method calls.
// E.g. useful for implementing local storage in a crawler to aid when the BMS server goes down.
type GRPCDataIngestion interface {
	SendBotReplyBatch(botReplyBatch *BotReplyBatch) (uint32, error)
	SendBotReplyBatchWithContext(ctx context.Context, botReplyBatch *BotReplyBatch) (uint32, error)

	SendFailedTryBatch(failedTryBatch *FailedTryBatch) (uint32, error)
	SendFailedTryBatchWithContext(ctx context.Context, failedTryBatch *FailedTryBatch) (uint32, error)

	SendEdgeBatch(edgeBatch *EdgeBatch) (uint32, error)
	SendEdgeBatchWithContext(ctx context.Context, edgeBatch *EdgeBatch) (uint32, error)

	AddBotReply(timestamp time.Time, ip net.IP, port uint16, botID string, otherData interface{})
	AddFailedTry(timestamp time.Time, ip net.IP, port uint16, botID string, reason string, otherData interface{})
	AddEdge(timestamp time.Time, srcIP net.IP, srcPort uint16, srcBotID string, dstIP net.IP, dstPort uint16, dstBotID string)
	Flush() error
	FlushBotReplies() error
	FlushFailedTries() error
	FlushEdges() error
	FlushWithContext(ctx context.Context) error
	FlushBotRepliesWithContext(ctx context.Context) error
	FlushFailedTriesWithContext(ctx context.Context) error
	FlushEdgesWithContext(ctx context.Context) error
}

// SimpleGRPCDataIngestion is an interface that the [SimpleClient] implements.
// It contains a collection of basic methods to send data to BMS.
//
// SimpleGRPCDataIngestion is a (guaranteed) subset of [GRPCDataIngestion] which enables you to easily switch from [SimpleClient] to [Client] and [Client.NewSession].
type SimpleGRPCDataIngestion interface {
	SendBotReplyBatch(botReplyBatch *BotReplyBatch) (uint32, error)
	SendFailedTryBatch(failedTryBatch *FailedTryBatch) (uint32, error)
	SendEdgeBatch(edgeBatch *EdgeBatch) (uint32, error)

	AddBotReply(timestamp time.Time, ip net.IP, port uint16, botID string, otherData interface{})
	AddFailedTry(timestamp time.Time, ip net.IP, port uint16, botID string, reason string, otherData interface{})
	AddEdge(timestamp time.Time, srcIP net.IP, srcPort uint16, srcBotID string, dstIP net.IP, dstPort uint16, dstBotID string)
	Flush() error
	FlushBotReplies() error
	FlushFailedTries() error
	FlushEdges() error
}

// Make sure Session implements GRPCDataIngestion
var _ GRPCDataIngestion = (*Session)(nil)

// Make sure SimpleGRPCDataIngestion is a subset of GRPCDataIngestion
var _ SimpleGRPCDataIngestion = (GRPCDataIngestion)(nil)

// Make sure SimpleClient implements SimpleGRPCDataIngestion
var _ SimpleGRPCDataIngestion = (*SimpleClient)(nil)
