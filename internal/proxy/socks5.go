package proxy

import (
	"context"
	"log/slog"
	"net"
	"time"

	"github.com/things-go/go-socks5"
)

type SOCKS5Proxy struct {
	listenAddr string
	sourceIP   string
	username   string
	password   string
	server     *socks5.Server
	listener   net.Listener
}

func NewSOCKS5Proxy(listenAddr, sourceIP, username, password string) *SOCKS5Proxy {
	p := &SOCKS5Proxy{
		listenAddr: listenAddr,
		sourceIP:   sourceIP,
		username:   username,
		password:   password,
	}

	opts := []socks5.Option{
		socks5.WithDial(p.dialFunc),
	}

	if username != "" {
		creds := socks5.StaticCredentials{username: password}
		opts = append(opts, socks5.WithAuthMethods([]socks5.Authenticator{
			socks5.UserPassAuthenticator{Credentials: creds},
		}))
	}

	p.server = socks5.NewServer(opts...)

	return p
}

func (p *SOCKS5Proxy) Start() error {
	slog.Info("SOCKS5 proxy starting", "addr", p.listenAddr, "sourceIP", p.sourceIP)

	var err error
	p.listener, err = net.Listen("tcp", p.listenAddr)
	if err != nil {
		return err
	}

	return p.server.Serve(p.listener)
}

func (p *SOCKS5Proxy) Close() error {
	if p.listener != nil {
		return p.listener.Close()
	}
	return nil
}

func (p *SOCKS5Proxy) dialFunc(_ context.Context, network string, addr string) (net.Conn, error) {
	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}

	if p.sourceIP != "" {
		switch network {
		case "tcp", "tcp4", "tcp6":
			dialer.LocalAddr = &net.TCPAddr{IP: net.ParseIP(p.sourceIP)}
		case "udp", "udp4", "udp6":
			dialer.LocalAddr = &net.UDPAddr{IP: net.ParseIP(p.sourceIP)}
		}
	}

	return dialer.Dial(network, addr)
}
