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

// ContentEncoding represents the encoding type used for compressing content.
type ContentEncoding int

const (
	ContentEncodingGzip    ContentEncoding = 0
	ContentEncodingDeflate ContentEncoding = 1
	ContentEncodingBrotli  ContentEncoding = 2
	ContentEncodingPlain   ContentEncoding = 3
)

// ErrUnknownContentEncoding is an error indicating an unknown content encoding.
var (
	ErrUnknownContentEncoding = errors.New("[RRPC] uknown content encoding")
)

// CompressorManager is a manager for compressors.
type CompressorManager struct {
	byteReaderPool   sync.Pool
	bufferPool       sync.Pool
	gzipWriterPool   sync.Pool
	zlibWriterPool   sync.Pool
	brotliWriterPool sync.Pool
}

// NewCompressorManager creates a new instance of CompressorManager.
// CompressorManager manages the pools for byte readers and various compressors.
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

// Compress compresses the given data using the specified content encoding.
// It returns the compressed data and an error, if any.
// If the data is nil, it returns nil, nil.
// The supported content encodings are Gzip, Deflate, Brotli, and Plain.
// If an unknown content encoding is provided, it returns nil and an error.
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

// Decompress Decompresses the given data using the specified content encoding.
// It returns the Decompressed data and an error, if any.
// The supported content encodings are Gzip, Deflate, Brotli, and Plain.
// If the data is nil, it returns nil, nil.
// If the content encoding is unknown, it returns nil and an ErrUnknownContentEncoding error.
func (c *CompressorManager) Decompress(tp ContentEncoding, data []byte) ([]byte, error) {
	if data == nil {
		return nil, nil
	}

	switch tp {
	case ContentEncodingGzip:
		return c.GzipDecompress(data)
	case ContentEncodingDeflate:
		return c.ZlibDecompress(data)
	case ContentEncodingBrotli:
		return c.BrotliDecompress(data)
	case ContentEncodingPlain:
		return data, nil
	default:
		return nil, ErrUnknownContentEncoding
	}
}

// GzipDecompress Decompresses the given data using Gzip compression algorithm.
// It takes a byte slice as input and returns the Decompressed data as a byte slice.
// If an error occurs during the Decompression process, it returns the error.
func (c *CompressorManager) GzipDecompress(data []byte) ([]byte, error) {
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

// GzipCompress compresses the given data using gzip compression algorithm.
// It returns the compressed data as a byte slice and an error if any occurred.
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

// ZlibDecompress Decompresses the given data using the zlib compression algorithm.
// It returns the Decompressed data and any error encountered during the Decompression process.
func (c *CompressorManager) ZlibDecompress(data []byte) ([]byte, error) {
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

// ZlibCompress compresses the given data using the zlib compression algorithm.
// It returns the compressed data as a byte slice.
// If an error occurs during compression, it returns nil and the error.
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

// BrotliDecompress Decompresses the given data using Brotli compression algorithm.
// It takes a byte slice as input and returns the Decompressed data as a byte slice.
// If an error occurs during the Decompression process, it returns the error.
func (c *CompressorManager) BrotliDecompress(data []byte) ([]byte, error) {
	byteReader := c.byteReaderPool.Get().(*bytes.Reader)
	defer c.byteReaderPool.Put(byteReader)
	byteReader.Reset(data)

	reader := brotli.NewReader(byteReader)

	return io.ReadAll(reader)
}

// BrotliCompress compresses the given data using the Brotli compression algorithm.
// It returns the compressed data as a byte slice.
// If an error occurs during compression, it returns nil and the error.
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
