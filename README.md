# plugin-websocket

WebSocket plugin for the [Dreego](https://github.com/dreego-stack/dreego)
framework. Adds real-time bidirectional communication using only the Go
standard library — no external dependencies.

## Quick Start

```go
package main

import (
    "log"
    "github.com/dreego-stack/dreego/core"
    ws "github.com/dreego-stack/plugin-websocket"
)

func main() {
    app := dreego.New()
    if err := ws.Register(app, ws.Options{Path: "/ws"}); err != nil {
        log.Fatal(err)
    }
    ws.HubInstance().Broadcast([]byte("hello"))
    app.Listen(":8080")
}
```

Frontend:

```javascript
const ws = new WebSocket("ws://localhost:8080/ws");
ws.onmessage = (e) => console.log("got:", e.data);
```

## Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Path` | `string` | `"/ws"` | URL path for the WebSocket endpoint |

## Hub

`websocket.HubInstance()` returns a singleton hub. Call `Broadcast([]byte)`
to send messages to all connected clients. The hub is thread-safe.

## Implementation

Uses only the Go standard library. The WebSocket protocol (RFC 6455) is
implemented in `ws.go` with `net/http`, `crypto/sha1`, `encoding/base64`, and
`encoding/binary`. No gorilla/websocket or other external packages.

## Getting Started (Development)

```sh
make init    # download and vendor dependencies
make test    # run tests
make run     # run the demo app
```

## License

MPL-2.0, same as Dreego.