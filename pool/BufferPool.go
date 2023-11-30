package pool

import "sync"


//============================================= Buffer Pool


func NewBufferPool(bufferSize uint64) *BufferPool {
	pool := &sync.Pool{
    New: func() interface{} {
      buf := make([]byte, 0, bufferSize)
			return &buf
    },
	}

	return &BufferPool{ bufferSize: bufferSize, pool: pool }
}

// GetBuffer
//	Get a presized buffer from the pool.
func (p *BufferPool) GetBuffer() []byte {
	buf := p.pool.Get().(*[]byte)
	return *buf
}

// PutBuffer
//	Reset and put a buffer back in the pool.
func (p *BufferPool) PutBuffer(buf []byte) {
	buf = buf[:0]
	p.pool.Put(&buf)
}