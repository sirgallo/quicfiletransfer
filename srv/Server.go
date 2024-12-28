package srv

import ( 
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/logging"
	"github.com/quic-go/quic-go/qlog"

	"github.com/sirgallo/quicfiletransfer/common"
)

//============================================= Server

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

// NewQuicServer
//	Create the quic file transfer server.
//	If tracer is enabled, a log of all events will be dumped to the directy the server is run in.
func NewQuicServer(opts *QuicServerOpts) (*QuicServer, error) {
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{*opts.TlsCert}, NextProtos: []string{ common.FTRANSFER_PROTO }}
	quicConfig := &quic.Config{ Allow0RTT: true, EnableDatagrams: true, KeepAlivePeriod: 3 * time.Second }

	if opts.EnableTracer {
		log.Printf("enable tracer: %t\n", opts.EnableTracer)
		tracer := func(ctx context.Context, p logging.Perspective, connID quic.ConnectionID) *logging.ConnectionTracer {
			role := "server"
			if p == logging.PerspectiveClient{role = "client"}
			filename := fmt.Sprintf("./log_%s_%s.qlog", connID, role)
			f, createErr := os.Create(filename)
			if createErr != nil { log.Fatal(createErr) }
			return qlog.NewConnectionTracer(f, p, connID)
		}

		quicConfig.Tracer = tracer
	}
	var err error
	udpConn, err := net.ListenUDP(common.NET_PROTOCOL, &net.UDPAddr{ IP: net.ParseIP(opts.Host), Port: opts.Port })
	if err != nil { return nil, err }

	tr := quic.Transport{ Conn: udpConn }
	listener, err := tr.ListenEarly(tlsConfig, quicConfig)
	if err != nil { return nil, err }

	log.Printf("quic transport layer started for: %s\n", listener.Addr().String())
	return &QuicServer{host: opts.Host, port: opts.Port, listener: listener}, nil
}

// Listen
//	Begin accepting and processing connections from clients.
//	This is asynchronous.
func (srv *QuicServer) Listen() error {
	defer srv.listener.Close()
	var listenWG sync.WaitGroup
	listenWG.Add(1)

	go func() {
		defer listenWG.Done()
		for {
			conn, err := srv.listener.Accept(context.Background())
			if err != nil { 
				log.Println("connection error:", err.Error())
				continue 
			}

			go func () {
				err := handleConnection(conn)
				if err != nil { log.Println("error on handler:", err.Error()) }
			}()
		}
	}()

	listenWG.Wait()
	return nil
}
