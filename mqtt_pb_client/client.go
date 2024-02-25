package mqtt_pb_client

import (
	"io"
	"net/rpc"
	"reflect"
	"sync"

	"github.com/cockroachdb/errors"
	rrpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/pb_encoding"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	// ErrInvalidRequestBody is an error indicating an invalid request body.
	ErrInvalidRequestBody = errors.New("[RRPC] invalid request body")

	// ErrMessageTypeNotMatch is an error indicating that the message type does not match.
	ErrMessageTypeNotMatch = errors.New("[RRPC] message type not match")

	// ErrUnknownContentEncoding is an error indicating an unknown content encoding.
	ErrUnknownContentEncoding = errors.New("[RRPC] unknown content encoding")
)

// RPCClientCodec is a custom implementation of rpc.ClientCodec interface.
// It handles encoding and decoding of requests and responses.
type RPCClientCodec struct {
	conn     io.ReadWriteCloser
	encoding pb_encoding.ContentEncoding

	request  pb_encoding.Request
	response pb_encoding.Response

	mutex   sync.Mutex
	pending map[uint64]string

	codec *pb_encoding.ProtobufClientCodec
	log   *zap.SugaredLogger
}

// NewRPCClientCodecWithConn creates a new instance of RPCClientCodec with the given connection and content encoding.
func NewRPCClientCodecWithConn(conn io.ReadWriteCloser, encoding pb_encoding.ContentEncoding) *RPCClientCodec {
	return &RPCClientCodec{
		conn:     conn,
		encoding: encoding,
		pending:  make(map[uint64]string),
		codec:    pb_encoding.NewProtobufClientCodec(),
		log:      zap.S().With("module", "rrpc.mqtt_pb_client"),
	}
}

// NewRPCClientCodec creates a new instance of RPCClientCodec with the given content encoding.
// The connection is set to nil.
func NewRPCClientCodec(encoding pb_encoding.ContentEncoding) rpc.ClientCodec {
	return NewRPCClientCodecWithConn(nil, encoding)
}

// Reset resets the connection of the RPCClientCodec to the given connection.
func (c *RPCClientCodec) Reset(conn io.ReadWriteCloser) {
	c.conn = conn
}

// SetEncoding sets the content encoding of the RPCClientCodec to the given encoding.
func (c *RPCClientCodec) SetEncoding(encoding pb_encoding.ContentEncoding) {
	c.encoding = encoding
}

// WriteRequest writes the RPC request to the connection.
// It encodes the request body using protobuf and the specified content encoding.
func (c *RPCClientCodec) WriteRequest(r *rpc.Request, param interface{}) error {
	c.mutex.Lock()
	c.pending[r.Seq] = r.ServiceMethod
	c.mutex.Unlock()

	m, ok := param.(proto.Message)
	if !ok {
		return ErrInvalidRequestBody
	}

	value, err := proto.Marshal(m)
	if err != nil {
		return err
	}

	c.request.Id = r.Seq
	c.request.Encoding = c.encoding
	c.request.Method = r.ServiceMethod
	c.request.Body = &anypb.Any{
		TypeUrl: reflect.TypeOf(m).String(),
		Value:   value,
	}

	body, err := c.codec.Marshal(&c.request)
	if err != nil {
		return err
	}

	_, err = c.conn.Write(body)
	if err != nil {
		return err
	}

	return nil
}

// ReadResponseHeader reads the RPC response header from the connection.
// It decodes the response using the custom codec and updates the provided rpc.Response object.
func (c *RPCClientCodec) ReadResponseHeader(r *rpc.Response) error {
	c.response.Reset()

	body, err := io.ReadAll(c.conn)
	if err != nil {
		return err
	}

	if len(body) == 0 {
		return io.EOF
	}

	err = c.codec.Unmarshal(body, &c.response)
	if err != nil {
		return err
	}

	c.mutex.Lock()
	r.ServiceMethod = c.pending[c.response.Id]
	delete(c.pending, c.response.Id)
	c.mutex.Unlock()

	r.Error = c.response.ErrorMessage
	r.Seq = c.response.Id

	if c.response.Status != rrpc.RPCStatusOK {
		c.log.Errorf("Request %d", c.response.Status)
	}

	return nil
}

// ReadResponseBody reads the RPC response body from the connection.
// It decodes the response body using protobuf and updates the provided target object.
func (c *RPCClientCodec) ReadResponseBody(x interface{}) error {
	if x == nil {
		// c.log.Warn("target is empty")
		return nil
	}

	m, ok := x.(proto.Message)
	if !ok {
		return ErrInvalidRequestBody
	}

	if c.response.Body == nil || len(c.response.Body.Value) == 0 {
		c.log.Warn("body is empty")
		return nil
	}

	actual := c.response.Body.TypeUrl
	expect := reflect.TypeOf(m).String()

	if actual != expect {
		c.log.Warnf("Expect %s actual %s", expect, actual)
		return ErrMessageTypeNotMatch
	}

	return proto.Unmarshal(c.response.Body.Value, m)
}

// Close closes the connection of the RPCClientCodec.
func (c *RPCClientCodec) Close() error {
	return c.conn.Close()
}

// NewClient creates a new RPC client with the given connection.
// It uses the RPCClientCodec with BROTLI content encoding.
func NewClient(conn io.ReadWriteCloser) *rpc.Client {
	return rpc.NewClientWithCodec(NewRPCClientCodecWithConn(conn, pb_encoding.ContentEncoding_BROTLI))
}
