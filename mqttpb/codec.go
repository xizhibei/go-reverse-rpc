package mqttpb

import (
	"github.com/xizhibei/go-reverse-rpc/compressor"
	"google.golang.org/protobuf/proto"
)

// ProtobufServerCodec is a codec used by the server to marshal and unmarshal RPC messages.
type ProtobufServerCodec struct {
	compressor *compressor.CompressorManager
}

// NewProtobufServerCodec creates a new instance of ServerCodec with a default CompressorManager.
func NewProtobufServerCodec() *ProtobufServerCodec {
	return &ProtobufServerCodec{
		compressor: compressor.NewCompressorManager(),
	}
}

// NewServerCodecWithCompressor creates a new instance of ServerCodec with the provided CompressorManager.
func NewServerCodecWithCompressor(compressor *compressor.CompressorManager) *ProtobufServerCodec {
	return &ProtobufServerCodec{
		compressor: compressor,
	}
}

// convertEncoding converts the ContentEncoding to compressor.ContentEncoding.
func convertEncoding(e ContentEncoding) compressor.ContentEncoding {
	switch e {
	case ContentEncoding_PLAIN:
		return compressor.ContentEncodingPlain
	case ContentEncoding_GZIP:
		return compressor.ContentEncodingGzip
	case ContentEncoding_DEFLATE:
		return compressor.ContentEncodingDeflate
	case ContentEncoding_BROTLI:
		return compressor.ContentEncodingBrotli
	}
	return compressor.ContentEncodingPlain
}

// Marshal marshals the Response message into bytes.
// It compresses the response body if it is not nil.
func (c *ProtobufServerCodec) Marshal(res *Response) ([]byte, error) {
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

// Unmarshal unmarshals the bytes into the Request message.
// It decompresses the request body if it is not nil.
func (c *ProtobufServerCodec) Unmarshal(body []byte, req *Request) error {
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

// ProtobufClientCodec is a codec used by the client to marshal and unmarshal RPC messages.
type ProtobufClientCodec struct {
	compressor *compressor.CompressorManager
}

// NewProtobufClientCodec creates a new instance of ClientCodec with a default CompressorManager.
func NewProtobufClientCodec() *ProtobufClientCodec {
	return &ProtobufClientCodec{
		compressor: compressor.NewCompressorManager(),
	}
}

// NewClientCodecWithCompressor creates a new instance of ClientCodec with the provided CompressorManager.
func NewClientCodecWithCompressor(compressor *compressor.CompressorManager) *ProtobufClientCodec {
	return &ProtobufClientCodec{
		compressor: compressor,
	}
}

// Marshal marshals the Request message into bytes.
// It compresses the request body if it is not nil.
func (c *ProtobufClientCodec) Marshal(req *Request) ([]byte, error) {
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

// Unmarshal unmarshals the bytes into the Response message.
// It decompresses the response body if it is not nil.
func (c *ProtobufClientCodec) Unmarshal(body []byte, res *Response) error {
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
