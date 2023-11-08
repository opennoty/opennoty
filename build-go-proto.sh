#!/bin/sh

mkdir -p ./server/api/peer_proto/ ./api/api_proto/

protoc --proto_path=./proto/ \
  --go_out=./server/api/peer_proto \
  --go_opt=paths=source_relative \
  peer.proto

protoc --proto_path=./proto/ \
  --go_out=./api/api_proto \
  --go_opt=paths=source_relative \
  api.proto
