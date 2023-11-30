package cli

import (
	"sync"
)


//============================================= Client Write Chunk Pool


func NewWriteChunkPool() *WriteChunkPool {
	pool := &sync.Pool{
  	New: func() interface{} { return &WriteChunk{} },
	}

	return &WriteChunkPool{ pool: pool }
}

// GetWriteChunk
//	Get a write chunk object from the pool.
func (p *WriteChunkPool) GetWriteChunk() *WriteChunk {
	wc := p.pool.Get().(*WriteChunk)
	return wc
}

// PutWriteChunk
//	Reset and put a write chunk object back in the pool.
func (p *WriteChunkPool) PutWriteChunk(wc *WriteChunk) {
	wc.resetWC()
	p.pool.Put(wc)
}