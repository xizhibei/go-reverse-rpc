package mqtt_json_client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/rpc"
	"sync"

	reverse_rpc "github.com/xizhibei/go-reverse-rpc"
	"github.com/xizhibei/go-reverse-rpc/reverse_rpc_json"
)

type rpcClientCodec struct {
	dec *json.Decoder
	enc *json.Encoder
	c   io.ReadWriteCloser

	req  reverse_rpc_json.Request
	resp reverse_rpc_json.Response

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

// WriteRequest writes a JSON-RPC request to the underlying connection.
// It takes a pointer to an rpc.Request and an interface{} as parameters.
// The param is marshaled into JSON format using the json.Marshal function.
// The method name, parameters, and request ID are set in the rpcClientCodec struct.
// Finally, the request is encoded and written to the connection using c.enc.Encode.
// If there is an error during the marshaling or encoding process, it is returned.
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

// ReadResponseHeader reads the response header from the RPC client codec.
// It decodes the response using the codec's decoder and populates the provided
// rpc.Response struct with the decoded values. It also handles error handling
// and updates the response's error, sequence number, and service method.
// If the response status is not 200, it attempts to unmarshal the response data
// into a map and sets the error message based on the "message" field in the map.
// Returns an error if there was an error decoding the response or unmarshaling
// the response data.
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
	if c.resp.Status != reverse_rpc.RPCStatusOK {
		var data map[string]interface{}
		err := json.Unmarshal(c.resp.Data, &data)
		if err != nil {
			return err
		}
		r.Error = fmt.Sprintf("%s", data["message"])
	}
	return nil
}

// ReadResponseBody reads the response body from the RPC client codec.
// It unmarshals the response data into the provided interface{} value.
// If the provided value is nil, it returns nil.
// Returns an error if there is an issue with unmarshaling the data.
func (c *rpcClientCodec) ReadResponseBody(x interface{}) error {
	if x == nil {
		return nil
	}
	return json.Unmarshal(c.resp.Data, x)
}

func (c *rpcClientCodec) Close() error {
	return c.c.Close()
}

// NewClient creates a new RPC client using the provided connection.
// It returns a pointer to an rpc.Client.
func NewClient(conn io.ReadWriteCloser) *rpc.Client {
	return rpc.NewClientWithCodec(newClientCodec(conn))
}
