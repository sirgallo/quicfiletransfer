# cmd

This is an example of quic client/server interaction for large files. 

To generate a random large file in the `cmd/srv` directory (this is our dummy service), run:
```bash
dd if=/dev/urandom of=dummyfile bs=1G count=10
```

The above will generate a `10GB` file, with random values.

Next generate a md5hash from the file. This will be used to ensure the transferred file's integrity.

`macOS`:
```bash
md5 -r dummyfile | sed 's/ dummyfile//' > dummyfile.md5
```

`linux`:
```bash
md5sum dummyfile | awk '{print $1}' > dummyfile.md5
```

The server has these optional command line arguments:
```
-host=string -> the server host (default is 0.0.0.0)
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

`build server`
```bash
docker build -f Dockerfile.build -t quicdependencies .
docker build -f Dockerfile.srv -t srv .
```

`build client`
```bash
docker build -f Dockerfile.build -t quicdependencies .
docker build -f Dockerfile.cli -t cli .
```

`run client`
```bash
docker run --net=host \
  --cpus=<n-num-cpus> \
  --memory=<n-ram>g \
  -p 1235:1235 \
  -v /<directory-on-host-to-write-file-to>:/home/quiccli/files cli \
  -host=<remote-host-ip> \
  -port=<remote-host-port> \
  -filename=dummyfile \
  -srcFolder=/home/quicsrv/files \
  -dstFolder=/home/quiccli/files \
  -insecure=true \
  -checkMd5=true \
  -streams=<n-streams>
```

`run server`
```bash
docker run --net=host \
  --cpus=<n-num-cpus> \
  --memory=<n-ram>g \
  -p 1234:1234 \
  -v /<directory-on-host>:/home/quicsrv/files srv
```


# tests

## remote

`system - both server and client`
```
48 cores, 32GB RAM, 11T SSD
```

```
Test (3 runs, 2 streams):

rsync:
  run 1: 3m37s - 46.08 MB/s
  run 2: 3m40s - 45.45 MB/s
  run 3: 3m34s - 46.54 MB/s

quic:
  run 1: 1m17s - 129.87 MB/s
  run 2: 1m13s - 136.99 MB/s
  run 3: 1m14s - 135.14 MB/s

rsync avg => 46.02 MB/s
quic avg => 134 MB/s

quic file transfer over 2.91x rsync for 10GB file
```


# Note

**on linux, the udp receive buffer size may need to be increased for better performance**

```bash
sysctl -w net.core.rmem_max=2500000
sysctl -w net.core.wmem_max=2500000
```