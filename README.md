# BMS gRPC Client [![GoDoc][doc-img]][doc] [![Build Status][ci-img]][ci]

[doc-img]: https://pkg.go.dev/badge/github.com/botnet-monitoring/grpc-client
[doc]: https://pkg.go.dev/github.com/botnet-monitoring/grpc-client
[ci-img]: https://github.com/botnet-monitoring/grpc-client/actions/workflows/go.yml/badge.svg
[ci]: https://github.com/botnet-monitoring/grpc-client/actions/workflows/go.yml


_This repository is part of the [Botnet Monitoring System (BMS)](https://github.com/botnet-monitoring), you can read more about BMS [here](https://github.com/botnet-monitoring)._

This repository contains an ergonomic client implementation of the [BMS gRPC API](https://github.com/botnet-monitoring/grpc-api) for Go.
Crawler and sensor implementations can use this Go package to send their measurement data to a [BMS server]().

The client is centered around the concept of sessions.
To send data to BMS, a client must request a session from the server.
The client starts a session by providing the server with its monitor ID, an auth token and the botnet ID it is configured to monitor (and optionally more configuration).
The server checks if the requested session is valid (e.g. if the botnet exists in the database) and returns a session token.

While a session is in progress, all settings are fixed.
If the client wants to change its configuration (e.g. switching the botnet it monitors), it has to end the current session and start a new one.

### Remarks
- The client currently uses an **unencrypted** gRPC connection. For now, you can use a VPN if concerned.
- The client currently **does not validate** any messages it gets from the server (which is alright, since we trust the server to adhere to the specification).
- The client **should be fully thread-safe**, but we haven't tested it thoroughly.


## Installation
```shell
$ go get github.com/botnet-monitoring/grpc-client
```

## Quick Start
<!--
  The following example is the same as ./example_simple_test.go contains.
  Please make your changes there, check if it compiles/works and then copy it back here.
-->
``` go
// Creates a simple client (which is a client that automatically starts a session)
client, err := bmsclient.NewSimpleClient(
    "your-bms-server.local:8083",
    "some-monitor-id",
    "AAAA/some+example+token/AAAAAAAAAAAAAAAAAAA=",
    "some-botnet-id",
)
if err != nil {
    panic(err)
}

// You can send data synchronously in batches
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
err = client.SendBotReplyBatch(&batch) // this blocks
if err != nil {
    panic(err)
}

// Or add it to the client as you gather it (concurrency-safe) and flush it at once
client.AddBotReply(time.Now(), net.ParseIP("192.0.2.3"), 10003, "", nil)
client.AddBotReply(time.Now(), net.ParseIP("192.0.2.4"), 10004, "", nil)
err = client.Flush() // this blocks
if err != nil {
    panic(err)
}

// If you're done, it'd be nice if you end the session
err = client.End()
if err != nil {
    panic(err)
}
```

You can find a more complex example in [`example_advanced_test.go`](./example_advanced_test.go).
