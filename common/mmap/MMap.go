package mmap

import "errors"
import "os"
import "golang.org/x/sys/unix"


//============================================= MMap


// Map 
//	Memory maps an entire file.
func Map(file *os.File, prot, flags int) (MMap, error) {
	return mapRegion(file, -1, prot, flags, 0)
}

// Flush
//	Writes the byte slice from the mmap to disk.
func (mapped MMap) Flush() error {
	return unix.Msync(mapped, unix.MS_SYNC)
}

// Unmap 
//	Unmaps the byte slice from the memory mapped file.
func (mapped MMap) Unmap() error {
	return unix.Munmap(mapped)
}

// mapRegion 
//	Memory maps a region of a file.
func mapRegion(file *os.File, length int, prot, flags int, offset int64) (MMap, error) {
	if offset % int64(os.Getpagesize()) != 0 {
		return nil, errors.New("offset parameter must be a multiple of the system's page size")
	}

	var fileDescriptor uintptr
	if flags & ANON == 0 {
		fileDescriptor = uintptr(file.Fd())
		
		if length < 0 {
			fileStat, statErr := file.Stat()
			if statErr != nil { return nil, statErr }
			
			length = int(fileStat.Size())
		}
	} else {
		if length <= 0 { return nil, errors.New("anonymous mapping requires non-zero length") }
		fileDescriptor = ^uintptr(0)
	}

	return mmapHelper(length, uintptr(prot), uintptr(flags), fileDescriptor, offset)
}

// mmapHelper 
//	Utility function for mmap.
func mmapHelper(length int, inprot, inflags, fileDescriptor uintptr, offset int64) ([]byte, error) {
	flags := unix.MAP_SHARED
	prot := unix.PROT_READ
	
	switch {
		case inprot & COPY != 0:
			prot |= unix.PROT_WRITE
			flags = unix.MAP_PRIVATE
		case inprot & RDWR != 0:
			prot |= unix.PROT_WRITE
	}
	
	if inprot & EXEC != 0 { prot |= unix.PROT_EXEC }
	if inflags & ANON != 0 { flags |= unix.MAP_ANON }

	bytes, mmapErr := unix.Mmap(int(fileDescriptor), offset, length, prot, flags)
	if mmapErr != nil { return nil, mmapErr }
	
	return bytes, nil
}