package websocket_json_server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
	"go.uber.org/zap"
)

// Service ...
type Service struct {
	*reverse_rpc.Server
	log    *zap.SugaredLogger
	conn   *websocket.Conn
	connMu sync.Mutex
	stop   bool

	uri  string
	host string
	path string
}

// New ...
func New(uri string, options ...reverse_rpc.ServerOption) (*Service, error) {
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	parsedURI.User = nil

	s := Service{
		Server: reverse_rpc.NewServer(options...),
		uri:    uri,
		host:   parsedURI.Host,
		path:   parsedURI.Path,
		log:    zap.S().With("module", "reverse_rpc.ws"),
	}

	s.initReceive()

	return &s, nil
}

func (s *Service) Close() error {
	s.stop = true
	return s.conn.Close()
}

type Request struct {
	ID     uint64          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type Response struct {
	ID     uint64      `json:"id"`
	Method string      `json:"method"`
	Status int         `json:"status"`
	Data   interface{} `json:"params"`
}

func (r *Request) GetResponse() *Response {
	return &Response{
		ID:     r.ID,
		Method: r.Method,
	}
}

func (r *Request) MakeOKResponse(data interface{}) *Response {
	res := r.GetResponse()
	res.Status = 200
	res.Data = data
	return res
}

func (r *Request) MakeErrResponse(status int, err error) *Response {
	res := r.GetResponse()
	res.Status = status
	res.Data = map[string]string{
		"message": err.Error(),
	}
	return res
}

func (s *Service) reply(res *Response) error {
	return s.conn.WriteJSON(res)
}

func (s *Service) Connect() error {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	header := http.Header{}
	conn, _, err := websocket.DefaultDialer.Dial(s.uri, header)
	if err != nil {
		return err
	}
	s.conn = conn

	return nil
}

func (s *Service) EnsureConnected() {
	if s.IsConnected() {
		return
	}

	go func() {
		for !s.stop {
			err := s.Connect()
			if err != nil {
				s.log.Errorf("%#v", err)
				time.Sleep(10 * time.Second)
				continue
			}
			break
		}
	}()
}

func (s *Service) initReceive() {
	go func() {
		for !s.stop {
			s.EnsureConnected()

			var req Request
			err := s.conn.ReadJSON(&req)
			if err != nil {
				s.log.Errorf("Read json %v, reconnect", err)
				_ = s.reply(req.MakeErrResponse(400, err))
				s.conn.Close()
				s.conn = nil
				continue
			}

			s.log.Infof("Request from method %s", req.Method)

			c := NewWSContext(&req, s)

			s.Server.Call(c)
		}
	}()
}

func (s *Service) IsConnected() bool {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	return s.conn != nil
}
