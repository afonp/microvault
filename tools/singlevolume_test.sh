#!/bin/bash
set -e

go build -o bin/master cmd/master/main.go
go build -o bin/volume cmd/volume/main.go

cleanup() {
    echo "cleaning up..."
    [ -n "$VOL_PID" ] && kill $VOL_PID
    [ -n "$MASTER_PID" ] && kill $MASTER_PID
}
trap cleanup EXIT

rm -rf data metadata.db
mkdir -p data

echo "starting volume wrapper..."
./bin/volume -port 8081 -root ./data &
VOL_PID=$!

echo "starting master..."
./bin/master -port 8080 -volume http://localhost:8081 &
MASTER_PID=$!

sleep 2

echo "testing PUT..."
curl -v -X PUT -d "hello world" http://localhost:8080/blob/testkey

echo "checking file existence..."
FOUND=$(find data -type f | wc -l)
if [ "$FOUND" -ne "1" ]; then
    echo "error: file not found in data/"
    exit 1
fi
echo "file found."

echo "testing GET (metadata)..."
# this will redirect to http://localhost:8081/...
# since the wrapper doesn't serve GET, curl will get 405 or 404 if it follows redirect.
LOCATION=$(curl -v http://localhost:8080/blob/testkey 2>&1 | grep "< Location:" | awk '{print $3}')
echo "redirect location: $LOCATION"

if [[ "$LOCATION" != *"http://localhost:8081"* ]]; then
    echo "error: Bad redirect location"
    exit 1
fi

echo "testing delete..."
curl -v -X DELETE http://localhost:8080/blob/testkey

echo "checking file deletion..."
FOUND_AFTER=$(find data -type f | wc -l)
if [ "$FOUND_AFTER" -ne "0" ]; then
    echo "error: file still exists after delete"
    exit 1
fi
echo "file deleted."

echo "success!"

kill $VOL_PID
kill $MASTER_PID
