package reverse_rpc_json

import (
	"encoding/json"
	"fmt"
	"io"
	"net/rpc"
	"sync"
)

type Request struct {
	ID       uint64            `json:"id"`
	Method   string            `json:"method"`
	Status   int               `json:"status"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Params   json.RawMessage   `json:"params"`
}

type Response struct {
	ID       uint64            `json:"id"`
	Method   string            `json:"method"`
	Status   int               `json:"status"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Data     json.RawMessage   `json:"data"`
}

type rpcClientCodec struct {
	dec *json.Decoder
	enc *json.Encoder
	c   io.ReadWriteCloser

	req  Request
	resp Response

	mutex   sync.Mutex
	pending map[uint64]string
}

// newClientCodec returns a new rpc.ClientCodec using JSON-RPC on conn.
func newClientCodec(conn io.ReadWriteCloser) rpc.ClientCodec {
	return &rpcClientCodec{
		dec:     json.NewDecoder(conn),
		enc:     json.NewEncoder(conn),
		c:       conn,
		pending: make(map[uint64]string),
	}
}

func (c *rpcClientCodec) WriteRequest(r *rpc.Request, param interface{}) error {
	paramsBytes, err := json.Marshal(param)
	if err != nil {
		return err
	}

	c.mutex.Lock()
	c.pending[r.Seq] = r.ServiceMethod
	c.mutex.Unlock()
	c.req.Method = r.ServiceMethod
	c.req.Params = json.RawMessage(paramsBytes)
	c.req.ID = r.Seq
	return c.enc.Encode(&c.req)
}

func (c *rpcClientCodec) ReadResponseHeader(r *rpc.Response) error {
	if err := c.dec.Decode(&c.resp); err != nil {
		return err
	}

	c.mutex.Lock()
	r.ServiceMethod = c.pending[c.resp.ID]
	delete(c.pending, c.resp.ID)
	c.mutex.Unlock()

	r.Error = ""
	r.Seq = c.resp.ID
	if c.resp.Data == nil {
		r.Error = "unspecified error"
	}
	if c.resp.Status != 200 {
		var data map[string]interface{}
		err := json.Unmarshal(c.resp.Data, &data)
		if err != nil {
			return err
		}
		r.Error = fmt.Sprintf("%s", data["message"])
	}
	return nil
}

func (c *rpcClientCodec) ReadResponseBody(x interface{}) error {
	if x == nil {
		return nil
	}
	return json.Unmarshal(c.resp.Data, x)
}

func (c *rpcClientCodec) Close() error {
	return c.c.Close()
}

func NewClient(conn io.ReadWriteCloser) *rpc.Client {
	return rpc.NewClientWithCodec(newClientCodec(conn))
}
