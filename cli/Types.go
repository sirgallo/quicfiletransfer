package cli

import (
	"os"
	"sync"
	"sync/atomic"

	"github.com/sirgallo/quicfiletransfer/pool"
)


// QuicClientOpts: options on client init
type QuicClientOpts struct {
	// Host: the host for the remote server
	Host string
	// Port: the port the host is listening on
	Port int
	// Streams: the number of streams the client should open (100 is default max)
	Streams uint8
	// Writers: the number of writers to create to write the stream data to disk
	Writers uint8
}

// QuicClient: the quic client implementation
type QuicClient struct {
	address string
	port int
	streams uint8
	writers uint8

	copyMu *sync.Mutex
	dstFile *os.File
	data atomic.Value
	signalFlushChan chan bool
	writeChunkChan chan *WriteChunk
	isResizing uint64
	writeChunkSize uint64

	writePool *pool.BufferPool
	wcPool *WriteChunkPool
}

// OpenConnectionOpts: options to pass when opening a new connection
type OpenConnectionOpts struct {
	// Insecure: tells the client to not verify server certs. Should only be used for testing
	Insecure bool
}

// WriteChunk: a chunk of the incoming file to be written to disk 
type WriteChunk struct {
	offset uint64
	data []byte
}

// WriteChunkPool: a pool to pre-allocate and reuse write chunk objects
type WriteChunkPool struct {
	pool *sync.Pool
}


const HANDSHAKE_TIMEOUT = 3
const STREAM_BUFFER_SIZE = 1024 * 2 // 2KB
const MAX_BATCHED_WRITE_SIZE = 1024 * 1024 * 1024 // 1GB