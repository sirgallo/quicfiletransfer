package mmap

// MMap is the byte array representation of the memory mapped file in memory.
type MMap []byte

const (
	// RDONLY: maps the memory read-only. Attempts to write to the MMap object will result in undefined behavior.
	RDONLY = 0
	// RDWR: maps the memory as read-write. Writes to the MMap object will update the underlying file.
	RDWR = 1 << iota
	// COPY: maps the memory as copy-on-write. Writes to the MMap object will affect memory, but the underlying file will remain unchanged.
	COPY
	// EXEC: marks the mapped memory as executable.
	EXEC
)

const (
	// If the ANON flag is set, the mapped memory will not be backed by a file.
	ANON = 1 << iota
)

// 1 << iota // this creates powers of 2