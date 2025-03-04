// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package specs

import (
	"fmt"
	"math"
	"sync"

	"github.com/klauspost/compress/zstd"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/gen/optional"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/omni/client/pkg/constants"
)

// CompressionConfig represents the configuration for compression.
type CompressionConfig struct {
	ZstdEncoder  *zstd.Encoder
	ZstdDecoder  *zstd.Decoder
	BufferPool   BufferPool
	Enabled      bool
	MinThreshold int // ensure that value stays in sync with `compressionThresholdBytes`
}

var (
	compressionConfig = CompressionConfig{
		ZstdEncoder: ensure.Value(zstd.NewWriter(
			nil,
			zstd.WithEncoderConcurrency(2),
			zstd.WithWindowSize(1<<18), // 256 KB
		)),
		ZstdDecoder:  ensure.Value(zstd.NewReader(nil, zstd.WithDecoderConcurrency(0))),
		BufferPool:   &NoOpBufferPool{},
		Enabled:      true,
		MinThreshold: constants.CompressionThresholdBytes,
	}

	compressionConfigMu sync.RWMutex
)

// SetCompressionConfig sets the default compression config to be used when an explicit config is not provided to accessor methods.
func SetCompressionConfig(config CompressionConfig) {
	compressionConfigMu.Lock()
	defer compressionConfigMu.Unlock()

	compressionConfig = config
}

// GetCompressionConfig returns the current default compression config.
func GetCompressionConfig() CompressionConfig {
	compressionConfigMu.RLock()
	defer compressionConfigMu.RUnlock()

	return compressionConfig
}

func getCompressionConfig(opts []CompressionOption) CompressionConfig {
	var options compressionOptions

	for _, opt := range opts {
		opt(&options)
	}

	config := options.Config.ValueOr(GetCompressionConfig())
	config.MinThreshold = options.minThreshold.ValueOr(config.MinThreshold)

	return config
}

type compressionOptions struct {
	Config       optional.Optional[CompressionConfig]
	minThreshold optional.Optional[int]
}

// CompressionOption is a functional option for configuring compression.
type CompressionOption func(*compressionOptions)

// WithConfigCompressionOption returns a CompressionOption that sets the given config as the compression config to be used instead of the default one.
func WithConfigCompressionOption(config CompressionConfig) CompressionOption {
	return func(opts *compressionOptions) {
		opts.Config = optional.Some(config)
	}
}

// WithCompressionMinThreshold returns a CompressionOption that sets the min threshold for compression.
func WithCompressionMinThreshold(threshold int) CompressionOption {
	return func(opts *compressionOptions) {
		opts.minThreshold = optional.Some(threshold)
	}
}

// FieldCompressor is an interface for the specs that contain a compressed field.
type FieldCompressor[T, R any] interface {
	proto.Message
	GetUncompressedData(...CompressionOption) (T, error)
	SetUncompressedData(R, ...CompressionOption) error
}

var jsonMarshaler = protojson.MarshalOptions{
	UseProtoNames:  true,
	UseEnumNumbers: true,
}

func doCompress(data []byte, opts CompressionConfig) []byte {
	return opts.ZstdEncoder.EncodeAll(data, nil)
}

func doDecompress(data []byte, config CompressionConfig) (Buffer, error) {
	if len(data) == 0 {
		return newNoOpBuffer(nil), nil
	}

	size, err := decompressedMaxSize(data)
	if err != nil {
		return Buffer{}, err
	}

	buffer := config.BufferPool.Get(size)
	dataRef := buffer.DataRef()

	decoded, err := config.ZstdDecoder.DecodeAll(data, *dataRef)
	if err != nil {
		return Buffer{}, err
	}

	*dataRef = decoded

	return buffer, nil
}

// decompressedMaxSize returns the max size of the decompressed data.
func decompressedMaxSize(src []byte) (int, error) {
	if len(src) == 0 {
		return 0, nil
	}

	var header zstd.Header

	if err := header.Decode(src); err != nil {
		return 0, err
	}

	if !header.HasFCS {
		// zstd does not set the frame content size in the header if the data is smaller than 256 bytes.
		// assume the max size to be 256 bytes.
		return 256, nil
	}

	// check for overflow
	if header.FrameContentSize > uint64(math.MaxInt) {
		return 0, fmt.Errorf("frame content size %d is too large", header.FrameContentSize)
	}

	return int(header.FrameContentSize), nil
}

// Buffer represents a byte buffer that can be re-used to store data.
//
// It provides a read-only view of the data and a method to free the buffer.
// Free should be called when the buffer is no longer needed.
type Buffer struct {
	data     *[]byte
	freeFunc func()
}

// NewBuffer creates a new Buffer with the given data and free function.
func NewBuffer(data *[]byte, freeFunc func()) Buffer {
	return Buffer{
		data:     data,
		freeFunc: freeFunc,
	}
}

// DataRef returns a reference to the data slice.
func (c *Buffer) DataRef() *[]byte {
	return c.data
}

// Data returns the data slice.
func (c *Buffer) Data() []byte {
	return *c.data
}

// Free frees the buffer.
func (c *Buffer) Free() {
	c.freeFunc()
}

// BufferPool represents a pool of buffers.
type BufferPool interface {
	Get(length int) Buffer
}

// NoOpBufferPool is a no-op implementation of BufferPool.
type NoOpBufferPool struct{}

// Get implements BufferPool interface.
func (s *NoOpBufferPool) Get(length int) Buffer {
	return newNoOpBuffer(make([]byte, 0, length))
}

func newNoOpBuffer(data []byte) Buffer {
	return NewBuffer(&data, func() {})
}

func unmarshalYAML(spec FieldCompressor[Buffer, []byte], alias any, node *yaml.Node) error {
	if err := node.Decode(alias); err != nil {
		return err
	}

	return unmarshal(spec)
}

func unmarshalYAMLMultiple(specs FieldCompressor[[]Buffer, [][]byte], alias any, node *yaml.Node, config CompressionConfig) error {
	if err := node.Decode(alias); err != nil {
		return err
	}

	return unmarshalMultiple(specs, config)
}

func unmarshalJSON(spec FieldCompressor[Buffer, []byte], data []byte) error {
	if err := protojson.Unmarshal(data, spec); err != nil {
		return err
	}

	return unmarshal(spec)
}

func unmarshalJSONMultiple(specs FieldCompressor[[]Buffer, [][]byte], data []byte, config CompressionConfig) error {
	if err := protojson.Unmarshal(data, specs); err != nil {
		return err
	}

	return unmarshalMultiple(specs, config)
}

func unmarshal(spec FieldCompressor[Buffer, []byte]) error {
	buffer, err := spec.GetUncompressedData()
	if err != nil {
		return err
	}

	defer buffer.Free()

	data := buffer.Data()

	if err = spec.SetUncompressedData(data); err != nil {
		return err
	}

	return nil
}

func unmarshalMultiple(specs FieldCompressor[[]Buffer, [][]byte], config CompressionConfig) error {
	configOpt := WithConfigCompressionOption(config)

	buffers, err := specs.GetUncompressedData(configOpt)
	if err != nil {
		return err
	}

	defer func() {
		for _, buffer := range buffers {
			buffer.Free()
		}
	}()

	data := make([][]byte, 0, len(buffers))

	for _, buffer := range buffers {
		data = append(data, buffer.Data())
	}

	if err = specs.SetUncompressedData(data, configOpt); err != nil {
		return err
	}

	return nil
}
