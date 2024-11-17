FROM golang:1.23-alpine

WORKDIR /src

RUN apk update && apk upgrade && apk add --no-cache ca-certificates openssh git
RUN update-ca-certificates

ENV CGO_ENABLED=0

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
