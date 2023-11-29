package srv

import (
	"context"
	"io"
	"log"
	"os"
	
	"github.com/quic-go/quic-go"
	
	"github.com/sirgallo/quicfiletransfer/common"
)


func handleSession(conn quic.Connection) error {
	stream, streamErr := conn.AcceptStream(context.Background())
	if streamErr != nil { 
		conn.CloseWithError(common.CONNECTION_ERROR, streamErr.Error())
		return streamErr 
	}

	defer stream.Close()

	buf := make([]byte, common.MAX_FILENAME_LENGTH)
	fileNameLength, readNameErr := stream.Read(buf)
	if readNameErr != nil { 
		conn.CloseWithError(common.TRANSPORT_ERROR, readNameErr.Error())
		return readNameErr 
	}

	fileName := string(buf[:fileNameLength])

	file, openErr := os.Open(fileName)
  if openErr != nil { 
		conn.CloseWithError(common.INTERNAL_ERROR, openErr.Error())
		return openErr 
	}

  defer file.Close()

	_, streamErr = io.Copy(stream, file)
	if streamErr != nil && streamErr != io.EOF { 
		conn.CloseWithError(common.TRANSPORT_ERROR, "end of file")
		return streamErr 
	}

	log.Println("successfully transferred file")
	return nil
}