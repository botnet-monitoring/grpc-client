// Package hooks contains the definitions of hooks that can be passed to NewSession via WithBotReplyBatchResponseSuccessHook (and similar).
//
// They are nicely tucked away in their own package as they are not needed much.
package hooks

import bmsapi "github.com/botnet-monitoring/grpc-api/gen"

// Functions which implement BotReplyBatchResponseSuccessHook and are passed to NewSession via WithBotReplyBatchResponseSuccessHook are executed when a bot reply batch was successfully sent.
type BotReplyBatchResponseSuccessHook func(*bmsapi.StoreDatedBotReplyBatchResponse)

// Functions which implement FailedTryBatchResponseSuccessHook and are passed to NewSession via WithFailedTryBatchResponseSuccessHook are executed when a failed try batch was successfully sent.
type FailedTryBatchResponseSuccessHook func(*bmsapi.StoreDatedFailedTryBatchResponse)

// Functions which implement EdgeBatchResponseSuccessHook and are passed to NewSession via WithEdgeBatchResponseSuccessHook are executed when an edge batch was successfully sent.
type EdgeBatchResponseSuccessHook func(*bmsapi.StoreDatedEdgeBatchResponse)

// Functions which implement BotReplyBatchResponseFailureHook and are passed to NewSession via WithBotReplyBatchResponseFailureHook are executed when the server could not successfully receive a bot reply batch.
//
// Note that the stream ends when the server returns an error.
// Therefore this hook will only be executed once.
// Also note that a stream cancellation is not considered an error, as this is is [typically requested by the client].
//
// [typically requested by the client]: https://pkg.go.dev/google.golang.org/grpc/codes#Canceled
type BotReplyBatchResponseFailureHook func(error)

// Functions which implement FailedTryBatchResponseFailureHook and are passed to NewSession via WithFailedTryBatchResponseFailureHook are executed when the server could not successfully receive a failed try batch.
//
// Note that the stream ends when the server returns an error.
// Therefore this hook will only be executed once.
// Also note that a stream cancellation is not considered an error, as this is is [typically requested by the client].
//
// [typically requested by the client]: https://pkg.go.dev/google.golang.org/grpc/codes#Canceled
type FailedTryBatchResponseFailureHook func(error)

// Functions which implement EdgeBatchResponseFailureHook and are passed to NewSession via WithEdgeBatchResponseFailureHook are executed when the server could not successfully receive an edge batch.
//
// Note that the stream ends when the server returns an error.
// Therefore this hook will only be executed once.
// Also note that a stream cancellation is not considered an error, as this is is [typically requested by the client].
//
// [typically requested by the client]: https://pkg.go.dev/google.golang.org/grpc/codes#Canceled
type EdgeBatchResponseFailureHook func(error)
