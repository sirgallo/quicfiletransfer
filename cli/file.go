package cli

import (
	"sync/atomic"
	"bytes"
	"errors"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/sirgallo/quicfiletransfer/md5"
)

// resizeDstFile
//	When the streams receive the metadata, the file created needs to be resized to match the size of the remote file.
func (cli *QuicClient) resizeDstFile(isResizing *uint64, remoteFileSize int64) error {
	var err error
	f, err := os.OpenFile(cli.dstFile, os.O_RDWR, 0666)
	if err != nil { return err }
	defer f.Close()

	fSize := int64(0)
	for fSize != remoteFileSize {
		stat, err := f.Stat()
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

	if !bytes.Equal(md5Bytes, sourceMd5) {
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
