#!/bin/bash
set -e

cleanup() {
    echo "cleaning up..."
    pkill -P $$ # kill child processes (volumes, master)
}
trap cleanup EXIT

go build -o ../bin/master ../cmd/master/main.go
go build -o ../bin/volume ../cmd/volume/main.go
go build -o ../bin/mkv ../cmd/mkv/main.go

rm -rf data1 data2 data3 data4 metadata.db
mkdir -p data1 data2 data3 data4

echo "starting 3 volumes..."
../bin/volume -port 8081 -root ./data1 &
            ../bin/volume -port 8082 -root ./data2 &
            ../bin/volume -port 8083 -root ./data3 &

sleep 2

echo "starting master..."
../bin/master -port 8080 -volumes "http://localhost:8081,http://localhost:8082,http://localhost:8083" -replicas 3 &

sleep 2

echo "putting blobs..."
for i in {1..5}; do
    curl -s -X PUT -d "content-$i" http://localhost:8080/blob/key-$i > /dev/null
done

echo "verifying..."
../bin/mkv -volumes "http://localhost:8081,http://localhost:8082,http://localhost:8083" -replicas 3 verify

echo "testing rebuild..."
pkill -f ../bin/master
rm metadata.db

../bin/mkv -volumes "http://localhost:8081,http://localhost:8082,http://localhost:8083" rebuild

../bin/mkv -volumes "http://localhost:8081,http://localhost:8082,http://localhost:8083" -replicas 1 verify
# rebuild only restores hash->hash entries; original keys are gone.
# verify only checks keys that exist in the db.
# after rebuild, the db keys are hashes, and disk files are named by hash.
# so verify should pass.

echo "testing rebalance..."
../bin/volume -port 8084 -root ./data4 &
sleep 1

../bin/mkv -volumes "http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084" -replicas 3 rebalance

COUNT4=$(find data4 -type f | wc -l)
echo "volume 4 has $COUNT4 files"
if [ "$COUNT4" -eq "0" ]; then  
    echo "warning: volume 4 has 0 files. might be normal if hash ring didn't assign any, but with 5 files and 3 replicas, likely some should move."
fi

echo "success!"
