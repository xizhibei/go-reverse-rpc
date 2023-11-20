package reverse_rpc_pb

import (
	rrpcpb "github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb/pb"
	"google.golang.org/protobuf/proto"
)

type ServerCodec struct {
	compressor *CompressorManager
}

func NewServerCodec() *ServerCodec {
	return &ServerCodec{
		compressor: NewCompressorManager(),
	}
}

func NewServerCodecWithCompressor(compressor *CompressorManager) *ServerCodec {
	return &ServerCodec{
		compressor: compressor,
	}
}

func (c *ServerCodec) Marshal(res *rrpcpb.Response) ([]byte, error) {
	if res.Body != nil {
		value, err := c.compressor.Compress(res.Encoding, res.Body.Value)
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

	value, err := c.compressor.Uncompress(req.Encoding, req.Body.Value)
	if err != nil {
		return err
	}
	req.Body.Value = value
	return nil
}

type ClientCodec struct {
	compressor *CompressorManager
}

func NewClientCodec() *ClientCodec {
	return &ClientCodec{
		compressor: NewCompressorManager(),
	}
}

func NewClientCodecWithCompressor(compressor *CompressorManager) *ClientCodec {
	return &ClientCodec{
		compressor: compressor,
	}
}

func (c *ClientCodec) Marshal(req *rrpcpb.Request) ([]byte, error) {
	if req.Body != nil {
		value, err := c.compressor.Compress(req.Encoding, req.Body.Value)
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

	value, err := c.compressor.Uncompress(res.Encoding, res.Body.Value)
	if err != nil {
		return err
	}
	res.Body.Value = value
	return nil
}
