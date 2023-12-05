package cli

import (
	"bytes"
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
	"github.com/sirgallo/quicfiletransfer/common/md5"
	"github.com/sirgallo/quicfiletransfer/common/serialize"
)


//============================================= Client


// NewClient
//	Create a new quic file transfer client.
func NewClient(opts *QuicClientOpts) (*QuicClient, error) {
	remoteHostPort := net.JoinHostPort(opts.RemoteHost, strconv.Itoa(opts.RemotePort))
	log.Printf("remote server address: %s\n", remoteHostPort)

	return &QuicClient{ 
		remoteAddress: remoteHostPort,
		cliPort: opts.ClientPort,
		streams: opts.Streams,
		checkMd5: opts.CheckMd5,
	}, nil
}

// StartFileTransferStream
//	Invoke a file transfer operation.
//	The client provides the total number of streams to open.
//	Once each stream receives a metadata response from the server, the file is resized.
//	The streams for the client connection then receive and write the file chunks from the server to disk.
func (cli *QuicClient) StartFileTransferStream(connectOpts *OpenConnectionOpts, filename, src, dst string) (*string, error) {
	isResizing := uint64(0)
	totReadBytes := uint64(0)
	signalProgress := make(chan bool)

	srcPath := filepath.Join(src, filename)
	cli.dstFile = filepath.Join(dst, filename)
	
	f, createErr := os.Create(cli.dstFile)
	if createErr != nil { return nil, createErr }
	f.Close()

	conn, connErr := cli.openConnection(connectOpts)
	if connErr != nil { return nil, connErr }
	defer conn.CloseWithError(common.NO_ERROR, "closing")

	commStream, openCommStreamErr := conn.OpenStream()
	if openCommStreamErr != nil {
		conn.CloseWithError(common.CONNECTION_ERROR, openCommStreamErr.Error())
		return nil, openCommStreamErr
	}

	fileReq := func() []byte {
		tags := []byte{ cli.streams, serialize.SerializeBool(cli.checkMd5) }
		return append(tags, []byte(srcPath)...)
	}()

	_, fileReqErr := commStream.Write(fileReq)
	if fileReqErr != nil {
		conn.CloseWithError(common.TRANSPORT_ERROR, fileReqErr.Error())
		return nil, fileReqErr
	}

	buf := make([]byte, common.FILE_META_PAYLOAD_MAX_LENGTH)
	payloadLength, readPayloadErr := commStream.Read(buf)
	if readPayloadErr != nil {
		conn.CloseWithError(common.TRANSPORT_ERROR, readPayloadErr.Error())
		return nil, readPayloadErr
	}

	remoteFileSize, sourceMd5, desMetaErr := cli.deserializeMetaPayload(buf[:payloadLength])
	if desMetaErr != nil {
		conn.CloseWithError(common.INTERNAL_ERROR, desMetaErr.Error())
		return nil, desMetaErr
	}

	resizeErr := cli.resizeDstFile(&isResizing, int64(remoteFileSize))
	if resizeErr != nil {
		conn.CloseWithError(common.INTERNAL_ERROR, resizeErr.Error())
		return nil, resizeErr
	}

	streamStartTime := time.Now()

	var clientWG sync.WaitGroup

	clientWG.Add(1)
	go func() {
		defer clientWG.Done()
		var lastP float64
		for range signalProgress {
			currTotRead := atomic.LoadUint64(&totReadBytes)
			p := (float64(currTotRead) / float64(remoteFileSize)) * 100

			if p >= lastP + 5 || p >= 100 {
				currTime := time.Now()
				log.Printf("total bytes received: %d, percentage of total: %f, time elapsed: %v\n", currTotRead, p, currTime.Sub(streamStartTime))
			}
		}
	}()

	for range make([]uint8, cli.streams) {
		dataStream, openSendStreamErr := conn.AcceptUniStream(context.Background())
		if openSendStreamErr != nil { 
			conn.CloseWithError(common.CONNECTION_ERROR, openSendStreamErr.Error())
			return nil, openSendStreamErr
		}

		go func() {
			buf := make([]byte, common.CHUNK_META_PAYLOAD_MAX_LENGTH)
			payloadLength, readPayloadErr := dataStream.Read(buf)
			if readPayloadErr != nil { 
				conn.CloseWithError(common.TRANSPORT_ERROR, readPayloadErr.Error())
				return
			}
		
			startOffset, chunkSize, desErr := cli.deserializeChunkPayload(buf[:payloadLength])
			if desErr != nil {
				conn.CloseWithError(common.INTERNAL_ERROR, desErr.Error())
				return
			}
	
			log.Printf("startOffset: %d, chunkSize: %d\n", startOffset, chunkSize)
	
			f, openErr := os.OpenFile(cli.dstFile, os.O_RDWR, 0666)
			if openErr != nil { return }
			defer f.Close()
	
			_, seekErr := f.Seek(int64(startOffset), 0)
			if seekErr != nil { 
				conn.CloseWithError(common.INTERNAL_ERROR, seekErr.Error())
				return
			}
	
			writeBuffer := make([]byte, common.MAX_S_REC_WINDOW)
			totBytesOfChunkRead := 0

			for int(chunkSize) > totBytesOfChunkRead {
				nRead, readErr := io.ReadFull(dataStream, writeBuffer)
				if readErr != nil && readErr != io.EOF && readErr != io.ErrUnexpectedEOF {
					conn.CloseWithError(common.TRANSPORT_ERROR, readErr.Error())
					return
				}

				atomic.AddUint64(&totReadBytes, uint64(nRead))
				select {
					case signalProgress <- true:
					default:
				}

				nWritten, writeErr := f.Write(writeBuffer[:nRead])
				if writeErr != nil {
					conn.CloseWithError(common.INTERNAL_ERROR, writeErr.Error())
					return
				}

				totBytesOfChunkRead += nWritten
			}
		}()
	}

	clientWG.Wait()

	streamEndTime := time.Now()
	streamElapsedTime := streamEndTime.Sub(streamStartTime)

	log.Println("file transfer complete, connection can now close")
	log.Println("total elapsed time for file transfer", streamElapsedTime)
	log.Printf("transfer speed: %dMB/s\n", func() int {
		return (int(remoteFileSize) / 1024 * 1024) / int(streamElapsedTime.Seconds())
	}())

	if cli.checkMd5 { return cli.performMd5Check(sourceMd5) }
	return &cli.dstFile, nil
}

// openConnection
//	Open a connection to a http3 server running over quic.
//	The DialEarly function attempts to make a connection using 0-RTT.
func (cli *QuicClient) openConnection(opts *OpenConnectionOpts) (quic.Connection, error) {
	tlsConfig := &tls.Config{ InsecureSkipVerify: opts.Insecure, NextProtos: []string{ common.FTRANSFER_PROTO }}
	quicConfig := &quic.Config{
		InitialStreamReceiveWindow: common.INITIAL_S_REC_WINDOW,
		MaxStreamReceiveWindow: common.MAX_S_REC_WINDOW,
		EnableDatagrams: true,
	}

	udpAddr, getAddrErr := net.ResolveUDPAddr(common.NET_PROTOCOL, cli.remoteAddress)
	if getAddrErr != nil { return nil, getAddrErr }

	udpConn, udpErr := net.ListenUDP(common.NET_PROTOCOL, &net.UDPAddr{ Port: cli.cliPort })
	if udpErr != nil { return nil, udpErr }
	
	ctx, cancel := context.WithTimeout(context.Background(), common.DEFAULT_HANDSHAKE_TIME * time.Second)
	defer cancel()

	tr := &quic.Transport{ Conn: udpConn }
	conn, connErr := tr.DialEarly(ctx, udpAddr, tlsConfig, quicConfig)
	if connErr != nil { return nil, connErr }
	
	log.Println("connection made with:", conn.RemoteAddr())
	return conn, nil
}

// deserializePayload
//	Initial metadata payload with remote filesize and md5.
//	Format:
//		bytes 0-7: uint64 representing the size of the file
//		bytes 8-23: md5 in byte format
func (cli *QuicClient) deserializeMetaPayload(payload []byte) (uint64, []byte, error) {
	if len(payload) != common.FILE_META_PAYLOAD_MAX_LENGTH { return 0, nil, errors.New("payload incorrect length") }

	remoteFileSize, desFSizeErr := serialize.DeserializeUint64(payload[:8])
	if desFSizeErr != nil { return 0, nil, desFSizeErr }

	return remoteFileSize, payload[8:], nil
}

// deserializeChunkPayload
//	Metadata payload regarding chunk size and start offset in file.
//	Format:
//		bytes 0-7: uint64 representing the start offset in the file where the stream should begin processing
//		bytes 8-16: uint64 representing the size of the chunk being received by the stream
func (cli *QuicClient) deserializeChunkPayload(payload []byte) (uint64, uint64, error) {
	if len(payload) != common.CHUNK_META_PAYLOAD_MAX_LENGTH { return 0, 0, errors.New("payload incorrect length") }
	
	startOffset, desOffsetErr := serialize.DeserializeUint64(payload[:8])
	if desOffsetErr != nil { return 0, 0, desOffsetErr }

	chunkSize, desChunkSizeErr := serialize.DeserializeUint64(payload[8:])
	if desChunkSizeErr != nil { return 0, 0, desChunkSizeErr }

	return startOffset, chunkSize, nil
}

// resizeDstFile
//	When the streams receive the metadata, the file created needs to be resized to match the size of the remote file.
func (cli *QuicClient) resizeDstFile(isResizing *uint64, remoteFileSize int64) error {
	f, openErr := os.OpenFile(cli.dstFile, os.O_RDWR, 0666)
	if openErr != nil { return openErr }
	defer f.Close()

	fSize := int64(0)
	for fSize != remoteFileSize {
		stat, statErr := f.Stat()
		if statErr != nil { return statErr }

		fSize = stat.Size()
		if atomic.CompareAndSwapUint64(isResizing, 0, 1) {				
			truncateErr := f.Truncate(remoteFileSize)
			if truncateErr != nil { return truncateErr }
			break
		}

		runtime.Gosched()
	}

	return nil
}

// performMd5Check
//	Optionally perform and md5 check on the transferred file.
func (cli *QuicClient) performMd5Check(sourceMd5 []byte) (*string, error){
	md5StartTime := time.Now()
	log.Println("calculating md5 checksum")
	
	md5Bytes, md5Err := md5.CalculateMD5(cli.dstFile)
	if md5Err != nil { return nil, md5Err }

	md5EndTime := time.Now()
	md5ElapsedTime := md5EndTime.Sub(md5StartTime)

	log.Printf("calculated md5: %v, source md5: %v\n", md5Bytes, sourceMd5)
	log.Println("total elapsed time for md5 calculation:", md5ElapsedTime)

	if ! bytes.Equal(md5Bytes, sourceMd5) {
		remErr := os.Remove(cli.dstFile)
		if remErr != nil { return nil, remErr }
		return nil, errors.New("md5 checksums did not match")
	}

	md5File, createFileErr := os.Create(cli.dstFile + ".md5")
	if createFileErr != nil { return nil, createFileErr }
	defer md5File.Close()

	md5Hex, decodeErr := md5.DeserializeMD5ToHex(md5Bytes)
	if decodeErr != nil { return nil, decodeErr }

	_, md5WriteErr := md5File.Write([]byte(md5Hex))
	if md5WriteErr != nil { return nil, md5WriteErr }

	log.Println("md5 check passed, done")
	return &cli.dstFile, nil
}