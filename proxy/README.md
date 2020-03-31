```text
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2019 Intel Corporation
```

# Prog - Proxy+Gateway HTTP/2 Intermediary

A dynamic layer 7 proxy and gateway for HTTP/2 connections

The current implementation - ironically - does _not_ actually include a proxy
or a gateway. A future version in development does. The current implementation
only includes a set of utils that will be included and maintained in the full
version.

Current implementation pre-opens a tcp connection from the side behind the NAT
to the server on the internet. This connection is then used to make API calls
in the reverse direction (from internet to the NAT).

## Usage

To traverse a NAT boundary and have both sides operate as unary RPC servers,
start with a basic implementation assuming routability and then update each
side:

### Routable Side

This side will be dialable, but won't be able to dail out to the other side
itself. Wrap the normal listener in a `progutil.PrefaceListener`, which will
separate incoming connections into client and server conns and make them
available via its `Accept` and `Dial` methods.

```go
// Listen on an ephemeral port on all interfaces
lis, _ := net.Listen("tcp", ":0")
prefaceLis := &progutil.PrefaceListener{Listener: lis}
defer prefaceLis.Close()

// Start HTTP server with PrefaceListener
go http.Serve(prefaceLis, nil)

// Dial back to NAT'd side with 2s timeout
conn, _ := prefaceLis.Dial("", 2 * time.Second)
defer conn.Close()
```

Note: `Accept` must be called in an infinite loop for `Dial` to work. This is
what is done by an HTTP or gRPC server under the covers, but if you're only
going to use the `PrefaceListener` for its dialing abilities, be sure to at
least call `go func() { for { prefaceListener.Accept() } }()`.

### NAT'd Side

This side will not be dialable, so it is tasked with initiating both the TCP
streams that will be used for client and server connections. To do this, the
`progutil.DialListener` can be used. Essentially this is a `net.Listener` that
`Accept`s new conns by dialing out.

```go
// Listen by connecting to the remote server
lis := &progutil.DialListener{RemoteAddr: prefaceLis.Addr()}
defer lis.Close()

// Serve HTTP as with any normal listener (except only HTTP/2 should be used)
go http.Serve(lis, nil)

// Dial the remote server normally for client conns
conn, _ := (&net.Dialer{}).DialContext(context.TODO(), 
	prefaceLis.Addr().Network(), prefaceLis.Addr().String())
defer conn.Close()

// Note: PrefaceListener on the remote server assumes only HTTP/2 conns
conn.Write(http2.ClientPreface)
```

## Testing

```
GODEBUG=http2debug=2 go test -v -race -count=1
```

