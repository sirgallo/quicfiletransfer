package cli

import (
	"errors"

	"github.com/sirgallo/quicfiletransfer/common"
	"github.com/sirgallo/quicfiletransfer/serialize"
)

// deserializePayload
//	Initial metadata payload with remote filesize and md5.
//	Format:
//		bytes 0-7: uint64 representing the size of the file
//		bytes 8-23: md5 in byte format
func (cli *QuicClient) deserializeMetaPayload(payload []byte) (uint64, []byte, error) {
	if len(payload) != common.FILE_META_PAYLOAD_MAX_LENGTH {
		return 0, nil, errors.New("payload incorrect length")
	}
	remoteFileSize, err := serialize.DeserializeUint64(payload[:8])
	if err != nil { return 0, nil, err }
	return remoteFileSize, payload[8:], nil
}

// deserializeChunkPayload
//	Metadata payload regarding chunk size and start offset in file.
//	Format:
//		bytes 0-7: uint64 representing the start offset in the file where the stream should begin processing
//		bytes 8-16: uint64 representing the size of the chunk being received by the stream
func (cli *QuicClient) deserializeChunkPayload(payload []byte) (uint64, uint64, error) {
	var err error
	if len(payload) != common.CHUNK_META_PAYLOAD_MAX_LENGTH {
		return 0, 0, errors.New("payload incorrect length")
	}
	startOffset, err := serialize.DeserializeUint64(payload[:8])
	if err != nil { return 0, 0, err }
	chunkSize, err := serialize.DeserializeUint64(payload[8:])
	if err != nil { return 0, 0, err }
	return startOffset, chunkSize, nil
}
