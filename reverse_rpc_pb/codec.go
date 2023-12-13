package reverse_rpc_pb

import (
	"github.com/xizhibei/go-reverse-rpc/compressor"
	"github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb/pb"
	rrpcpb "github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb/pb"
	"google.golang.org/protobuf/proto"
)

type ServerCodec struct {
	compressor *compressor.CompressorManager
}

func NewServerCodec() *ServerCodec {
	return &ServerCodec{
		compressor: compressor.NewCompressorManager(),
	}
}

func NewServerCodecWithCompressor(compressor *compressor.CompressorManager) *ServerCodec {
	return &ServerCodec{
		compressor: compressor,
	}
}

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

func (c *ServerCodec) Unmarshal(body []byte, req *rrpcpb.Request) error {
	err := proto.Unmarshal(body, req)
	if err != nil {
		return err
	}

	value, err := c.compressor.Uncompress(convertEncoding(req.Encoding), req.Body.Value)
	if err != nil {
		return err
	}
	req.Body.Value = value
	return nil
}

type ClientCodec struct {
	compressor *compressor.CompressorManager
}

func NewClientCodec() *ClientCodec {
	return &ClientCodec{
		compressor: compressor.NewCompressorManager(),
	}
}

func NewClientCodecWithCompressor(compressor *compressor.CompressorManager) *ClientCodec {
	return &ClientCodec{
		compressor: compressor,
	}
}

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

func (c *ClientCodec) Unmarshal(body []byte, res *rrpcpb.Response) error {
	err := proto.Unmarshal(body, res)
	if err != nil {
		return err
	}

	if res.Body == nil {
		return nil
	}

	value, err := c.compressor.Uncompress(convertEncoding(res.Encoding), res.Body.Value)
	if err != nil {
		return err
	}
	res.Body.Value = value
	return nil
}
