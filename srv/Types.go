package srv

import "crypto/tls"
import "net/http"

import "github.com/quic-go/quic-go"


type QuicServerOpts struct {
	Host string
	Port int
	TlsCert *tls.Certificate
	HandshakeIdleTimeout *int
	Insecure bool
}

type QuicServer struct {
	listener *quic.Listener
	mux *http.ServeMux
	host string
	port int
	closeChan chan struct{}
}