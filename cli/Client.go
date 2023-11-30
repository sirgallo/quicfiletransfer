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
	"github.com/sirgallo/quicfiletransfer/common/mmap"
	"github.com/sirgallo/quicfiletransfer/common/serialize"
	"github.com/sirgallo/quicfiletransfer/pool"
)


//============================================= Client


func NewClient(opts *QuicClientOpts) (*QuicClient, error) {
	remoteHostPort := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	log.Printf("remote server address: %s", remoteHostPort)

	data := atomic.Value{}
	data.Store(mmap.MMap{})

	writePool := pool.NewBufferPool(uint64(WRITE_SIZE))
	wcPool := NewWriteChunkPool()

	cli := &QuicClient{ 
		address: remoteHostPort,
		data: data,
		streams: opts.Streams,
		writers: opts.Writers,
		copyMu: &sync.Mutex{},
		signalFlushChan: make(chan bool),
		writeChunkChan: make(chan *WriteChunk, 10),
		writePool: writePool,
		wcPool: wcPool,
	}

	go cli.handleFlush()
	for range make([]uint8, cli.writers) { go cli.handleWrite() }

	return cli, nil
}

// StartFileTransferStream
//	Invoke a file transfer operation.
//	The client provides the total number of streams to open.
//	Once each stream receives a metadata response from the server, the file is resized and memory mapped.
//	The streams for the client connection then receive and write the file chunks from the server to disk.
//	Stream data is first buffered and batched in memory before being written to the mem map to avoid excessive I/O operations.
//	Writes are flushed "optimistically", so each write attempts to flush to disk but if a current flush op is happening the flush is skipped.
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

			dataLen := uint64(0)
			offset := startOffset
			readBuffer := make([]byte, STREAM_BUFFER_SIZE)
			
			writeBuffer := cli.writePool.GetBuffer()

			var writeMmapWG sync.WaitGroup

			for {
				currLen, readErr := stream.Read(readBuffer)
				if readErr == io.EOF { break }
				if readErr != nil { 
					conn.CloseWithError(common.INTERNAL_ERROR, readErr.Error())
					return
				}

				if dataLen + uint64(currLen) > uint64(WRITE_SIZE) {
					wc := cli.wcPool.GetWriteChunk()
					wc.offset = offset
					wc.data = writeBuffer[:dataLen]

					cli.writeChunkChan <- wc
					
					cli.writePool.PutBuffer(writeBuffer)
					writeBuffer = cli.writePool.GetBuffer()
					
					offset = offset + dataLen
					dataLen = uint64(0)
				} else {
					copy(writeBuffer[dataLen:dataLen + uint64(currLen)], readBuffer[:currLen])
					dataLen += uint64(currLen)
				}
			}

			writeMmapWG.Wait()
		}(uint8(s))
	}

	multiplexStreamWG.Wait()

	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)

	log.Println("file transfer complete, connection can now close")
	log.Println("total elapsed time for file transfer:", elapsedTime)

	mUnmapErr := cli.munmap()
	if mUnmapErr != nil {
		conn.CloseWithError(common.INTERNAL_ERROR, mUnmapErr.Error()) 
		return nil, mUnmapErr 
	}

	return &dstpath, nil
}

// openConnection
//	Open a connection to a http3 server running over quic.
//	The DialEarly function attempts to make a connection using 0-RTT
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
			initFileErr := cli.initializeFile(remoteFileSize)
			if initFileErr != nil { return initFileErr }
			break
		}

		runtime.Gosched()
	}

	return nil
}

// handleWrite
//	Run in a separate go routine.
//	Utilizes "go routine pooling", which limits the total number of go routines created for async writes
func (cli *QuicClient) handleWrite() error {
	for wc := range cli.writeChunkChan {
		mMap := cli.data.Load().(mmap.MMap)
		copy(mMap[wc.offset:], wc.data)
		cli.signalFlush()

		cli.wcPool.PutWriteChunk(wc)
	}

	return nil
}