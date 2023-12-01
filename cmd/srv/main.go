package main

import ( 
	"crypto/tls"
	"flag"
	"log"
	"os"

	"github.com/sirgallo/quicfiletransfer/srv"

	customtls "github.com/sirgallo/quicfiletransfer/common/tls"
)


const HOST = "127.0.0.1"
const PORT = 1234
const ORG = "test"


func main() {
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

	var cert *tls.Certificate
	switch {
		case certPath == "" || keyPath == "":
			srvSelfSigned, genSrvCertErr := customtls.GenerateTLSCert(ORG)
			if genSrvCertErr != nil { log.Fatal(genSrvCertErr) }
	
			cert = srvSelfSigned
		default:
			fCert, readCertErr := os.ReadFile(certPath)
			if readCertErr != nil { log.Fatalf("Failed to read certificate file: %v", readCertErr) }
		
			fKey, readKeyErr := os.ReadFile(keyPath)
			if readKeyErr != nil { log.Fatalf("Failed to read private key file: %v", readKeyErr) }

			tlsCert, getCertErr := tls.X509KeyPair(fCert, fKey)
			if getCertErr != nil { log.Fatalf("Failed to load certificate: %v", getCertErr) }

			cert = &tlsCert
	}

	srvOpts := &srv.QuicServerOpts{ Host: host, Port: port, TlsCert: cert, EnableTracer: enableTracer }
	server, newSrvErr := srv.NewQuicServer(srvOpts)
	if newSrvErr != nil { log.Fatal(newSrvErr) }

	err := server.Listen()
	if err != nil { log.Fatal(err) }

	select{}
}