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
-port=string -> the port the server host is serving from (default is 1234)
-org=string -> the organization for self signed certs (default is test)
-certPath=string -> the path to the valid tls cert file (default is "")
-keyPath=string (default is "")
```

By default, if neither `certPath` or `keyPath` are provided, a self signed cert is generated.

To run the server (in `./srv`):
```bash
go run main.go
```

The server also implements a tracer, so a log file is dopped in the `./srv`, which all events are written to.

The cli has these optional command line arguments:
```
-host=string -> the remote host (default is 127.0.0.1)
-port=string -> the port the remote host is serving from (default is 1234)
-filename=<the-file-to-transfer> (default is dummyfile)
-srcFolder=<the-source-folder-on-remote> (default is /<path-to-quic-file-transfer>/quicfiletransfer/cmd/srv)
-dstFolder=<the-destination-folder-on-local> (default is /<path-to-quic-file-transfer>/quicfiletransfer/cmd/cli)
-insecure=<stops-the-client-> (default is false)
```

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
  Measuse total time taken for the file to transfer

run 1: 3m28.216701209s
run 2: 3m24.914857792s
run 3: 3m13.410814417s
run 4: 3m21.885894583s
run 5: 3m14.459741334s

avg => 250.21MB/s
```

This can be improved by further tweaking. The MTU size option in the quic server was excluded since quic utilizes a "best path" algorithm and dynamically finds the optimum MTU size based on network conditions. However, initial congestion could be tweaked further as well.