package srv

import ( 
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"sync"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/logging"
	"github.com/quic-go/quic-go/qlog"

	"github.com/sirgallo/quicfiletransfer/common"
)


func NewQuicServer(opts *QuicServerOpts) (*QuicServer, error) {
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{ *opts.TlsCert },
		NextProtos: []string{ common.FTRANSFER_PROTO },
	}

	quicConfig := &quic.Config{ Allow0RTT: true, EnableDatagrams: true }

	if opts.EnableTracer {
		log.Println("enable tracer:", opts.EnableTracer)
		tracer := func(ctx context.Context, p logging.Perspective, connID quic.ConnectionID) *logging.ConnectionTracer {
			role := "server"
			if p == logging.PerspectiveClient { role = "client" }
			
			filename := fmt.Sprintf("./log_%s_%s.qlog", connID, role)
			f, createErr := os.Create(filename)
			if createErr != nil { log.Fatal(createErr) }
			
			return qlog.NewConnectionTracer(f, p, connID)
		}

		quicConfig.Tracer = tracer
	}

	udpConn, udpErr := net.ListenUDP(common.NET_PROTOCOL, &net.UDPAddr{ IP: net.ParseIP(opts.Host), Port: opts.Port })
	if udpErr != nil { return nil, udpErr }

	tr := quic.Transport{ Conn: udpConn }
	listener, listenQuicErr := tr.Listen(tlsConfig, quicConfig)
	if listenQuicErr != nil { return nil, listenQuicErr }

	log.Printf("quic transport layer started for: %s", listener.Addr().String())

	return &QuicServer{
		host: opts.Host,
		port: opts.Port,
		listener: listener,
	}, nil
}

func (srv *QuicServer) Listen() error {
	defer srv.listener.Close()
	
	var listenWG sync.WaitGroup
	listenWG.Add(1)
	
	go func() {
		defer listenWG.Done()
		for {
			conn, connErr := srv.listener.Accept(context.Background())
			if connErr != nil { 
				log.Println("connection error:", connErr.Error())
				continue 
			}

			go func () {
				handleErr := handleSession(conn)
				if handleErr != nil { log.Println("error on handler:", handleErr.Error()) }
			}()
		}
	}()

	listenWG.Wait()
	return nil
}