# QUIC File Transfer Service

## a cli + srv for transferring large files


## Design

The `File Transfer Service` utilizes the [quic-go](https://github.com/quic-go/quic-go) implementation of the [quic](https://en.wikipedia.org/wiki/QUIC) Protocol, built on top of `UDP`. Since `quic` allows for multiplexing of streams on a single connection, the service takes advantage of this to attempt to speed up file transfers by processing and writing the file from the remote host (the server) to the destination (the client) concurrently.

A client attempts to make a connection to a host running the server implementation. If the connection is successful, the client then opens a stream, or multiple streams, to the host, requesting a file, along with providing the current stream and the total number of streams opened. The server determines the number of chunks and size of each chunk to then stream back to the client. Each stream is made aware of its start offset and the size of the chunk it is processing. 

[0RTT](https://http3-explained.haxx.se/en/quic/quic-0rtt) has also been enabled, which reduces the number of handshakes needed to make a secure connection.

A `MD5` checksum is calculated as well for the transferred file to verify that the content is the same as the source file. The server provides its own `MD5` for comparison once the file is written.


## cmd

A server and client implementation are both provided. For usage and configuration, check [CMD](./cmd/Cmd.md).