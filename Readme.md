# QUIC File Transfer Service

## a cli + srv for transferring large files


## Design

The `File Transfer Service` utilizes the [quic-go](https://github.com/quic-go/quic-go) implementation of the [quic](https://en.wikipedia.org/wiki/QUIC) Protocol, built on top of `UDP`. Since `quic` allows for multiplexing of multiple streams on a single connection, the service takes advantage of this to attempt to speed up file transfers by processing and writing the file from the remote host (the server) to the destination (the client).

A client attempts to make a connection to a host running the server implementation. If the connection is successful, the client then opens a stream, or multiple streams, to the host, requesting a file, along with providing the current stream and the total number of streams opened. The server determines the number of chunks and size of each chunk to then stream back to the client.

To handle concurrent writes, the client opens a memory-mapped file the size of the remote file to transfer. A memory mapped file was chosen due to the write isolation each stream has and the inherent strength of memory mapped files to perform random access operations. Each stream is made aware of its start offset and the size of the chunk it is processing.

The client then processes the incoming stream data by buffering writes in memory and batching the writes together to limit overall `I/O` operations on the memory mapped file. Writes are asynchronous and run in a separate go routine. Each stream pipes its batched data into a channel to be processed. Multiple writers can be utilized, but be aware that this may have a performance impact as more writers are added, since each will be competing for system resouces. With more go routines, more context switching occurs.

[0RTT](https://http3-explained.haxx.se/en/quic/quic-0rtt) has also been enabled, which reduces the number of handshakes needed to make a secure connection.


## cmd

A server and client implementation are both provided. For usage and configuration, check [CMD](./cmd/Cmd.md).