package main

import  (
	"flag"
	"log"
	"os"
	"time"

	"github.com/sirgallo/quicfiletransfer/cli"
)


func main() {
	homeDir, getHomeDirErr := os.UserHomeDir()
	if getHomeDirErr != nil { log.Fatal(getHomeDirErr) }

	cwd, getCwdErr := os.Getwd()
	if getCwdErr != nil { log.Fatal(getCwdErr) }

	var host, filename, srcFolder, dstFolder string
	var port int
	var insecure bool

	flag.StringVar(&host, "host", "127.0.0.1", "the host where the remote file exists")
	flag.IntVar(&port, "port", 1234, "the port serving the file")
	flag.StringVar(&filename, "filename", "dummyfile", "the name of the file to transfer")
	flag.StringVar(&srcFolder, "srcFolder", homeDir, "the source folder for the file on the remote system")
	flag.StringVar(&dstFolder, "dstFolder", cwd, "the destination folder on the local system")
	flag.BoolVar(&insecure, "insecure", false, "whether or not to use an insecure connection")

	flag.Parse()

	cliOpts := &cli.QuicClientOpts{ Host: host, Port: port }
	client, newCliErr := cli.NewClient(cliOpts)
	if newCliErr != nil { log.Fatal(newCliErr) }
	
	startTime := time.Now()

	openOpts := &cli.OpenConnectionOpts{ Insecure: insecure }
	path, transferErr := client.StartFileTransferStream(openOpts, filename, srcFolder, dstFolder)
	if transferErr != nil { log.Fatal(transferErr) }
	
	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)
	
	log.Printf("new path: %s, elapsedTime: %v\n", *path, elapsedTime)
}