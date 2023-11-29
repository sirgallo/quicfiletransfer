package cli


type QuicClientOpts struct {
	Host string
	Port int
}

type QuicClient struct {
	address string
	port int
}

type OpenConnectionOpts struct {
	Insecure bool
}


const HANDSHAKE_TIMEOUT = 3