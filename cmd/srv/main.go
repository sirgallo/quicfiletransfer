package main

import ( 
	"crypto/tls"
	"flag"
	"log"
	"os"

	"github.com/sirgallo/quicfiletransfer/internal/srv"
	customtls "github.com/sirgallo/quicfiletransfer/internal/common/tls"
)


const HOST = "0.0.0.0"
const PORT = 1234
const ORG = "test"


func main() {
	var err error
	var cert *tls.Certificate
	var host, org, certPath, keyPath string
	var port int
	var enableTracer bool

	flag.StringVar(&host, "host", HOST, "the host IP/domain for the quic server")
	flag.IntVar(&port, "port", PORT, "the port tot listen on")
	flag.StringVar(&org, "org", ORG, "the organization for self signed certs")
	flag.StringVar(&certPath, "certPath", "", "the path to the cert. If not provided will generate self signed")
	flag.StringVar(&keyPath, "keyPath", "", "the path the private key. If not provided will generate self signed")
	flag.BoolVar(&enableTracer, "enableTracer", false, "enable the tracer. This creates a log file in the working directory")

	flag.Parse()

	switch {
		case certPath == "" || keyPath == "":
			srvSelfSigned, err := customtls.GenerateTLSCert(ORG)
			if err != nil { log.Fatal(err) }
			cert = srvSelfSigned
		default:
			fCert, err := os.ReadFile(certPath)
			if err != nil { log.Fatalf("failed to read certificate file: %v", err) }
	
			fKey, err := os.ReadFile(keyPath)
			if err != nil { log.Fatalf("failed to read private key file: %v", err) }

			tlsCert, err := tls.X509KeyPair(fCert, fKey)
			if err != nil { log.Fatalf("failed to load certificate: %v", err) }
			cert = &tlsCert
	}

	srvOpts := &srv.QuicServerOpts{ Host: host, Port: port, TlsCert: cert, EnableTracer: enableTracer }
	server, err := srv.NewQuicServer(srvOpts)
	if err != nil { log.Fatal(err) }

	err = server.Listen()
	if err != nil { log.Fatal(err) }

	select{}
}