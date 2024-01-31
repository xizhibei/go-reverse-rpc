package reverse_rpc_pb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb/pb"
	rrpcpb "github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb/pb"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestServerCodec_Marshal(t *testing.T) {
	scodec := NewServerCodec()
	ccodec := NewClientCodec()

	// Test decompression with non-nil body
	res := &rrpcpb.Response{
		Encoding: pb.ContentEncoding_GZIP,
		Body: &anypb.Any{
			TypeUrl: "[]byte",
			Value:   []byte("response body"),
		},
	}
	data, err := scodec.Marshal(res)
	assert.NoError(t, err)

	var resT rrpcpb.Response
	err = ccodec.Unmarshal(data, &resT)
	assert.NoError(t, err)
	assert.Equal(t, []byte("response body"), resT.Body.Value)

	// Test decompression with nil body
	res = &rrpcpb.Response{
		Encoding: pb.ContentEncoding_GZIP,
		Body:     nil,
	}
	data, err = scodec.Marshal(res)
	assert.NoError(t, err)

	err = ccodec.Unmarshal(data, &resT)
	assert.NoError(t, err)
	assert.Nil(t, res.Body)
}

func TestClientCodec_Marshal(t *testing.T) {
	scodec := NewServerCodec()
	ccodec := NewClientCodec()

	// Test compression with non-nil body
	req := &rrpcpb.Request{
		Encoding: pb.ContentEncoding_GZIP,
		Body: &anypb.Any{
			TypeUrl: "[]byte",
			Value:   []byte("response body"),
		},
	}
	data, err := ccodec.Marshal(req)
	assert.NoError(t, err)

	var reqT rrpcpb.Request
	err = scodec.Unmarshal(data, &reqT)
	assert.NoError(t, err)
	assert.Equal(t, []byte("response body"), reqT.Body.Value)

	// Test compression with nil body
	req = &rrpcpb.Request{
		Encoding: pb.ContentEncoding_GZIP,
		Body:     nil,
	}
	data, err = ccodec.Marshal(req)
	assert.NoError(t, err)

	err = scodec.Unmarshal(data, &reqT)
	assert.NoError(t, err)
	assert.Nil(t, reqT.Body)
}
