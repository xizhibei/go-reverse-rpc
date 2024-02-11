package websocket_json_server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
	"go.uber.org/zap"
)

// Service represents a websocket service.
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

// New creates a new Service instance with the provided URI and options.
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
		log:    zap.S().With("module", "rrpc.ws_json_server"),
	}

	s.initReceive()

	return &s, nil
}

// Close closes the WebSocket connection and stops the service.
// It sets the stop flag to true and then calls the Close method on the underlying connection.
// Returns an error if there was a problem closing the connection.
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

// GetResponse returns a new Response object based on the current Request object.
// The new Response object has the same ID and Method as the current Request object.
func (r *Request) GetResponse() *Response {
	return &Response{
		ID:     r.ID,
		Method: r.Method,
	}
}

// MakeOKResponse creates a successful response with the given data.
// It sets the status code to 200 and assigns the data to the response.
// Returns the created response.
func (r *Request) MakeOKResponse(data interface{}) *Response {
	res := r.GetResponse()
	res.Status = reverse_rpc.RPCStatusOK
	res.Data = data
	return res
}

// MakeErrResponse creates an error response with the specified status code and error message.
// It sets the response status and data fields accordingly.
// The error message is included in the "message" field of the response data.
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

// Connect establishes a WebSocket connection to the specified URI.
// It uses the default Dialer from the "github.com/gorilla/websocket" package.
// The connection is stored in the Service struct for future use.
// Returns an error if the connection cannot be established.
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

// EnsureConnected checks if the service is connected and establishes a connection if not.
// It runs in a separate goroutine and keeps trying to connect until successful or stopped.
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

			wsCtx := NewWSContext(&req, s)
			c := reverse_rpc.NewRequestContext(context.Background(), wsCtx)

			s.Server.Call(c)
		}
	}()
}

// IsConnected returns a boolean value indicating whether the service is currently connected to a client.
func (s *Service) IsConnected() bool {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	return s.conn != nil
}
