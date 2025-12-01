#!/bin/bash
set -e

cleanup() {
    echo "cleaning up..."
    pkill -P $$ # kill child processes (volumes, master)
}
trap cleanup EXIT

go build -o ../bin/master ../cmd/master/main.go
go build -o ../bin/volume ../cmd/volume/main.go

rm -rf data1 data2 data3 metadata.db
mkdir -p data1 data2 data3

../bin/volume -port 8081 -root ./data1 &
../bin/volume -port 8082 -root ./data2 &
../bin/volume -port 8083 -root ./data3 &

sleep 2

../bin/master -port 8080 -volumes "http://localhost:8081,http://localhost:8082,http://localhost:8083" -replicas 3 &

sleep 2

echo "putting blob..."
curl -v -X PUT -d "replication-test" http://localhost:8080/blob/rep-key

echo "verifying replication..."
COUNT1=$(find data1 -type f | wc -l)
COUNT2=$(find data2 -type f | wc -l)
COUNT3=$(find data3 -type f | wc -l)

echo "volume 1: $COUNT1 files"
echo "volume 2: $COUNT2 files"
echo "volume 3: $COUNT3 files"

if [ "$COUNT1" -ne "1" ] || [ "$COUNT2" -ne "1" ] || [ "$COUNT3" -ne "1" ]; then
    echo "error: blob not replicated to all volumes"
    exit 1
fi

echo "success! blob found on all 3 volumes."

echo "testing GET..."
curl -v http://localhost:8080/blob/rep-key

echo "testing DELETE..."
curl -v -X DELETE http://localhost:8080/blob/rep-key

echo "verifying deletion..."
COUNT1=$(find data1 -type f | wc -l)
COUNT2=$(find data2 -type f | wc -l)
COUNT3=$(find data3 -type f | wc -l)

if [ "$COUNT1" -ne "0" ] || [ "$COUNT2" -ne "0" ] || [ "$COUNT3" -ne "0" ]; then
    echo "error: blob not deleted from all volumes"
    exit 1
fi

echo "success!"
