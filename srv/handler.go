package srv

import (
	"context"
	"io"
	"log"
	"os"
	"sync"

	"github.com/quic-go/quic-go"

	"github.com/sirgallo/quicfiletransfer/common"
	"github.com/sirgallo/quicfiletransfer/md5"
	"github.com/sirgallo/quicfiletransfer/serialize"
)

//============================================= Server Handlers

// handleConnection
//	Accept multiple streams from a single connection since QUIC can multiplex streams.
func handleConnection(conn quic.Connection) error {
	for {
		stream, err := conn.AcceptStream(context.Background())
		if err != nil { 
			conn.CloseWithError(common.CONNECTION_ERROR, err.Error())
			return err 
		}

		go handleCommStream(conn, stream)
	}
}

// handleCommStream
//	The bidirectional communication channel between the client and server.
//	For individual streams get the file to transfer.
//	The server opens the file and determines the size of the chunk to send to the client.
//	The server then sends a metadata payload to the client containing filesize, chunksize, and the start offset to process.
//	The data from the chunk in the file is written to the stream to be received by the client.
func handleCommStream(conn quic.Connection, commStream quic.Stream) error {
	defer commStream.Close()
	var err error

	buf := make([]byte, common.CLIENT_PAYLOAD_MAX_LENGTH)
	payloadLength, err := commStream.Read(buf)
	if err != nil { 
		conn.CloseWithError(common.TRANSPORT_ERROR, err.Error())
		return err 
	}

	totalStreamsForFile := uint8(buf[0])
	fileName := string(buf[1:payloadLength])

	log.Printf("filename: %s, total streams for file: %d\n", fileName, totalStreamsForFile)
	
	file, err := os.Open(fileName)
	if err != nil { 
		conn.CloseWithError(common.INTERNAL_ERROR, err.Error())
		return err 
	}

	fileStat, err := file.Stat()
	if err != nil {
		file.Close()
		conn.CloseWithError(common.INTERNAL_ERROR, err.Error())
		return err
	}
	file.Close()

	fileSize := uint64(fileStat.Size())
	md5, err := md5.ReadMD5FromFile(fileName + ".md5")
	if err != nil {
		conn.CloseWithError(common.INTERNAL_ERROR, err.Error())
		return err 
	}

	log.Printf("fileSize: %d\n", fileSize)

	metaPayload := func() []byte {
		p := make([]byte, common.FILE_META_PAYLOAD_MAX_LENGTH)
		copy(p[:8], serialize.SerializeUint64(fileSize))
		copy(p[8:], md5)
		return p
	}()

	_, err = commStream.Write(metaPayload)
	if err != nil {
		conn.CloseWithError(common.TRANSPORT_ERROR, err.Error())
		return err
	}

	var multiplexWG sync.WaitGroup
	for s := range make([]uint8, totalStreamsForFile) {
		multiplexWG.Add(1)
		dataStream, err := conn.OpenUniStream()
		if err != nil {
			conn.CloseWithError(common.TRANSPORT_ERROR, err.Error())
			return err
		}

		go func(s uint8) {
			defer multiplexWG.Done()
			defer dataStream.Close()
			var streamErr error

			chunkSize := fileSize / uint64(totalStreamsForFile)
			startOffset := uint64(s) * chunkSize
			if fileSize % uint64(totalStreamsForFile) != 0 && uint8(s) == totalStreamsForFile - 1 {
				chunkSize += fileSize % uint64(totalStreamsForFile)
			}
		
			log.Printf("startOffset: %d, chunkSize: %d\n", startOffset, chunkSize)
		
			sendPayload := func() []byte {
				p := make([]byte, common.CHUNK_META_PAYLOAD_MAX_LENGTH)
				copy(p[:8], serialize.SerializeUint64(startOffset))
				copy(p[8:], serialize.SerializeUint64(chunkSize))
				
				return p
			}()
		
			_, streamErr = dataStream.Write(sendPayload)
			if streamErr != nil {
				conn.CloseWithError(common.TRANSPORT_ERROR, streamErr.Error()) 
				return 
			}

			f, streamErr := os.OpenFile(fileName, os.O_RDONLY, 0666)
			if streamErr != nil {
				conn.CloseWithError(common.INTERNAL_ERROR, streamErr.Error())
				return
			}
			defer f.Close()

			totalBytesStreamed := int64(0)
			for int64(chunkSize) > totalBytesStreamed {
				_, streamErr = f.Seek(int64(startOffset) + totalBytesStreamed, 0)
				if streamErr != nil { 
					conn.CloseWithError(common.INTERNAL_ERROR, streamErr.Error())
					return
				}
				
				var n int64
				copyChunk := func () int64 {
					if totalBytesStreamed + int64(STREAM_CHUNK_BUFFER_SIZE) > int64(chunkSize) {
						return int64(chunkSize) - totalBytesStreamed
					}
					return int64(STREAM_CHUNK_BUFFER_SIZE)
				}()

				n, streamErr = io.CopyN(dataStream, f, copyChunk)
				if streamErr == io.EOF { break }
				if streamErr != nil && streamErr != io.EOF {
					conn.CloseWithError(common.TRANSPORT_ERROR, streamErr.Error())
					return 
				}

				totalBytesStreamed += n
		
				_, streamErr = commStream.Write(serialize.SerializeUint64(uint64(n)))
				if streamErr != nil {
					conn.CloseWithError(common.TRANSPORT_ERROR, streamErr.Error()) 
					return 
				}
			}

			log.Println("successfully transferred chunk", s)
		}(uint8(s))
	}

	multiplexWG.Wait()
	log.Println("done")
	return nil
}

const STREAM_CHUNK_BUFFER_SIZE = 1024 * 1024 * 2 // 2KiB
