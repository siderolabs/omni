// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package compression provides a buffer pool for resource decompression.
// It also initializes the default buffer pool and zstd encoder/decoder with a dictionary for the specs.
package compression

import (
	_ "embed"
	"fmt"
	"sort"
	"sync"

	"github.com/klauspost/compress/zstd"

	"github.com/siderolabs/omni/client/api/omni/specs"
)

var (
	//go:embed data/config.1.zdict
	dict1 []byte

	noOpBufferPool = &specs.NoOpBufferPool{}
)

// InitConfig initializes the compression configuration for the resource specs.
func InitConfig(enabled bool) error {
	compressionConfig, err := BuildConfig(enabled, true, false)
	if err != nil {
		return fmt.Errorf("failed to build compression config: %w", err)
	}

	specs.SetCompressionConfig(compressionConfig)

	return nil
}

// BuildConfig creates a new CompressionConfig with the given options.
func BuildConfig(enabled, useDict, usePool bool) (specs.CompressionConfig, error) {
	encoderOpts := []zstd.EOption{
		zstd.WithEncoderConcurrency(2),
		zstd.WithWindowSize(1 << 18), // 256KB
	}

	decoderOpts := []zstd.DOption{
		zstd.WithDecoderConcurrency(0),
	}

	if useDict {
		encoderOpts = append(encoderOpts, zstd.WithEncoderDict(dict1))
		decoderOpts = append(decoderOpts, zstd.WithDecoderDicts(dict1))
	}

	encoder, err := zstd.NewWriter(nil, encoderOpts...)
	if err != nil {
		return specs.CompressionConfig{}, err
	}

	decoder, err := zstd.NewReader(nil, decoderOpts...)
	if err != nil {
		return specs.CompressionConfig{}, err
	}

	var pool specs.BufferPool = noOpBufferPool
	if usePool {
		pool = NewTieredBufferPool(
			1024, // 1KB
			[]int{
				32 * 1024,       // 32KB
				128 * 1024,      // 128KB
				512 * 1024,      // 512KB
				2 * 1024 * 1024, // 2MB
			})
	}

	return specs.CompressionConfig{
		Enabled:     enabled,
		ZstdEncoder: encoder,
		ZstdDecoder: decoder,
		BufferPool:  pool,
	}, nil
}

// TieredBufferPool is a buffer pool that uses multiple sized buffer pools.
type TieredBufferPool struct {
	sizedPools []*sizedBufferPool
	minSize    int
}

// NewTieredBufferPool creates a new TieredBufferPool with the given minimum size and pool sizes.
func NewTieredBufferPool(minSize int, poolSizes []int) *TieredBufferPool {
	sort.Ints(poolSizes)

	pools := make([]*sizedBufferPool, 0, len(poolSizes))
	for _, s := range poolSizes {
		pools = append(pools, newSizedBufferPool(s))
	}

	return &TieredBufferPool{
		minSize:    minSize,
		sizedPools: pools,
	}
}

func (t *TieredBufferPool) getPool(size int) specs.BufferPool {
	if size < t.minSize { // no need to pool too small buffers
		return nil
	}

	poolIdx := sort.Search(len(t.sizedPools), func(i int) bool {
		return t.sizedPools[i].defaultSize >= size
	})

	if poolIdx == len(t.sizedPools) {
		return nil
	}

	return t.sizedPools[poolIdx]
}

// Get implements the specs.BufferPool interface.
func (t *TieredBufferPool) Get(length int) specs.Buffer {
	pool := t.getPool(length)
	if pool == nil {
		pool = noOpBufferPool
	}

	return pool.Get(length)
}

type sizedBufferPool struct {
	pool        sync.Pool
	defaultSize int
}

func (p *sizedBufferPool) Get(int) specs.Buffer {
	buf := p.pool.Get().(*[]byte) //nolint:errcheck,forcetypeassert
	b := *buf

	clear(b[:cap(b)])

	*buf = b[:0]

	return specs.NewBuffer(buf, func() {
		p.pool.Put(buf)
	})
}

func newSizedBufferPool(size int) *sizedBufferPool {
	return &sizedBufferPool{
		pool: sync.Pool{
			New: func() any {
				buf := make([]byte, 0, size)

				return &buf
			},
		},
		defaultSize: size,
	}
}
