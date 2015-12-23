package http

import (
	"io"
	"time"

	websocket "github.com/gorilla/websocket"
)

type WebsocketIO struct {
	conn *websocket.Conn
	reader io.Reader
}

func NewWebsocketIO(conn *websocket.Conn) WebsocketIO {
	return WebsocketIO{conn, nil}
}

func (wsio WebsocketIO) Read(buf []byte) (int, error) {
	for {
		if wsio.reader == nil {
			_, reader, err := wsio.conn.NextReader()
			closeError, ok := err.(*websocket.CloseError)
			if ok && closeError.Code == 1000 {
				return 0, io.EOF
			}
			if err != nil {
				return 0, err 
			}
			wsio.reader = reader
		}
		n, err := wsio.reader.Read(buf)
		if (err != nil) {
			wsio.reader = nil
			if n == 0 && err == io.EOF {
				continue
			}
		}
		return n, err
	}
}

func (wsio WebsocketIO) Write(buf []byte) (int, error) {
	err := wsio.conn.WriteMessage(websocket.BinaryMessage, buf)
	if err != nil {
		return 0, err
	}
	return len(buf), nil
}

func (wsio WebsocketIO) Close() error {
	_ = wsio.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(2 * time.Second))
	return wsio.conn.Close()
}
