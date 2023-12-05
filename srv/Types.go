package srv

import (
	"crypto/tls"

	"github.com/quic-go/quic-go"
)


// QuicServerOpts: the options for the quic server on init
type QuicServerOpts struct {
	// Host: the host for server
	Host string
	// Port: the port the host is listening on
	Port int
	// TlsCert: the server certificate
	TlsCert *tls.Certificate
	// EnableTracer: adds a file logger to capture events on the http3 server
	EnableTracer bool
}

// QuicServer: the quic server implementation
type QuicServer struct {
	listener *quic.EarlyListener
	host string
	port int
}

const INITIAL_STREAM_CHUNK_SIZE = 1024 * 512 // 512KiB
const STREAM_CHUNK_BUFFER_SIZE = 1024 * 1024 * 2 // 2KiB