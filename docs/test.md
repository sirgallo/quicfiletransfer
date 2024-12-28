# test

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