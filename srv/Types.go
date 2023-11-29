package srv

import (
	"crypto/tls"

	"github.com/quic-go/quic-go"
)


type QuicServerOpts struct {
	Host string
	Port int
	TlsCert *tls.Certificate
	HandshakeIdleTimeout *int
	Insecure bool
	EnableTracer bool
}

type QuicServer struct {
	listener *quic.Listener
	host string
	port int
}