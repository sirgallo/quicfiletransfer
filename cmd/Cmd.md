# cmd

This is an example of quic client/server interaction for large files. 

To generate a random large file in the `cmd/srv` directory (this is our dummy service), run:
```bash
dd if=/dev/urandom of=dummyfile bs=1G count=50
```

The above will generate a `50GB` file, with random values.

Next generate a md5hash from the file. This will be used to ensure the transferred file's integrity.

The following is for macOS:
```bash
md5 -r dummyfile | sed 's/ dummyfile//' > dummyfile.md5
```

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

The server also implements a tracer, so a log file is dopped in `./srv`, where all events will be written to.

**NOTE** Enabling the tracer will have a performance impact on the server.

The cli has these optional command line arguments:
```
-host=string -> the remote host (default is 127.0.0.1)
-port=int -> the port the remote host is serving from (default is 1234)
-cliPort=int -> the port the client establishes udp connection on (default is 1235)
-filename=string -> the name of the file to be transfered (default is dummyfile)
-srcFolder=string -> the path to the file on the remote server(default is /<path-to-quic-file-transfer>/quicfiletransfer/cmd/srv)
-dstFolder=string -> the path to the destination folder on the local machine (default is /<path-to-quic-file-transfer>/quicfiletransfer/cmd/cli)
-insecure=bool -> determines if the client should verify the server's cert (default is false)
-streams=int -> the number of streams to open on the file transfer (default is 1)
-checkMd5=bool -> perform additional md5 check against remote md5 file (default is false)
```

**NOTE** The insecure flag should only be used in development

In a separate terminal window (in `./cli`), run the following to test the `50GB` file transfer (local needs to be `insecure` connection):
```bash
go run main.go -filename=dummyfile -srcFolder=/<path-to-quic-file-transfer>/quicfiletransfer/cmd/srv -dstFolder=/<path-to-quic-file-transfer>/quicfiletransfer/cmd/cli -insecure=true -checkMd5=true
```


# docker

`build images`
```bash
docker build -f Dockerfile.build -t quicdependencies .
docker build -f Dockerfile.srv -t srv .
docker build -f Dockerfile.srv -t cli .
```

`run`
```bash
docker run --net=host -p 1235:1235 -v /<directory-on-host>:/home/quiccli/files cli \
  -filename=dummyfile \
  -srcFolder=/home/quicsrv/files/Projects/quicfiletransfer/cmd/srv \
  -dstFolder=/home/quiccli/files/Projects/quicfiletransfer/cmd/cli \
  -insecure=true \
  -checkMd5=true


docker run --net=host -p 1234:1234 -v /<directory-on-host>:/home/quicsrv/files srv \
  -port=1234
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


**NOTE**

Running tests on localhost is most likely not indictive of real world performance, and may be worse than the expected throughput for a local file transfer. This is partly due to the fact that `quic-go` is implemented in the application space and not at the kernel level, so the data transfer has to move through multiple os levels. A real world test over both short and long distance would have to be conducted to confirm this.