package srv

import (
	"context"
	"io"
	"log"
	"os"
	"sync"

	"github.com/quic-go/quic-go"

	"github.com/sirgallo/quicfiletransfer/common"
	"github.com/sirgallo/quicfiletransfer/common/md5"
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

	buf := make([]byte, common.CLIENT_PAYLOAD_MAX_LENGTH)
	payloadLength, readPayloadErr := commStream.Read(buf)
	if readPayloadErr != nil { 
		conn.CloseWithError(common.TRANSPORT_ERROR, readPayloadErr.Error())
		return readPayloadErr 
	}

	totalStreamsForFile := uint8(buf[0])
	fileName := string(buf[1:payloadLength])

	log.Printf("filename: %s, total streams for file: %d\n", fileName, totalStreamsForFile)
	
	file, openErr := os.Open(fileName)
	if openErr != nil { 
		conn.CloseWithError(common.INTERNAL_ERROR, openErr.Error())
		return openErr 
	}

	fileStat, statErr := file.Stat()
	if statErr != nil {
		file.Close()
		conn.CloseWithError(common.INTERNAL_ERROR, openErr.Error()) 
		return statErr
	}

	file.Close()

	fileSize := uint64(fileStat.Size())
	md5, getMd5Err := md5.ReadMD5FromFile(fileName + ".md5")
	if getMd5Err != nil { return getMd5Err }

	log.Printf("fileSize: %d\n", fileSize)

	metaPayload := func() []byte {
		p := make([]byte, common.FILE_META_PAYLOAD_MAX_LENGTH)
		copy(p[:8], serialize.SerializeUint64(fileSize))
		copy(p[8:], md5)

		return p
	}()

	_, writeMetaErr := commStream.Write(metaPayload)
	if writeMetaErr != nil {
		conn.CloseWithError(common.TRANSPORT_ERROR, writeMetaErr.Error())
		return writeMetaErr
	}

	var multiplexWG sync.WaitGroup
	for s := range make([]uint8, totalStreamsForFile) {
		multiplexWG.Add(1)

		dataStream, openStreamErr := conn.OpenUniStream()
		if openStreamErr != nil {
			conn.CloseWithError(common.TRANSPORT_ERROR, openStreamErr.Error())
			return openStreamErr
		}

		go func(s uint8) {
			defer multiplexWG.Done()
			defer dataStream.Close()

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
		
			_, writeErr := dataStream.Write(sendPayload)
			if writeErr != nil {
				conn.CloseWithError(common.TRANSPORT_ERROR, openErr.Error()) 
				return 
			}

			f, openChunkErr := os.OpenFile(fileName, os.O_RDONLY, 0666)
			if openChunkErr != nil {
				conn.CloseWithError(common.INTERNAL_ERROR, openChunkErr.Error())
				return
			}

			defer f.Close()

			totalBytesStreamed := int64(0)
			for int64(chunkSize) > totalBytesStreamed {
				_, seekErr := f.Seek(int64(startOffset) + totalBytesStreamed, 0)
				if seekErr != nil { 
					conn.CloseWithError(common.INTERNAL_ERROR, seekErr.Error())
					return
				}
				
				var n int64
				var streamFileErr error

				if totalBytesStreamed + int64(STREAM_CHUNK_BUFFER_SIZE) > int64(chunkSize) {
					n, streamFileErr = io.CopyN(dataStream, f, int64(chunkSize) - totalBytesStreamed)
					if streamFileErr == io.EOF { break }
					if streamFileErr != nil && streamFileErr != io.EOF {
						conn.CloseWithError(common.TRANSPORT_ERROR, streamFileErr.Error())
						return 
					}

					totalBytesStreamed += n
				} else {
					n, streamFileErr = io.CopyN(dataStream, f, int64(STREAM_CHUNK_BUFFER_SIZE))
					if streamFileErr == io.EOF { break }
					if streamFileErr != nil && streamFileErr != io.EOF {
						conn.CloseWithError(common.TRANSPORT_ERROR, streamFileErr.Error())
						return 
					}
			
					totalBytesStreamed += n
				}
		
				_, writeBytesErr := commStream.Write(serialize.SerializeUint64(uint64(n)))
				if writeBytesErr != nil {
					conn.CloseWithError(common.TRANSPORT_ERROR, writeBytesErr.Error()) 
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