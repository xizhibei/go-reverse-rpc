package mqttpb_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xizhibei/go-reverse-rpc/mqttpb"
	"google.golang.org/protobuf/types/known/anypb"
)

func TestServerCodec_Marshal(t *testing.T) {
	scodec := mqttpb.NewProtobufServerCodec()
	ccodec := mqttpb.NewProtobufClientCodec()

	encodings := []mqttpb.ContentEncoding{
		mqttpb.ContentEncoding_PLAIN,
		mqttpb.ContentEncoding_GZIP,
		mqttpb.ContentEncoding_DEFLATE,
		mqttpb.ContentEncoding_BROTLI,
	}

	for _, e := range encodings {
		// Test decompression with non-nil body
		res := &mqttpb.Response{
			Encoding: e,
			Body: &anypb.Any{
				TypeUrl: "[]byte",
				Value:   []byte("response body"),
			},
		}
		data, err := scodec.Marshal(res)
		assert.NoError(t, err)

		var resT mqttpb.Response
		err = ccodec.Unmarshal(data, &resT)
		assert.NoError(t, err)
		assert.Equal(t, []byte("response body"), resT.Body.Value)

		// Test decompression with nil body
		res = &mqttpb.Response{
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
	scodec := mqttpb.NewProtobufServerCodec()
	ccodec := mqttpb.NewProtobufClientCodec()

	encodings := []mqttpb.ContentEncoding{
		mqttpb.ContentEncoding_PLAIN,
		mqttpb.ContentEncoding_GZIP,
		mqttpb.ContentEncoding_DEFLATE,
		mqttpb.ContentEncoding_BROTLI,
	}

	for _, e := range encodings {
		// Test compression with non-nil body
		req := &mqttpb.Request{
			Encoding: e,
			Body: &anypb.Any{
				TypeUrl: "[]byte",
				Value:   []byte("response body"),
			},
		}
		data, err := ccodec.Marshal(req)
		assert.NoError(t, err)

		var reqT mqttpb.Request
		err = scodec.Unmarshal(data, &reqT)
		assert.NoError(t, err)
		assert.Equal(t, []byte("response body"), reqT.Body.Value)

		// Test compression with nil body
		req = &mqttpb.Request{
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
