FROM golang:1.26-alpine

RUN apk add --no-cache protobuf protoc

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

ENV PATH="$PATH:/go/bin"

WORKDIR /workspace
