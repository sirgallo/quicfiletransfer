package cli

import (
	"os"
)


// QuicClientOpts: options on client init
type QuicClientOpts struct {
	// Host: the host for the remote server
	RemoteHost string
	// RemotePort: the port for the remote server
	RemotePort int
	// ClientPort: the port the client starts the udp connection with
	ClientPort int
	// Streams: the number of streams the client should open (100 is default max)
	Streams uint8
}

// QuicClient: the quic client implementation
type QuicClient struct {
	remoteAddress string
	cliPort int
	streams uint8
	dstFile *os.File
}

// OpenConnectionOpts: options to pass when opening a new connection
type OpenConnectionOpts struct {
	// Insecure: tells the client to not verify server certs. Should only be used for testing
	Insecure bool
}


const HANDSHAKE_TIMEOUT = 3