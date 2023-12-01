package cli

import (
	//"os"
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
	// CheckMD5: optionally check the md5 file to ensure validity of data
	CheckMd5 bool
}

// QuicClient: the quic client implementation
type QuicClient struct {
	remoteAddress string
	cliPort int
	streams uint8
	dstFile string
	checkMd5 bool
}

// OpenConnectionOpts: options to pass when opening a new connection
type OpenConnectionOpts struct {
	// Insecure: tells the client to not verify server certs. Should only be used for testing
	Insecure bool
}


const HANDSHAKE_TIMEOUT = 3
// const PROGRESS_CHUNK_SIZE = 1024 * 1024 * 256 // 256MB