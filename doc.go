// Package bmsclient implements an easy-to-use client for the [BMS gRPC API].
// With it, you can send measurement data of botnets to a [BMS server].
// See for example the [BMS basecrawler] which implements a stateful P2P botnet crawler using this package.
//
// If you don't know what BMS is, you can read more [here].
//
// # Concepts
//
// The client is centered around the concept of sessions.
// To send data to BMS, a client must request a session from the server.
// The client starts a session by providing the server with its monitor ID, an auth token and the botnet ID it is configured to monitor (and optionally more configuration).
// The server checks if the requested session is valid (e.g. if the botnet exists in the database) and returns a session token.
//
// While a session is in progress, all settings are fixed.
// If the client wants to change its configuration (e.g. switching the botnet it monitors), it has to end the current session and start a new one.
//
// For more on sessions (e.g. why they exist) and other concepts around BMS (e.g. what campaigns are, why they exist and how to use them), you can consult the explanations [here].
//
// # Modes of Usage
//
// This package provides two modes of usage: [SimpleClient] is a client that internally opens a session and implements basic methods for sending data to BMS.
// For most use cases this should be alright and it's definitely easier to use.
//
// However, if you have a more advanced use case, you can create also a [Client] instance and open multiple [Session] instances on it.
// In addition, the [Session] struct implements more advanced methods to send data (e.g. sending with [context.Context]).
//
// # Add vs. Send
//
// This client offers two different ways to send data to BMS.
// The methods prefixed with Add take a single bot reply, edge or failed try.
// You can add the measurement data as it occurs (also concurrently) and flush it after a fixed time or a fixed number of added measurements.
//
// The methods prefixed with Send take a batch that will be sent synchronously.
// After the batch is sent, the client will by default wait for the server to acknowledge it.
// This means that if a send method returned without error, it is guaranteed that the server received the batch successfully.
// The flush methods internally use the send methods.
//
// [BMS gRPC API]: https://github.com/botnet-monitoring/grpc-api
// [BMS server]: https://github.com/botnet-monitoring/server
// [BMS basecrawler]: https://github.com/botnet-monitoring/basecrawler
// [here]: https://github.com/botnet-monitoring
package bmsclient
