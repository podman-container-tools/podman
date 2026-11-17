#!/bin/bash

echo "zstd" > hellozstd-withzeros && \
truncate -c -s 10 hellozstd-withzeros && \
zstd -f --sparse hellozstd-withzeros -o sample-withzeros.zst && \
rm hellozstd-withzeros

echo "zstd" > hellozstd && \
zstd -f --sparse hellozstd -o sample.zst && \
rm hellozstd

echo "gzip" > hellogzip-withzeros && \
truncate -c -s 10 hellogzip-withzeros && \
gzip -f -c hellogzip-withzeros > sample-withzeros.gz && \
rm hellogzip-withzeros

echo "gzip" > hellogzip && \
gzip -f -c hellogzip > sample.gz && \
rm hellogzip

echo "bzip2" > hellobzip2-withzeros && \
truncate -c -s 10 hellobzip2-withzeros && \
bzip2 -f -c hellobzip2-withzeros > sample-withzeros.bz2 && \
rm hellobzip2-withzeros

echo "bzip2" > hellobzip2 && \
bzip2 -f -c hellobzip2 > sample.bz2 && \
rm hellobzip2

echo "uncompressed" > sample-withzeros.uncompressed && \
truncate -c -s 20 sample-withzeros.uncompressed

echo "uncompressed" > sample.uncompressed

echo "xz" > helloxz-withzeros && \
truncate -c -s 10 helloxz-withzeros && \
xz -f -z -c helloxz-withzeros > sample-withzeros.xz && \
rm helloxz-withzeros

echo "xz" > helloxz && \
xz -f -z -c helloxz > sample.xz && \
rm helloxz

echo "zip" > hellozip-withzeros && \
truncate -c -s 10 hellozip-withzeros && \
zip sample-withzeros.zip hellozip-withzeros && \
rm hellozip-withzeros

echo "zip" > hellozip && \
zip sample.zip hellozip && \
rm hellozip
