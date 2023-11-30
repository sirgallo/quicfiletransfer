# cmd

This is an example of quic client/server interaction for large files. 

To generate a random large file in the `cmd/srv` directory (this is our dummy service), run:
```bash
dd if=/dev/zero of=dummyfile bs=1G count=50
```

The above will generate a `50GB` file.

The server has these optional command line arguments:
```
-host=string -> the server host (default is 127.0.0.1)
-port=int -> the port the server host is serving from (default is 1234)
-org=string -> the organization for self signed certs (default is test)
-certPath=string -> the path to the valid tls cert file (default is "")
-keyPath=string -> the path to the valid tls private key file (default is "")
-enableTracer=bool -> enable the tracer, which will create a log file for all events (default is false)
```

By default, if neither `certPath` or `keyPath` are provided, a self signed cert is generated.

To run the server (in `./srv`):
```bash
go run main.go
```

The server also implements a tracer, so a log file is dopped in the `./srv`, which all events are written to.

**NOTE** Enabling the tracer will have a performance impact on the server.

The cli has these optional command line arguments:
```
-host=string -> the remote host (default is 127.0.0.1)
-port=string -> the port the remote host is serving from (default is 1234)
-filename=string -> the name of the file to be transfered (default is dummyfile)
-srcFolder=string -> the path to the file on the remote server(default is /<path-to-quic-file-transfer>/quicfiletransfer/cmd/srv)
-dstFolder=string -> the path to the destination folder on the local machine (default is /<path-to-quic-file-transfer>/quicfiletransfer/cmd/cli)
-insecure=bool -> determines if the client should verify the server's cert (default is false)
-streams=int -> the number of streams to open on the file transfer (default is 1)
-writers=int -> the number of writers to create to process the file (default is 1)
```

**NOTE** The insecure flag should only be used in development

In a separate terminal window (in `./cli`), run the following to test the `50GB` file transfer (locally needs to be `insecure` connection):
```bash
go run main.go -filename=dummyfile -srcFolder=/<path-to-quic-file-transfer>/quicfiletransfer/cmd/srv -dstFolder=/<path-to-quic-file-transfer>/quicfiletransfer/cmd/cli insecure=true
```


# test

`system`
```
Macbook Pro 2023
M2Pro, 16GB RAM, 512GB SSD
```

```
Test (5 runs, first iteration of quic file transfer):
  send a 0 filled 50GB file from the server, with the dummy file located in its directory, to the client and its directory.
  Measuse total time taken for the file to transfer.

run 1: 3m28.216701209s
run 2: 3m24.914857792s
run 3: 3m13.410814417s
run 4: 3m21.885894583s
run 5: 3m14.459741334s

avg => 250.21MB/s
```

This can be improved by further tweaking. The `MTU` size option in the quic server was excluded since quic utilizes a "best path" algorithm and dynamically finds the optimum `MTU` size based on network conditions. However, initial congestion could be tweaked further as well. `0RTT` is also already implemented as well.

Performance can also be improved by having the client send multiple streams asking for different chunks of the file. `QUIC` supports multiplexing multiple streams on a single connection, so this would also enable parallel processing of the file, potentially substantially increasing throughput.