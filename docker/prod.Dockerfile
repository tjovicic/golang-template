FROM golang:1.23-alpine AS builder

WORKDIR /src

RUN apk update && apk upgrade && apk add --no-cache ca-certificates git openssh
RUN update-ca-certificates

ENV CGO_ENABLED=0

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG TARGETDIR
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /bin/main ${TARGETDIR}

FROM scratch as bin
COPY --from=builder /bin/main /
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/main"]
