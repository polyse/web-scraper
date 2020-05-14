FROM golang:1.14 AS builder
WORKDIR /go/src/github.com/polyse/web-scrapper

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN GOOS=linux CGO_ENABLED=0 go build -installsuffix cgo -o app github.com/polyse/web-scrapper/cmd/daemon

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=0 /go/src/github.com/polyse/web-scrapper .
ENTRYPOINT ["/app/app"]