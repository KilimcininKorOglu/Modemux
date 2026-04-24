FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

RUN go install github.com/a-h/templ/cmd/templ@latest

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN templ generate ./internal/web/templates/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /modemux ./cmd/modemux

FROM alpine:3.20

RUN apk add --no-cache \
    modemmanager \
    libqmi \
    usb-modeswitch \
    ca-certificates \
    tzdata

RUN adduser -D -H -s /sbin/nologin modemux && \
    addgroup modemux dialout

RUN mkdir -p /var/lib/modemux /etc/modemux && \
    chown modemux:modemux /var/lib/modemux

COPY --from=builder /modemux /usr/local/bin/modemux
COPY configs/config.example.yaml /etc/modemux/config.yaml

USER modemux
WORKDIR /var/lib/modemux

EXPOSE 8080 8901 8902 8903 1081 1082 1083

ENTRYPOINT ["modemux"]
CMD ["serve", "--config", "/etc/modemux/config.yaml"]
