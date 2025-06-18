#!/bin/bash
set -e -x

rpm -q clamav-devel clamav || yum install clamav-devel clamav
LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/lib CGO_LDFLAGS="-L/usr/local/lib -lclamav" go build -o s3archiver .

rsync -av /var/lib/clamav/ ./db/
#CGO_LDFLAGS="-L/usr/local/lib -lclamav" go run main.go
