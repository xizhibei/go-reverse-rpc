package reverse_rpc_pb

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"
	"sync"

	"github.com/andybalholm/brotli"
	rrpcpb "github.com/xizhibei/go-reverse-rpc/reverse_rpc_pb/pb"
)

var (
	ErrUnknownCompressor = errors.New("Uknown compressor")
)

type CompressorManager struct {
	byteReaderPool   sync.Pool
	bufferPool       sync.Pool
	gzipWriterPool   sync.Pool
	zlibWriterPool   sync.Pool
	brotliWriterPool sync.Pool
}

func NewCompressorManager() *CompressorManager {
	return &CompressorManager{
		byteReaderPool: sync.Pool{
			New: func() interface{} {
				return bytes.NewReader(nil)
			},
		},
		gzipWriterPool: sync.Pool{
			New: func() interface{} {
				return gzip.NewWriter(nil)
			},
		},
		zlibWriterPool: sync.Pool{
			New: func() interface{} {
				return zlib.NewWriter(nil)
			},
		},
		brotliWriterPool: sync.Pool{
			New: func() interface{} {
				return brotli.NewWriter(nil)
			},
		},
		bufferPool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

func (c *CompressorManager) Compress(tp rrpcpb.ContentEncoding, data []byte) ([]byte, error) {
	if data == nil {
		return nil, nil
	}

	switch tp {
	case rrpcpb.ContentEncoding_GZIP:
		return c.GzipCompress(data)
	case rrpcpb.ContentEncoding_DEFLATE:
		return c.ZlibCompress(data)
	case rrpcpb.ContentEncoding_BROTLI:
		return c.BrotliCompress(data)
	case rrpcpb.ContentEncoding_PLAIN:
		return data, nil
	default:
		return nil, ErrUnknownCompressor
	}
}

func (c *CompressorManager) Uncompress(tp rrpcpb.ContentEncoding, data []byte) ([]byte, error) {
	if data == nil {
		return nil, nil
	}

	switch tp {
	case rrpcpb.ContentEncoding_GZIP:
		return c.GzipUncompress(data)
	case rrpcpb.ContentEncoding_DEFLATE:
		return c.ZlibUncompress(data)
	case rrpcpb.ContentEncoding_BROTLI:
		return c.BrotliUncompress(data)
	case rrpcpb.ContentEncoding_PLAIN:
		return data, nil
	default:
		return nil, ErrUnknownCompressor
	}
}

func (c *CompressorManager) GzipUncompress(data []byte) ([]byte, error) {
	byteReader := c.byteReaderPool.Get().(*bytes.Reader)
	defer c.byteReaderPool.Put(byteReader)
	byteReader.Reset(data)

	reader, err := gzip.NewReader(byteReader)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func (c *CompressorManager) GzipCompress(data []byte) ([]byte, error) {
	gzipWritter := c.gzipWriterPool.Get().(*gzip.Writer)
	defer c.gzipWriterPool.Put(gzipWritter)

	buf := c.bufferPool.Get().(*bytes.Buffer)
	defer c.bufferPool.Put(buf)

	buf.Reset()
	gzipWritter.Reset(buf)

	_, err := gzipWritter.Write(data)
	if err != nil {
		return nil, err
	}
	err = gzipWritter.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *CompressorManager) ZlibUncompress(data []byte) ([]byte, error) {
	byteReader := c.byteReaderPool.Get().(*bytes.Reader)
	defer c.byteReaderPool.Put(byteReader)
	byteReader.Reset(data)

	reader, err := zlib.NewReader(byteReader)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

func (c *CompressorManager) ZlibCompress(data []byte) ([]byte, error) {
	writter := c.zlibWriterPool.Get().(*zlib.Writer)
	defer c.zlibWriterPool.Put(writter)

	buf := c.bufferPool.Get().(*bytes.Buffer)
	defer c.bufferPool.Put(buf)

	buf.Reset()
	writter.Reset(buf)

	_, err := writter.Write(data)
	if err != nil {
		return nil, err
	}
	err = writter.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *CompressorManager) BrotliUncompress(data []byte) ([]byte, error) {
	byteReader := c.byteReaderPool.Get().(*bytes.Reader)
	defer c.byteReaderPool.Put(byteReader)
	byteReader.Reset(data)

	reader := brotli.NewReader(byteReader)

	return io.ReadAll(reader)
}

func (c *CompressorManager) BrotliCompress(data []byte) ([]byte, error) {
	writter := c.brotliWriterPool.Get().(*brotli.Writer)
	defer c.brotliWriterPool.Put(writter)

	buf := c.bufferPool.Get().(*bytes.Buffer)
	defer c.bufferPool.Put(buf)

	buf.Reset()
	writter.Reset(buf)

	_, err := writter.Write(data)
	if err != nil {
		return nil, err
	}
	err = writter.Close()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
