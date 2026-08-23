package websocket

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type Conn struct {
	conn     net.Conn
	br       *bufio.Reader
	isServer bool
}

func upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	if strings.ToLower(r.Header.Get("Upgrade")) != "websocket" {
		return nil, errors.New("not a websocket upgrade")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, errors.New("missing Sec-WebSocket-Key")
	}
	h := sha1.New()
	h.Write([]byte(key + wsGUID))
	accept := base64.StdEncoding.EncodeToString(h.Sum(nil))
	rc := http.NewResponseController(w)
	w.Header().Set("Upgrade", "websocket")
	w.Header().Set("Connection", "Upgrade")
	w.Header().Set("Sec-WebSocket-Accept", accept)
	w.WriteHeader(http.StatusSwitchingProtocols)
	rawConn, brw, err := rc.Hijack()
	if err != nil {
		return nil, err
	}
	brw.Flush()
	return &Conn{conn: rawConn, br: bufio.NewReader(brw.Reader), isServer: true}, nil
}

func (c *Conn) WriteText(data []byte) error {
	return c.writeFrame(0x1, data)
}

func (c *Conn) WritePing(data []byte) error {
	return c.writeFrame(0x9, data)
}

func (c *Conn) WritePong(data []byte) error {
	return c.writeFrame(0xA, data)
}

func (c *Conn) Close(code int, reason string) error {
	payload := make([]byte, 2+len(reason))
	binary.BigEndian.PutUint16(payload, uint16(code))
	copy(payload[2:], reason)
	c.writeFrame(0x8, payload)
	return c.conn.Close()
}

func (c *Conn) writeFrame(opcode byte, data []byte) error {
	var header []byte
	mask := byte(0)
	if !c.isServer {
		mask = 0x80
	}
	if len(data) < 126 {
		header = []byte{0x80 | opcode, mask | byte(len(data))}
	} else if len(data) < 65536 {
		header = make([]byte, 4)
		header[0] = 0x80 | opcode
		header[1] = mask | 126
		binary.BigEndian.PutUint16(header[2:], uint16(len(data)))
	} else {
		header = make([]byte, 10)
		header[0] = 0x80 | opcode
		header[1] = mask | 127
		binary.BigEndian.PutUint64(header[2:], uint64(len(data)))
	}
	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	if _, err := c.conn.Write(data); err != nil {
		return err
	}
	return nil
}

func (c *Conn) ReadMessage() (byte, []byte, error) {
	for {
		opcode, payload, err := c.readFrame()
		if err != nil {
			return 0, nil, err
		}
		switch opcode {
		case 0x8:
			return 0x8, payload, nil
		case 0x9:
			c.WritePong(payload)
			continue
		case 0xA:
			continue
		case 0x1, 0x2:
			return opcode, payload, nil
		default:
			return opcode, payload, nil
		}
	}
}

func (c *Conn) readFrame() (byte, []byte, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(c.br, hdr[:]); err != nil {
		return 0, nil, err
	}
	opcode := hdr[0] & 0x0F
	masked := hdr[1]&0x80 != 0
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
	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(c.br, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(c.br, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return opcode, payload, nil
}

func (c *Conn) CloseConn() error {
	return c.conn.Close()
}