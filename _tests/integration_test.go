package tests

import (
	"io"
	"net"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	dreego "github.com/dreego-stack/dreego/core"
	ws "github.com/dreego-stack/plugin-websocket"
)

func TestWebSocketPluginIntegration(t *testing.T) {
	app := dreego.New()
	if err := ws.Register(app, ws.Options{Path: "/ws"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	conn, err := websocketDial(server.URL + "/ws")
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer conn.Close()

	msg := "hello integration"
	if err := wsWriteText(conn, msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := wsReadText(conn)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != msg {
		t.Errorf("echo: got %q, want %q", got, msg)
	}
}

func TestWebSocketPluginBroadcast(t *testing.T) {
	app := dreego.New()
	if err := ws.Register(app, ws.Options{Path: "/ws"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	server := httptest.NewServer(app.Handler())
	defer server.Close()

	conn, err := websocketDial(server.URL + "/ws")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)

	ws.HubInstance().Broadcast([]byte("broadcast-msg"))

	got, err := wsReadText(conn)
	if err != nil {
		t.Fatalf("read broadcast: %v", err)
	}
	if got != "broadcast-msg" {
		t.Errorf("broadcast: got %q, want %q", got, "broadcast-msg")
	}
}

func TestWebSocketPluginRejectNonUpgrade(t *testing.T) {
	app := dreego.New()
	if err := ws.Register(app, ws.Options{Path: "/ws"}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	server := httptest.NewServer(app.Handler())
	defer server.Close()

	conn, err := net.Dial("tcp", server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	io.WriteString(conn, "GET /ws HTTP/1.1\r\nHost: localhost\r\n\r\n")
	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	if !strings.Contains(string(buf[:n]), "400 Bad Request") {
		t.Errorf("expected 400 for non-upgrade request, got: %s", string(buf[:n]))
	}
}

func TestWebSocketPluginErrAppBuilt(t *testing.T) {
	app := dreego.New()
	_ = app.Handler()
	err := ws.Register(app, ws.Options{})
	if err == nil {
		t.Fatal("expected ErrAppBuilt after build")
	}
}

func websocketDial(rawURL string) (net.Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		return nil, err
	}
	req := "GET " + u.Path + " HTTP/1.1\r\n" +
		"Host: " + u.Host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	io.WriteString(conn, req)
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		return nil, err
	}
	resp := string(buf[:n])
	if !strings.Contains(resp, "101 Switching Protocols") {
		conn.Close()
		return nil, io.EOF
	}
	return conn, nil
}

func wsWriteText(conn net.Conn, msg string) error {
	mask := [4]byte{0x12, 0x34, 0x56, 0x78}
	header := []byte{0x81, 0x80 | byte(len(msg))}
	if _, err := conn.Write(header); err != nil {
		return err
	}
	if _, err := conn.Write(mask[:]); err != nil {
		return err
	}
	masked := make([]byte, len(msg))
	for i := range msg {
		masked[i] = msg[i] ^ mask[i%4]
	}
	_, err := conn.Write(masked)
	return err
}

func wsReadText(conn net.Conn) (string, error) {
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return "", err
	}
	var hdr [2]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return "", err
	}
	length := int(hdr[1] & 0x7F)
	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return "", err
	}
	return string(payload), nil
}