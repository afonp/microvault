# microvault

radically simple distributed blob store in ~1000 lines.

## features

- content-addressable storage (sha256)
- configurable replication
- consistent hashing for distribution
- nginx-powered reads (zero overhead)
- simple http api

## components

- `master/` - metadata index and coordination
- `volume/` - thin wrapper around nginx for storage
- `client/` - smart routing library
- `tools/` - rebuild, rebalance, verify, compact

## quickstart

```bash
# build everything
make build

# start master server
./bin/master -port 8080 -data ./data/index

# start volume servers
./bin/volume -port 9001 -data ./data/volume-1 -master http://localhost:8080
./bin/volume -port 9002 -data ./data/volume-2 -master http://localhost:8080

# store a blob
curl -X PUT http://localhost:8080/blob/myfile --data-binary @file.jpg

# retrieve a blob
curl http://localhost:8080/blob/myfile
```

## philosophy

simplicity over features. use boring, battle-tested components. the on-disk format should be trivial enough that you could rebuild the entire system from scratch in a weekend.
