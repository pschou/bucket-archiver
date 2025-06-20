#!/bin/bash
#
# This script takes the golang code and spits out a portable executable for doing bucket scanning.
#
set -e -x

version=$(date +%Y%m%d.%H%M)
rpm -q clamav-devel clamav golang || yum install clamav-devel clamav golang
LD_LIBRARY_PATH=$LD_LIBRARY_PATH:/usr/local/lib CGO_LDFLAGS="-L/usr/local/lib -lclamav" go build -ldflags "-X main.version=$version" -o s3archiver .

