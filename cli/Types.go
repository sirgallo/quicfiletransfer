package cli


type QuicClientOpts struct {
	Host string
	Port int16
}

type QuicClient struct {
	address string
}

type OpenConnectionOpts struct {
	Insecure bool
}