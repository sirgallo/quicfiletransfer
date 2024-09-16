package cli

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"io/fs"
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

	"github.com/sirgallo/quicfiletransfer/internal/common"
	"github.com/sirgallo/quicfiletransfer/internal/common/md5"
	"github.com/sirgallo/quicfiletransfer/internal/common/serialize"
)


//============================================= Client

// NewClient
//	Create a new quic file transfer client.
func NewClient(opts *QuicClientOpts) (*QuicClient, error) {
	remoteAddress := net.JoinHostPort(opts.RemoteHost, strconv.Itoa(opts.RemotePort))
	log.Printf("remote server address: %s\n", remoteAddress)

	return &QuicClient{ 
		remoteAddress: remoteAddress,
		cliPort: opts.CliPort,
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
	var err error

	isResizing := uint64(0)
	totReadBytes := uint64(0)
	signalProgress := make(chan bool)

	srcPath := filepath.Join(src, filename)
	cli.dstFile = filepath.Join(dst, filename)
	
	f, err := os.Create(cli.dstFile)
	if err != nil { return nil, err }
	f.Close()

	conn, err := cli.openConnection(connectOpts)
	if err != nil { return nil, err }
	defer conn.CloseWithError(common.NO_ERROR, "closing")

	commStream, err := conn.OpenStream()
	if err != nil {
		conn.CloseWithError(common.CONNECTION_ERROR, err.Error())
		return nil, err
	}

	fileReq := func() []byte {
		tags := []byte{ cli.streams, serialize.SerializeBool(cli.checkMd5) }
		return append(tags, []byte(srcPath)...)
	}()

	_, err = commStream.Write(fileReq)
	if err != nil {
		conn.CloseWithError(common.TRANSPORT_ERROR, err.Error())
		return nil, err
	}

	buf := make([]byte, common.FILE_META_PAYLOAD_MAX_LENGTH)
	payloadLength, err := commStream.Read(buf)
	if err != nil {
		conn.CloseWithError(common.TRANSPORT_ERROR, err.Error())
		return nil, err
	}

	remoteFileSize, sourceMd5, err := cli.deserializeMetaPayload(buf[:payloadLength])
	if err != nil {
		conn.CloseWithError(common.INTERNAL_ERROR, err.Error())
		return nil, err
	}

	err = cli.resizeDstFile(&isResizing, int64(remoteFileSize))
	if err != nil {
		conn.CloseWithError(common.INTERNAL_ERROR, err.Error())
		return nil, err
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

	var dataStream quic.ReceiveStream
	for range make([]uint8, cli.streams) {
		dataStream, err = conn.AcceptUniStream(context.Background())
		if err != nil { 
			conn.CloseWithError(common.CONNECTION_ERROR, err.Error())
			return nil, err
		}

		go func() {
			buf := make([]byte, common.CHUNK_META_PAYLOAD_MAX_LENGTH)
			payloadLength, readPayloadErr := dataStream.Read(buf)
			if readPayloadErr != nil { 
				conn.CloseWithError(common.TRANSPORT_ERROR, readPayloadErr.Error())
				return
			}
		
			startOffset, chunkSize, err := cli.deserializeChunkPayload(buf[:payloadLength])
			if err != nil {
				conn.CloseWithError(common.INTERNAL_ERROR, err.Error())
				return
			}
	
			log.Printf("start offset: %d, chunkSize: %d\n", startOffset, chunkSize)
	
			f, err := os.OpenFile(cli.dstFile, os.O_RDWR, 0666)
			if err != nil { return }
			defer f.Close()
	
			_, err = f.Seek(int64(startOffset), 0)
			if err != nil { 
				conn.CloseWithError(common.INTERNAL_ERROR, err.Error())
				return
			}
	
			writeBuffer := make([]byte, common.MAX_S_REC_WINDOW)
			totBytesOfChunkRead := 0

			var nRead int
			for int(chunkSize) > totBytesOfChunkRead {
				nRead, err = io.ReadFull(dataStream, writeBuffer)
				if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
					conn.CloseWithError(common.TRANSPORT_ERROR, err.Error())
					return
				}

				atomic.AddUint64(&totReadBytes, uint64(nRead))
				select {
					case signalProgress <- true:
					default:
				}

				nWritten, err := f.Write(writeBuffer[:nRead])
				if err != nil {
					conn.CloseWithError(common.INTERNAL_ERROR, err.Error())
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
	var err error

	tlsConfig := &tls.Config{ InsecureSkipVerify: opts.Insecure, NextProtos: []string{ common.FTRANSFER_PROTO }}
	quicConfig := &quic.Config{
		InitialStreamReceiveWindow: common.INITIAL_S_REC_WINDOW,
		MaxStreamReceiveWindow: common.MAX_S_REC_WINDOW,
		EnableDatagrams: true,
	}

	udpAddr, err := net.ResolveUDPAddr(common.NET_PROTOCOL, cli.remoteAddress)
	if err != nil { return nil, err }

	udpConn, err := net.ListenUDP(common.NET_PROTOCOL, &net.UDPAddr{ Port: cli.cliPort })
	if err != nil { return nil, err }
	
	ctx, cancel := context.WithTimeout(context.Background(), common.DEFAULT_HANDSHAKE_TIME * time.Second)
	defer cancel()

	tr := &quic.Transport{ Conn: udpConn }
	conn, err := tr.DialEarly(ctx, udpAddr, tlsConfig, quicConfig)
	if err != nil { return nil, err }
	
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

	if len(payload) != common.CHUNK_META_PAYLOAD_MAX_LENGTH { return 0, 0, errors.New("payload incorrect length") }
	
	startOffset, err := serialize.DeserializeUint64(payload[:8])
	if err != nil { return 0, 0, err }

	chunkSize, err := serialize.DeserializeUint64(payload[8:])
	if err != nil { return 0, 0, err }

	return startOffset, chunkSize, nil
}

// resizeDstFile
//	When the streams receive the metadata, the file created needs to be resized to match the size of the remote file.
func (cli *QuicClient) resizeDstFile(isResizing *uint64, remoteFileSize int64) error {
	var err error

	f, err := os.OpenFile(cli.dstFile, os.O_RDWR, 0666)
	if err != nil { return err }
	defer f.Close()

	fSize := int64(0)
	var stat fs.FileInfo

	for fSize != remoteFileSize {
		stat, err = f.Stat()
		if err != nil { return err }

		fSize = stat.Size()
		if atomic.CompareAndSwapUint64(isResizing, 0, 1) {				
			err = f.Truncate(remoteFileSize)
			if err != nil { return err }
			break
		}

		runtime.Gosched()
	}

	return nil
}

// performMd5Check
//	Optionally perform and md5 check on the transferred file.
func (cli *QuicClient) performMd5Check(sourceMd5 []byte) (*string, error){
	var err error

	md5StartTime := time.Now()
	log.Println("calculating md5 checksum")
	
	md5Bytes, err := md5.CalculateMD5(cli.dstFile)
	if err != nil { return nil, err }

	md5EndTime := time.Now()
	md5ElapsedTime := md5EndTime.Sub(md5StartTime)

	log.Printf("calculated md5: %v, source md5: %v\n", md5Bytes, sourceMd5)
	log.Println("total elapsed time for md5 calculation:", md5ElapsedTime)

	if ! bytes.Equal(md5Bytes, sourceMd5) {
		err = os.Remove(cli.dstFile)
		if err != nil { return nil, err }
		return nil, errors.New("md5 checksums did not match")
	}

	md5File, err := os.Create(cli.dstFile + ".md5")
	if err != nil { return nil, err }
	defer md5File.Close()

	md5Hex, err := md5.DeserializeMD5ToHex(md5Bytes)
	if err != nil { return nil, err }

	_, err = md5File.Write([]byte(md5Hex))
	if err != nil { return nil, err }

	log.Println("md5 check passed, done")
	return &cli.dstFile, nil
}