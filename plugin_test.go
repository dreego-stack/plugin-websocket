package websocket

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dreego "github.com/dreego-stack/dreego/core"
)

type wsClient struct {
	conn net.Conn
	br   *bufio.Reader
}

func dialWS(t *testing.T, srv *httptest.Server, path string) *wsClient {
	t.Helper()
	addr := srv.Listener.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial %s: %v", addr, err)
	}
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand: %v", err)
	}
	encodedKey := base64.StdEncoding.EncodeToString(key)
	req := fmt.Sprintf(
		"GET %s HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Key: %s\r\n"+
			"Sec-WebSocket-Version: 13\r\n\r\n",
		path, addr, encodedKey)
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("write upgrade: %v", err)
	}
	br := bufio.NewReader(conn)
	statusLine, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !strings.Contains(statusLine, "101") {
		conn.Close()
		t.Fatalf("upgrade status = %q, want 101", statusLine)
	}
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			t.Fatalf("read headers: %v", err)
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}
	return &wsClient{conn: conn, br: br}
}

func (c *wsClient) writeFrame(opcode byte, data []byte) error {
	var header []byte
	maskKey := make([]byte, 4)
	if _, err := rand.Read(maskKey); err != nil {
		return err
	}
	masked := make([]byte, len(data))
	for i := range data {
		masked[i] = data[i] ^ maskKey[i%4]
	}
	if len(data) < 126 {
		header = []byte{0x80 | opcode, 0x80 | byte(len(data))}
	} else if len(data) < 65536 {
		header = make([]byte, 4)
		header[0] = 0x80 | opcode
		header[1] = 0x80 | 126
		binary.BigEndian.PutUint16(header[2:], uint16(len(data)))
	} else {
		header = make([]byte, 10)
		header[0] = 0x80 | opcode
		header[1] = 0x80 | 127
		binary.BigEndian.PutUint64(header[2:], uint64(len(data)))
	}
	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	if _, err := c.conn.Write(maskKey); err != nil {
		return err
	}
	if len(masked) > 0 {
		if _, err := c.conn.Write(masked); err != nil {
			return err
		}
	}
	return nil
}

func (c *wsClient) readFrame() (byte, []byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(c.br, hdr[:]); err != nil {
		return 0, nil, err
	}
	opcode := hdr[0] & 0x0F
	length := int64(hdr[1] & 0x7F)
	if length == 126 {
		var ext [2]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return 0, nil, err
		}
		length = int64(binary.BigEndian.Uint16(ext[:]))
	} else if length == 127 {
		var ext [8]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return 0, nil, err
		}
		length = int64(binary.BigEndian.Uint64(ext[:]))
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(c.br, payload); err != nil {
		return 0, nil, err
	}
	return opcode, payload, nil
}

func (c *wsClient) close() {
	c.conn.Close()
}

func TestRegisterDefaultPath(t *testing.T) {
	app := dreego.New()
	if err := Register(app, Options{}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	client := dialWS(t, srv, "/ws")
	defer client.close()
}

func TestRegisterCustomPath(t *testing.T) {
	app := dreego.New()
	if err := Register(app, Options{Path: "/socket"}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	client := dialWS(t, srv, "/socket")
	defer client.close()
}

func TestWebSocketEcho(t *testing.T) {
	app := dreego.New()
	if err := Register(app, Options{}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	client := dialWS(t, srv, "/ws")
	defer client.close()
	msg := []byte("hello echo")
	if err := client.writeFrame(0x1, msg); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	if err := client.conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	opcode, payload, err := client.readFrame()
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	if opcode != 0x1 {
		t.Fatalf("opcode = 0x%x, want 0x1 (text)", opcode)
	}
	if string(payload) != string(msg) {
		t.Fatalf("payload = %q, want %q", payload, msg)
	}
}

func TestBroadcastReachesClient(t *testing.T) {
	app := dreego.New()
	if err := Register(app, Options{}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	client := dialWS(t, srv, "/ws")
	defer client.close()
	time.Sleep(100 * time.Millisecond)
	hub := HubInstance()
	hub.Broadcast([]byte("hello"))
	if err := client.conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	opcode, payload, err := client.readFrame()
	if err != nil {
		t.Fatalf("read broadcast: %v", err)
	}
	if opcode != 0x1 {
		t.Fatalf("opcode = 0x%x, want 0x1", opcode)
	}
	if string(payload) != "hello" {
		t.Fatalf("payload = %q, want hello", payload)
	}
}

func TestRegisterAfterBuild(t *testing.T) {
	app := dreego.New()
	if err := app.Build(); err != nil {
		t.Fatal(err)
	}
	if err := Register(app, Options{}); !errors.Is(err, dreego.ErrAppBuilt) {
		t.Fatalf("register after build error = %v, want ErrAppBuilt", err)
	}
}

func TestRejectNonWebSocketRequest(t *testing.T) {
	app := dreego.New()
	if err := Register(app, Options{}); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(app.Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/ws")
	if err != nil {
		t.Fatalf("GET /ws: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}