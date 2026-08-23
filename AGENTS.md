# Agent Instructions for plugin-websocket

- Don't create binaries here — only in /tmp or ./tmp

## Language Rule

- **Chat with user:** German
- **Everything in this repository:** English (code, comments, docs, commits, tests)

## What This Is

This is the **WebSocket plugin** for the Dreego framework. It adds WebSocket
support using only the Go standard library — no external dependencies
(gorilla/websocket, nhooyr.io/websocket, etc.).

## How It Works

1. `websocket.Register(app, websocket.Options{Path: "/ws"})` registers the WebSocket endpoint
2. Clients connect via `new WebSocket("ws://host/ws")` in the browser
3. `websocket.HubInstance().Broadcast([]byte("data"))` sends messages to all clients
4. The handler echoes received messages back to the sender
5. Client disconnects are cleaned up automatically

## Implementation

The WebSocket protocol (RFC 6455) is implemented in `ws.go` using only:
- `net/http` for the HTTP upgrade handshake
- `crypto/sha1` + `encoding/base64` for the Sec-WebSocket-Accept calculation
- `encoding/binary` for frame encoding/decoding
- `bufio` for buffered reading

No external WebSocket library is used.

## Plugin Contract

- Exports `Register(app *dreego.App, options Options) error`
- Typed Options (Path)
- No central Plugin interface
- Must be called before `app.Build()` / `app.Listen()`
- Core never imports a plugin; the plugin imports `github.com/dreego-stack/dreego/core`

## Testing

- `go test ./...` — unit tests with real WebSocket connections via httptest
- `go test -race ./...` — race detection

## CI

- `.github/workflows/ci.yml` — `go vet`, `go test -race`, and a compatibility job
- `.github/workflows/release.yml` — validates change file, tests, creates tag
- `.github/dependabot.yml` — auto-updates dreego dependency weekly

## Coding Rules

- Max 300 lines per handwritten file
- No code comments (except where needed for clarity)
- Go 1.22+, standard library only (no external deps)
- One Go package per repository

## Commit Convention

Every change lands via a pull request with one `.changes/*.md` file.