package reverse_rpc_pb

import (
	"github.com/xizhibei/go-reverse-rpc/compressor"
	"github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb/pb"
	rrpcpb "github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb/pb"
	"google.golang.org/protobuf/proto"
)

// ServerCodec is a codec used by the server to marshal and unmarshal RPC messages.
type ServerCodec struct {
	compressor *compressor.CompressorManager
}

// NewServerCodec creates a new instance of ServerCodec with a default CompressorManager.
func NewServerCodec() *ServerCodec {
	return &ServerCodec{
		compressor: compressor.NewCompressorManager(),
	}
}

// NewServerCodecWithCompressor creates a new instance of ServerCodec with the provided CompressorManager.
func NewServerCodecWithCompressor(compressor *compressor.CompressorManager) *ServerCodec {
	return &ServerCodec{
		compressor: compressor,
	}
}

// convertEncoding converts the pb.ContentEncoding to compressor.ContentEncoding.
func convertEncoding(e pb.ContentEncoding) compressor.ContentEncoding {
	switch e {
	case pb.ContentEncoding_PLAIN:
		return compressor.ContentEncodingPlain
	case pb.ContentEncoding_GZIP:
		return compressor.ContentEncodingGzip
	case pb.ContentEncoding_DEFLATE:
		return compressor.ContentEncodingDeflate
	case pb.ContentEncoding_BROTLI:
		return compressor.ContentEncodingBrotli
	}
	return compressor.ContentEncodingPlain
}

// Marshal marshals the rrpcpb.Response message into bytes.
// It compresses the response body if it is not nil.
func (c *ServerCodec) Marshal(res *rrpcpb.Response) ([]byte, error) {
	if res.Body != nil {
		value, err := c.compressor.Compress(convertEncoding(res.Encoding), res.Body.Value)
		if err != nil {
			return nil, err
		}
		res.Body.Value = value
	}

	data, err := proto.Marshal(res)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Unmarshal unmarshals the bytes into the rrpcpb.Request message.
// It decompresses the request body if it is not nil.
func (c *ServerCodec) Unmarshal(body []byte, req *rrpcpb.Request) error {
	err := proto.Unmarshal(body, req)
	if err != nil {
		return err
	}

	if req.Body == nil || req.Body.Value == nil {
		return nil
	}

	value, err := c.compressor.Decompress(convertEncoding(req.Encoding), req.Body.Value)
	if err != nil {
		return err
	}
	req.Body.Value = value
	return nil
}

// ClientCodec is a codec used by the client to marshal and unmarshal RPC messages.
type ClientCodec struct {
	compressor *compressor.CompressorManager
}

// NewClientCodec creates a new instance of ClientCodec with a default CompressorManager.
func NewClientCodec() *ClientCodec {
	return &ClientCodec{
		compressor: compressor.NewCompressorManager(),
	}
}

// NewClientCodecWithCompressor creates a new instance of ClientCodec with the provided CompressorManager.
func NewClientCodecWithCompressor(compressor *compressor.CompressorManager) *ClientCodec {
	return &ClientCodec{
		compressor: compressor,
	}
}

// Marshal marshals the rrpcpb.Request message into bytes.
// It compresses the request body if it is not nil.
func (c *ClientCodec) Marshal(req *rrpcpb.Request) ([]byte, error) {
	if req.Body != nil {
		value, err := c.compressor.Compress(convertEncoding(req.Encoding), req.Body.Value)
		if err != nil {
			return nil, err
		}
		req.Body.Value = value
	}

	data, err := proto.Marshal(req)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Unmarshal unmarshals the bytes into the rrpcpb.Response message.
// It decompresses the response body if it is not nil.
func (c *ClientCodec) Unmarshal(body []byte, res *rrpcpb.Response) error {
	err := proto.Unmarshal(body, res)
	if err != nil {
		return err
	}

	if res.Body == nil {
		return nil
	}

	value, err := c.compressor.Decompress(convertEncoding(res.Encoding), res.Body.Value)
	if err != nil {
		return err
	}
	res.Body.Value = value
	return nil
}
