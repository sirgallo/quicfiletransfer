package cli

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/sirgallo/quicfiletransfer/common"
	"github.com/sirgallo/quicfiletransfer/common/serialize"
)


//============================================= Client


// NewClient
//	Create a new quic file transfer client.
func NewClient(opts *QuicClientOpts) (*QuicClient, error) {
	remoteHostPort := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	log.Printf("remote server address: %s", remoteHostPort)

	return &QuicClient{ address: remoteHostPort, streams: opts.Streams, isResizing: uint64(0) }, nil
}

// StartFileTransferStream
//	Invoke a file transfer operation.
//	The client provides the total number of streams to open.
//	Once each stream receives a metadata response from the server, the file is resized.
//	The streams for the client connection then receive and write the file chunks from the server to disk.
func (cli *QuicClient) StartFileTransferStream(connectOpts *OpenConnectionOpts, filename, src, dst string) (*string, error){
	srcPath := filepath.Join(src, filename)
	dstpath := filepath.Join(dst, filename)
	
	var createErr error
	cli.dstFile, createErr = os.Create(dstpath)
	if createErr != nil { return nil, createErr }
	defer cli.dstFile.Close()

	conn, connErr := cli.openConnection(connectOpts)
	if connErr != nil { return nil, connErr }
	defer conn.CloseWithError(common.NO_ERROR, "closing")

	startTime := time.Now()
	
	var multiplexStreamWG sync.WaitGroup

	for s := range make([]uint8, cli.streams) {
		multiplexStreamWG.Add(1)
		go func(s uint8) { 
			defer multiplexStreamWG.Done()
		
			stream, openStreamErr := conn.OpenStream()
			if openStreamErr != nil { 
				conn.CloseWithError(common.CONNECTION_ERROR, openStreamErr.Error())
				return
			}

			payload := func() []byte {
				tags := []byte{ cli.streams, s }
				return append(tags, []byte(srcPath)...)
			}()
		
			_, writeErr := stream.Write(payload)
			if writeErr != nil { 
				conn.CloseWithError(common.TRANSPORT_ERROR, writeErr.Error())
				return 
			}

			buf := make([]byte, common.SERVER_PAYLOAD_MAX_LENGTH)
			payloadLength, readPayloadErr := stream.Read(buf)
			if readPayloadErr != nil { 
				conn.CloseWithError(common.TRANSPORT_ERROR, readPayloadErr.Error())
				return 
			}

			remoteFileSize, startOffset, chunkSize, desErr := cli.deserializePayload(buf[:payloadLength])
			if desErr != nil {
				conn.CloseWithError(common.INTERNAL_ERROR, desErr.Error())
				return
			}

			resizeErr := cli.resizeDstFile(int64(remoteFileSize))
			if resizeErr != nil {
				conn.CloseWithError(common.INTERNAL_ERROR, resizeErr.Error())
				return
			}
			
			log.Printf("startOffset: %d, chunkSize: %d, stream: %d, remoteFSize: %d", startOffset, chunkSize, s, remoteFileSize)

			_, seekErr := cli.dstFile.Seek(int64(startOffset), 0)
			if seekErr != nil { 
				conn.CloseWithError(common.INTERNAL_ERROR, seekErr.Error())
				return
			}

			totBytesWritten, copyErr := io.CopyN(cli.dstFile, stream, int64(chunkSize))
			if copyErr != nil && copyErr != io.EOF { 
				conn.CloseWithError(common.TRANSPORT_ERROR, copyErr.Error())
				return 
			}

			if totBytesWritten == int64(chunkSize) { log.Println("total bytes written same as chunk size:", totBytesWritten) }
		}(uint8(s))
	}

	multiplexStreamWG.Wait()

	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)

	log.Println("file transfer complete, connection can now close")
	log.Println("total elapsed time for file transfer:", elapsedTime)

	return &dstpath, nil
}

// openConnection
//	Open a connection to a http3 server running over quic.
//	The DialEarly function attempts to make a connection using 0-RTT.
func (cli *QuicClient) openConnection(opts *OpenConnectionOpts) (quic.Connection, error) {
	tlsConfig := &tls.Config{ InsecureSkipVerify: opts.Insecure, NextProtos: []string{ common.FTRANSFER_PROTO }}
	quicConfig := &quic.Config{ EnableDatagrams: true }

	udpAddr, getAddrErr := net.ResolveUDPAddr(common.NET_PROTOCOL, cli.address)
	if getAddrErr != nil { return nil, getAddrErr }

	udpConn, udpErr := net.ListenUDP(common.NET_PROTOCOL, &net.UDPAddr{ Port: cli.port })
	if udpErr != nil { return nil, udpErr }
	
	ctx, cancel := context.WithTimeout(context.Background(), HANDSHAKE_TIMEOUT * time.Second)
	defer cancel()

	tr := &quic.Transport{ Conn: udpConn }
	conn, connErr := tr.DialEarly(ctx, udpAddr, tlsConfig, quicConfig)
	if connErr != nil { return nil, connErr }
	
	log.Println("connection made with:", conn.RemoteAddr())
	return conn, nil
}

// deserializePayload
//	When an initial metadata payload is received from the remote server, it is deserialized from bytes.
//	Format:
//		bytes 0-7: uint64 representing the size of the file
//		bytes 8-15: uint64 representing the start offset in the file where the stream should begin processing
//		bytes 16-23: uint64 representing the size of the chunk being received by the stream
func (cli *QuicClient) deserializePayload(payload []byte) (uint64, uint64, uint64, error) {
	if len(payload) != common.SERVER_PAYLOAD_MAX_LENGTH { return 0, 0, 0, errors.New("payload incorrect length") }

	remoteFileSize, desFSizeErr := serialize.DeserializeUint64(payload[:8])
	if desFSizeErr != nil { return 0, 0, 0, desFSizeErr }

	startOffset, desOffsetErr := serialize.DeserializeUint64(payload[8:16])
	if desOffsetErr != nil { return 0, 0, 0, desOffsetErr }

	chunkSize, desChunkSizeErr := serialize.DeserializeUint64(payload[16:])
	if desChunkSizeErr != nil { return 0, 0, 0, desChunkSizeErr }

	return remoteFileSize, startOffset, chunkSize, nil
}

// resizeDstFile
//	When the streams receive the metadata, the file created needs to be resized to match the size of the remote file.
func (cli *QuicClient) resizeDstFile(remoteFileSize int64) error {
	fSize := int64(0)
	for fSize != remoteFileSize {
		stat, statErr := cli.dstFile.Stat()
		if statErr != nil { return statErr }

		fSize = stat.Size()
		if atomic.CompareAndSwapUint64(&cli.isResizing, 0, 1) {				
			truncateErr := cli.dstFile.Truncate(remoteFileSize)
			if truncateErr != nil { return truncateErr }
			break
		}

		runtime.Gosched()
	}

	return nil
}