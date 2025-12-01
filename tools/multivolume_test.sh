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

../bin/master -port 8080 -volumes "http://localhost:8081,http://localhost:8082,http://localhost:8083" &

sleep 2

echo "putting blobs..."
for i in {1..10}; do
    curl -s -X PUT -d "content-$i" http://localhost:8080/blob/key-$i > /dev/null
done

echo "verifying distribution..."
COUNT1=$(find data1 -type f | wc -l)
COUNT2=$(find data2 -type f | wc -l)
COUNT3=$(find data3 -type f | wc -l)

echo "volume 1: $COUNT1 files"
echo "volume 2: $COUNT2 files"
echo "volume 3: $COUNT3 files"

TOTAL=$((COUNT1 + COUNT2 + COUNT3))
if [ "$TOTAL" -ne "10" ]; then
    echo "error: expected 10 files, found $TOTAL"
    exit 1
fi

if [ "$COUNT1" -eq "0" ] && [ "$COUNT2" -eq "0" ] && [ "$COUNT3" -eq "0" ]; then
    echo "error: no files found anywhere"
    exit 1
fi

echo "reading blobs..."
for i in {1..10}; do
    HEADERS=$(curl -s -I http://localhost:8080/blob/key-$i)
    LOCATION=$(echo "$HEADERS" | grep -i "Location:" | awk '{print $2}' | tr -d '\r')
    
    if [ -z "$LOCATION" ]; then
        echo "error: no location for key-$i"
        echo "headers:"
        echo "$HEADERS"
        exit 1
    fi
    echo "key $i -> $LOCATION"
done

echo "success!"
