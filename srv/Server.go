package srv

import "context"
import "errors"
import "fmt"
import "log"
import "net"
import "net/http"
import "crypto/tls"
import "sync"

import "github.com/quic-go/quic-go"

import "github.com/sirgallo/quicfiletransfer/common"


func NewQuicServer(opts *QuicServerOpts) (*QuicServer, error) {
	mux := http.NewServeMux()

	tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{ *opts.TlsCert },
    NextProtos: []string{ common.FTRANSFER_PROTO },
	}

	quicConfig := &quic.Config{}

	localHostPort := net.JoinHostPort(opts.Host, fmt.Sprint(opts.Port))
	listener, listenQuicErr := quic.ListenAddr(localHostPort, tlsConfig, quicConfig)
	if listenQuicErr != nil { return nil, listenQuicErr }

	log.Printf("quic transport layer started for: %s", listener.Addr().String())

	return &QuicServer{
		host: opts.Host,
		port: opts.Port,
		listener: listener,
		mux: mux,
		closeChan: make(chan struct{}),
	}, nil
}

func (srv *QuicServer) Listen() error {
	defer close(srv.closeChan)

	errorChan := make(chan string)
	defer close(errorChan)
	
	var listenWG sync.WaitGroup

	listenWG.Add(1)
	go func() {
		defer listenWG.Done()
		select {
			case <- srv.closeChan:
				closeErr := srv.listener.Close()
				if closeErr != nil { errorChan <- closeErr.Error() }
				return
			default:
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
		}
	}()

	listenWG.Wait()

	select {
		case err :=<- errorChan:
			return errors.New(err)
		default:
			return nil
	}
}

func (srv *QuicServer) Close() {
	close(srv.closeChan)
}