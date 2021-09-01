package nbhttp

import (
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/lesismal/nbio"
)

type Client struct {
	Conn net.Conn

	Engine *Engine

	mux      sync.Mutex
	handlers []func(res *http.Response, err error)
}

func (c *Client) Close() {
	c.mux.Lock()
	defer c.mux.Unlock()
	for _, h := range c.handlers {
		h(nil, io.EOF)
	}
	c.handlers = nil
}

func (c *Client) CloseWithError(err error) {
	c.mux.Lock()
	defer c.mux.Unlock()
	for _, h := range c.handlers {
		h(nil, err)
	}
	c.handlers = nil
}

func (c *Client) onResponse(res *http.Response, err error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	switch len(c.handlers) {
	case 0:
	case 1:
		c.handlers[0](res, err)
		c.handlers = nil
	default:
		c.handlers[0](res, err)
		c.handlers = c.handlers[1:]
	}
}

func (c *Client) Do(req *http.Request, handler func(res *http.Response, err error)) {
	c.Engine.Execute(func() {
		if c.Conn == nil {
			// for test
			addr := "localhost:8888"
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				handler(nil, err)
				return
			}

			nbc, err := nbio.NBConn(conn)
			if err != nil {
				handler(nil, err)
				return
			}

			readLimit := 1024 * 1024 * 32
			processor := NewClientProcessor(c, c.onResponse)
			parser := NewParser(processor, true, readLimit, nbc.Execute)
			parser.Engine = c.Engine
			nbc.SetSession(parser)

			c.Conn, _ = c.Engine.AddConn(nbc)
		}

		data := []byte("POST /echo HTTP/1.1\r\nHost: localhost:8888\r\nContent-Length: 5\r\nAccept-Encoding: gzip\r\n\r\nhello")

		c.mux.Lock()
		defer c.mux.Unlock()

		_, err := c.Conn.Write(data)
		if err != nil {
			handler(nil, err)
			return
		}
		c.handlers = append(c.handlers, handler)
	})
}

func NewClient(engine *Engine) *Client {
	return &Client{
		Engine: engine,
	}
}
