package srv

import (
	"context"
	"io"
	"log"
	"os"
	
	"github.com/quic-go/quic-go"
	
	"github.com/sirgallo/quicfiletransfer/common"
	"github.com/sirgallo/quicfiletransfer/common/serialize"
)


//============================================= Server Handlers


// handleConnection
//	Accept multiple streams from a single connection since QUIC can multiplex streams.
func handleConnection(conn quic.Connection) error {
	for {
		stream, streamErr := conn.AcceptStream(context.Background())
		if streamErr != nil { 
			conn.CloseWithError(common.CONNECTION_ERROR, streamErr.Error())
			return streamErr 
		}

		go handleStream(conn, stream)
	}
}

// handleStream
//	For individual streams, determine the file to transfer.
//	The server opens the file and determines the size of the chunk to send to the client.
//	The server then sends a metadata payload to the client containing filesize, chunksize, and the start offset to process.
//	The data from the chunk in the file is written to the stream to be received by the client.
func handleStream(conn quic.Connection, stream quic.Stream) error {
	defer stream.Close()

	buf := make([]byte, common.CLIENT_PAYLOAD_MAX_LENGTH)
	payloadLength, readPayloadErr := stream.Read(buf)
	if readPayloadErr != nil { 
		conn.CloseWithError(common.TRANSPORT_ERROR, readPayloadErr.Error())
		return readPayloadErr 
	}

	totalStreamsForFile := uint8(buf[0])
	currentStream := uint8(buf[1])
	fileName := string(buf[2:payloadLength])

	log.Printf("filename: %s, total streams for file: %d, current stream: %d", fileName, totalStreamsForFile, currentStream)
	
	file, openErr := os.Open(fileName)
	if openErr != nil { 
		conn.CloseWithError(common.INTERNAL_ERROR, openErr.Error())
		return openErr 
	}
	
	defer file.Close()

	fileStat, statErr := file.Stat()
	if statErr != nil {
		conn.CloseWithError(common.INTERNAL_ERROR, openErr.Error()) 
		return statErr
	}

	fileSize := uint64(fileStat.Size())
	chunkSize := fileSize / uint64(totalStreamsForFile)
	startOffset := uint64(currentStream) * chunkSize

	if fileSize % uint64(totalStreamsForFile) != 0 && currentStream == totalStreamsForFile - 1 { chunkSize += fileSize % uint64(totalStreamsForFile) }

	log.Printf("fileSize: %d, startOffset: %d, chunkSize: %d\n", fileSize, startOffset, chunkSize)

	payload := func() []byte {
		p := make([]byte, common.SERVER_PAYLOAD_MAX_LENGTH)
		copy(p[:8], serialize.SerializeUint64(fileSize))
		copy(p[8:16], serialize.SerializeUint64(startOffset))
		copy(p[16:], serialize.SerializeUint64(chunkSize))
		
		return p
	}()

	_, writeErr := stream.Write(payload)
	if writeErr != nil {
		conn.CloseWithError(common.TRANSPORT_ERROR, openErr.Error()) 
		return writeErr 
	}

	_, seekErr := file.Seek(int64(startOffset), 0)
	if seekErr != nil { 
		conn.CloseWithError(common.INTERNAL_ERROR, openErr.Error())
		return seekErr
	}

	_, streamFileErr := io.CopyN(stream, file, int64(chunkSize))
	if streamFileErr != nil && streamFileErr != io.EOF { 
		conn.CloseWithError(common.TRANSPORT_ERROR, streamFileErr.Error())
		return streamFileErr 
	}
	
	log.Println("successfully transferred file chunk")
	return nil
}