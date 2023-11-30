package pool

import "sync"


// BufferPool: a pool to pre-allocate and reuse read/write buffers
type BufferPool struct {
	bufferSize uint64
	pool *sync.Pool
}