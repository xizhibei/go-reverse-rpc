package compressor

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"
	"sync"

	"github.com/andybalholm/brotli"
)

type ContentEncoding int

const (
	ContentEncodingGzip    ContentEncoding = 0
	ContentEncodingDeflate ContentEncoding = 1
	ContentEncodingBrotli  ContentEncoding = 2
	ContentEncodingPlain   ContentEncoding = 3
)

var (
	ErrUnknownContentEncoding = errors.New("[RRPC] uknown content encoding")
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

func (c *CompressorManager) Compress(tp ContentEncoding, data []byte) ([]byte, error) {
	if data == nil {
		return nil, nil
	}

	switch tp {
	case ContentEncodingGzip:
		return c.GzipCompress(data)
	case ContentEncodingDeflate:
		return c.ZlibCompress(data)
	case ContentEncodingBrotli:
		return c.BrotliCompress(data)
	case ContentEncodingPlain:
		return data, nil
	default:
		return nil, ErrUnknownContentEncoding
	}
}

func (c *CompressorManager) Uncompress(tp ContentEncoding, data []byte) ([]byte, error) {
	if data == nil {
		return nil, nil
	}

	switch tp {
	case ContentEncodingGzip:
		return c.GzipUncompress(data)
	case ContentEncodingDeflate:
		return c.ZlibUncompress(data)
	case ContentEncodingBrotli:
		return c.BrotliUncompress(data)
	case ContentEncodingPlain:
		return data, nil
	default:
		return nil, ErrUnknownContentEncoding
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
