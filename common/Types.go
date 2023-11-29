package common 


const FTRANSFER_PROTO = "quic-file-transfer"
const DEFAULT_HANDSHAKE_TIME = 3
const MAX_FILENAME_LENGTH = 1024
const NET_PROTOCOL = "udp4"

const (
	NO_ERROR = 0x0
	INTERNAL_ERROR = 0x1
	CONNECTION_ERROR = 0x2
	TRANSPORT_ERROR = 0x3
)