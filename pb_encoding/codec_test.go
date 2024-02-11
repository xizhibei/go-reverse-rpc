package pb_encoding_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xizhibei/go-reverse-rpc/pb_encoding"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestServerCodec_Marshal(t *testing.T) {
	scodec := pb_encoding.NewProtobufServerCodec()
	ccodec := pb_encoding.NewProtobufClientCodec()

	encodings := []pb_encoding.ContentEncoding{
		pb_encoding.ContentEncoding_PLAIN,
		pb_encoding.ContentEncoding_GZIP,
		pb_encoding.ContentEncoding_DEFLATE,
		pb_encoding.ContentEncoding_BROTLI,
	}

	for _, e := range encodings {
		// Test decompression with non-nil body
		res := &pb_encoding.Response{
			Encoding: e,
			Body: &anypb.Any{
				TypeUrl: "[]byte",
				Value:   []byte("response body"),
			},
		}
		data, err := scodec.Marshal(res)
		assert.NoError(t, err)

		var resT pb_encoding.Response
		err = ccodec.Unmarshal(data, &resT)
		assert.NoError(t, err)
		assert.Equal(t, []byte("response body"), resT.Body.Value)

		// Test decompression with nil body
		res = &pb_encoding.Response{
			Encoding: e,
			Body:     nil,
		}
		data, err = scodec.Marshal(res)
		assert.NoError(t, err)

		err = ccodec.Unmarshal(data, &resT)
		assert.NoError(t, err)
		assert.Nil(t, res.Body)
	}
}

func TestClientCodec_Marshal(t *testing.T) {
	scodec := pb_encoding.NewProtobufServerCodec()
	ccodec := pb_encoding.NewProtobufClientCodec()

	encodings := []pb_encoding.ContentEncoding{
		pb_encoding.ContentEncoding_PLAIN,
		pb_encoding.ContentEncoding_GZIP,
		pb_encoding.ContentEncoding_DEFLATE,
		pb_encoding.ContentEncoding_BROTLI,
	}

	for _, e := range encodings {
		// Test compression with non-nil body
		req := &pb_encoding.Request{
			Encoding: e,
			Body: &anypb.Any{
				TypeUrl: "[]byte",
				Value:   []byte("response body"),
			},
		}
		data, err := ccodec.Marshal(req)
		assert.NoError(t, err)

		var reqT pb_encoding.Request
		err = scodec.Unmarshal(data, &reqT)
		assert.NoError(t, err)
		assert.Equal(t, []byte("response body"), reqT.Body.Value)

		// Test compression with nil body
		req = &pb_encoding.Request{
			Encoding: e,
			Body:     nil,
		}
		data, err = ccodec.Marshal(req)
		assert.NoError(t, err)

		err = scodec.Unmarshal(data, &reqT)
		assert.NoError(t, err)
		assert.Nil(t, reqT.Body)
	}
}
