package cli

import (
	"context"
	"crypto/tls"
	"log"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
	
	"github.com/quic-go/quic-go"
	"github.com/sirgallo/quicfiletransfer/common"
)


func NewClient(opts *QuicClientOpts) (*QuicClient, error) {
	remoteHostPort := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))
	log.Printf("remote server address: %s", remoteHostPort)

	return &QuicClient{ address: remoteHostPort }, nil
}

func (cli *QuicClient) StartFileTransferStream(connectOpts *OpenConnectionOpts, filename, src, dst string) (*string, error){
	conn, connErr := cli.openConnection(connectOpts)
	if connErr != nil { return nil, connErr }

	defer conn.CloseWithError(common.NO_ERROR, "closing")

	stream, openErr := conn.OpenStream()
	if openErr != nil { 
		conn.CloseWithError(common.CONNECTION_ERROR, connErr.Error())
		return nil, openErr 
	}

	log.Println("opened file transfer stream")

	srcPath := filepath.Join(src, filename)
	_, writeErr := stream.Write([]byte(srcPath))
	if writeErr != nil { 
		conn.CloseWithError(common.TRANSPORT_ERROR, writeErr.Error())
		return nil, writeErr 
	}

	dstpath := filepath.Join(dst, filename)
	file, openErr := os.Create(dstpath)
	if openErr != nil { 
		conn.CloseWithError(common.INTERNAL_ERROR, openErr.Error())
		return nil, openErr 
	}

	log.Printf("srcPath from remote: %s, dstPath for local: %s", srcPath, dstpath)

	defer file.Close()

	_, copyErr := io.Copy(file, stream)
	if copyErr != nil && copyErr != io.EOF { 
		conn.CloseWithError(common.TRANSPORT_ERROR, copyErr.Error())
		return nil, copyErr 
	}

	log.Println("file transfer complete, connection can now close")
	return &dstpath, nil
}

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
	conn, connErr := tr.Dial(ctx, udpAddr, tlsConfig, quicConfig)
	if connErr != nil { return nil, connErr }
	
	log.Println("connection made with:", conn.RemoteAddr())
	return conn, nil
}