package reverse_rpc_pb

import (
	"io"
	"net/rpc"
	"reflect"
	"sync"

	"github.com/cockroachdb/errors"
	rrpcpb "github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb/pb"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

var (
	ErrInvalidRequestBody     = errors.New("[RRPC] invalid request body")
	ErrMessageTypeNotMatch    = errors.New("[RRPC] message type not match")
	ErrUnknownContentEncoding = errors.New("[RRPC] unknown content encoding")
)

type RPCClientCodec struct {
	conn     io.ReadWriteCloser
	encoding rrpcpb.ContentEncoding

	request  rrpcpb.Request
	response rrpcpb.Response

	mutex   sync.Mutex
	pending map[uint64]string

	codec *ClientCodec
	log   *zap.SugaredLogger
}

func NewRPCClientCodecWithConn(conn io.ReadWriteCloser, encoding rrpcpb.ContentEncoding) *RPCClientCodec {
	return &RPCClientCodec{
		conn:     conn,
		encoding: encoding,
		pending:  make(map[uint64]string),
		codec:    NewClientCodec(),
		log:      zap.S().With("module", "reverse_rpc.client"),
	}
}

func NewRPCClientCodec(encoding rrpcpb.ContentEncoding) rpc.ClientCodec {
	return NewRPCClientCodecWithConn(nil, encoding)
}

func (c *RPCClientCodec) Reset(conn io.ReadWriteCloser) {
	c.conn = conn
}

func (c *RPCClientCodec) SetEncoding(encoding rrpcpb.ContentEncoding) {
	c.encoding = encoding
}

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

	if c.response.Status != 200 {
		c.log.Errorf("Request %d", c.response.Status)
	}

	return nil
}

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

func (c *RPCClientCodec) Close() error {
	return c.conn.Close()
}

func NewClient(conn io.ReadWriteCloser) *rpc.Client {
	return rpc.NewClientWithCodec(NewRPCClientCodecWithConn(conn, rrpcpb.ContentEncoding_BROTLI))
}
