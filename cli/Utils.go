package cli

import (
	"github.com/sirgallo/quicfiletransfer/common/mmap"
)


//============================================= Client Utils


// handleFlush
//	This is "optimistic" flushing. 
//	A separate go routine is spawned and signalled to flush changes to the mmap to disk.
func (cli *QuicClient) handleFlush() {
	for range cli.signalFlushChan { cli.dstFile.Sync() }
}

func (cli *QuicClient) initializeFile(remoteFileSize int64) error {
	truncateErr := cli.dstFile.Truncate(remoteFileSize)
	if truncateErr != nil { return truncateErr }

	mmapErr := cli.mMap()
	if mmapErr != nil { return mmapErr }
	return nil
}

// mmap
//	Helper to memory map the mariInst File in to buffer.
func (cli *QuicClient) mMap() error {
	mMap, mmapErr := mmap.Map(cli.dstFile, mmap.RDWR, 0)
	if mmapErr != nil { return mmapErr }

	cli.data.Store(mMap)
	return nil
}

// munmap
//	Unmaps the memory map from RAM.
func (cli *QuicClient) munmap() error {
	flushErr := cli.dstFile.Sync()
	if flushErr != nil { return flushErr }

	mMap := cli.data.Load().(mmap.MMap)
	unmapErr := mMap.Unmap()
	if unmapErr != nil { return unmapErr }

	cli.data.Store(mmap.MMap{})
	return nil
}

// signalFlush
//	Called by all writes to "optimistically" handle flushing changes to the mmap to disk.
func (cli *QuicClient) signalFlush() {
	select {
		case cli.signalFlushChan <- true:
		default:
	}
}

// resetWC
//	Reset a write chunk object to be placed back in the pool.
func (wc *WriteChunk) resetWC() {
	wc.offset = 0
	wc.data = nil
}