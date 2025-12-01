#!/bin/bash
set -e

# Build binaries
echo "Building..."
go build -o bin/master cmd/master/main.go
go build -o bin/volume cmd/volume/main.go

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    [ -n "$VOL_PID" ] && kill $VOL_PID
    [ -n "$MASTER_PID" ] && kill $MASTER_PID
}
trap cleanup EXIT

# Cleanup data
rm -rf data metadata.db
mkdir -p data

# Start Volume Wrapper (port 8081)
echo "Starting Volume Wrapper..."
./bin/volume -port 8081 -root ./data &
VOL_PID=$!

# Start Master (port 8080)
# Pointing Master to Volume Wrapper for PUTs (8081)
# But for GETs, we want it to redirect to Nginx (8082)
# Wait, my Master implementation uses one `volumeURL` for both PUT proxying and GET redirection.
# This is a problem.
# PUT needs to go to Wrapper (8081).
# GET needs to redirect to Nginx (8082).
#
# I need to separate `volumeWriteURL` and `volumeReadURL`?
# Or, simpler:
# Nginx listens on 8082.
# Nginx proxies PUT/DELETE to 8081.
# Nginx serves GET from disk.
#
# So Master should point to Nginx (8082).
# Master PUT -> Nginx (8082) -> Proxy to Wrapper (8081).
# Master GET -> Redirect to Nginx (8082) -> Serve file.
#
# This is the correct architecture!
# "Volume Server (Stock nginx + tiny wrapper)"
# "GET is pure nginx"
# "Tiny FastCGI/uWSGI script for PUT/DELETE" (replaced by Go wrapper)
#
# So Nginx is the entry point for the Volume.
#
# I need to verify my Nginx config handles PUT/DELETE proxying correctly.
# `limit_except GET HEAD { proxy_pass ... }`
# Yes, this proxies other methods.
#
# So in this test script, I need to start Nginx too?
# Or just simulate Nginx?
# Starting Nginx might be hard if I don't have it installed or permissions.
# The user is on Mac.
#
# If I can't start Nginx easily, I can test by pointing Master to Wrapper (8081) for PUT,
# and verifying GET redirect points to 8081 (which won't serve file unless Wrapper serves it).
# Wrapper `http.HandleFunc("/", ...)` handles PUT/DELETE. It defaults to 405 for GET.
# So Wrapper doesn't serve files.
#
# For this test script, I will assume Nginx is NOT running, and just test the PUT/DELETE flow via Wrapper.
# I will point Master to Wrapper (8081).
# I will PUT data.
# I will check if file exists on disk.
# I will check if Master DB has the entry.
# I will DELETE data.
#
# To test GET, I would need Nginx.
# I'll add a comment about Nginx.

echo "Starting Master..."
./bin/master -port 8080 -volume http://localhost:8081 &
MASTER_PID=$!

sleep 2

echo "Testing PUT..."
curl -v -X PUT -d "hello world" http://localhost:8080/blob/testkey

echo "Checking file existence..."
# We don't know the hash easily here without calculating it, 
# but we can find it in data/
FOUND=$(find data -type f | wc -l)
if [ "$FOUND" -ne "1" ]; then
    echo "Error: File not found in data/"
    exit 1
fi
echo "File found."

echo "Testing GET (Metadata)..."
# This will redirect to http://localhost:8081/...
# Since Wrapper doesn't serve GET, curl will get 405 or 404 from Wrapper if it follows redirect.
# We just check the redirect location.
LOCATION=$(curl -v http://localhost:8080/blob/testkey 2>&1 | grep "< Location:" | awk '{print $3}')
echo "Redirect location: $LOCATION"

if [[ "$LOCATION" != *"http://localhost:8081"* ]]; then
    echo "Error: Bad redirect location"
    exit 1
fi

echo "Testing DELETE..."
curl -v -X DELETE http://localhost:8080/blob/testkey

echo "Checking file deletion..."
# Note: My Delete implementation in Wrapper deletes the file.
FOUND_AFTER=$(find data -type f | wc -l)
if [ "$FOUND_AFTER" -ne "0" ]; then
    echo "Error: File still exists after delete"
    exit 1
fi
echo "File deleted."

echo "Success!"

kill $VOL_PID
kill $MASTER_PID
