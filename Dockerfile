FROM golang:1.14 AS builder
WORKDIR /usr/src

ARG MODE=daemon
COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN echo build mode: ${MODE}
RUN GOOS=linux CGO_ENABLED=0 go build -installsuffix cgo -o app ./cmd/${MODE}


FROM alpine:latest
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /usr/src .
CMD ["/app/app"]