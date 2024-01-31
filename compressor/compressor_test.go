package compressor

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/stretchr/testify/assert"
)

func TestNewCompressorManager(t *testing.T) {
	manager := NewCompressorManager()

	// Test byteReaderPool
	byteReader := manager.byteReaderPool.Get().(*bytes.Reader)
	assert.NotNil(t, byteReader)
	manager.byteReaderPool.Put(byteReader)

	// Test gzipWriterPool
	gzipWriter := manager.gzipWriterPool.Get().(*gzip.Writer)
	assert.NotNil(t, gzipWriter)
	manager.gzipWriterPool.Put(gzipWriter)

	// Test zlibWriterPool
	zlibWriter := manager.zlibWriterPool.Get().(*zlib.Writer)
	assert.NotNil(t, zlibWriter)
	manager.zlibWriterPool.Put(zlibWriter)

	// Test brotliWriterPool
	brotliWriter := manager.brotliWriterPool.Get().(*brotli.Writer)
	assert.NotNil(t, brotliWriter)
	manager.brotliWriterPool.Put(brotliWriter)

	// Test bufferPool
	buffer := manager.bufferPool.Get().(*bytes.Buffer)
	assert.NotNil(t, buffer)
	manager.bufferPool.Put(buffer)
}

func TestCompressorManager_Compress(t *testing.T) {
	manager := NewCompressorManager()

	// Test GzipCompress
	gzipData := []byte("gzip data")
	gzipCompressed, err := manager.GzipCompress(gzipData)
	assert.NoError(t, err)
	gzipDecompressed, err := manager.GzipDecompress(gzipCompressed)
	assert.NoError(t, err)
	assert.Equal(t, gzipData, gzipDecompressed)

	// Test ZlibCompress
	zlibData := []byte("zlib data")
	zlibCompressed, err := manager.ZlibCompress(zlibData)
	assert.NoError(t, err)
	zlibDecompressed, err := manager.ZlibDecompress(zlibCompressed)
	assert.NoError(t, err)
	assert.Equal(t, zlibData, zlibDecompressed)

	// Test BrotliCompress
	brotliData := []byte("brotli data")
	brotliCompressed, err := manager.BrotliCompress(brotliData)
	assert.NoError(t, err)
	brotliDecompressed, err := manager.BrotliDecompress(brotliCompressed)
	assert.NoError(t, err)
	assert.Equal(t, brotliData, brotliDecompressed)

	// Test plain data
	plainData := []byte("plain data")
	plainCompressed, err := manager.Compress(ContentEncodingPlain, plainData)
	assert.NoError(t, err)
	assert.Equal(t, plainData, plainCompressed)

	// Test unknown content encoding
	unknownData := []byte("unknown data")
	unknownCompressed, err := manager.Compress(4, unknownData)
	assert.Equal(t, ErrUnknownContentEncoding, err)
	assert.Nil(t, unknownCompressed)
}
