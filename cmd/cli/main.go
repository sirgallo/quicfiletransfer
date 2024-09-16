package main

import  (
	"flag"
	"log"
	"os"

	"github.com/sirgallo/quicfiletransfer/internal/cli"
)


const STREAMS = 1

func main() {
	var err error
	var host, filename, srcFolder, dstFolder string
	var port, cliport, streams int
	var insecure, checkMd5 bool

	homeDir, err := os.UserHomeDir()
	if err != nil { log.Fatal(err) }

	cwd, err := os.Getwd()
	if err != nil { log.Fatal(err) }

	flag.StringVar(&host, "host", "127.0.0.1", "the host where the remote file exists")
	flag.IntVar(&port, "port", 1234, "the port serving the file")
	flag.IntVar(&cliport, "cliPort", 1235, "the port the client establishes udp connection on")
	flag.StringVar(&filename, "filename", "dummyfile", "the name of the file to transfer")
	flag.StringVar(&srcFolder, "srcFolder", homeDir, "the source folder for the file on the remote system")
	flag.StringVar(&dstFolder, "dstFolder", cwd, "the destination folder on the local system")
	flag.IntVar(&streams, "streams", STREAMS, "determine the total number of streams to launch for a connection")
	flag.BoolVar(&insecure, "insecure", false, "whether or not to use an insecure connection")
	flag.BoolVar(&checkMd5, "checkMd5", false, "whether or not to additionally compute + check the md5checksum for the file")

	flag.Parse()

	cliOpts := &cli.QuicClientOpts{
		RemoteHost: host,
		RemotePort: port,
		ClientPort: cliport,
		Streams: uint8(streams),
		CheckMd5: checkMd5,
	}

	client, err := cli.NewClient(cliOpts)
	if err != nil { log.Fatal(err) }
	
	openOpts := &cli.OpenConnectionOpts{ Insecure: insecure }
	path, err := client.StartFileTransferStream(openOpts, filename, srcFolder, dstFolder)
	if err != nil { log.Fatal(err) }
	
	log.Printf("new path: %s\n", *path)
}