package proxy

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

type HTTPProxy struct {
	listenAddr string
	sourceIP   string
	username   string
	password   string
	server     *http.Server
	dialer     *net.Dialer
}

func NewHTTPProxy(listenAddr, sourceIP, username, password string) *HTTPProxy {
	p := &HTTPProxy{
		listenAddr: listenAddr,
		sourceIP:   sourceIP,
		username:   username,
		password:   password,
	}

	if sourceIP != "" {
		p.dialer = &net.Dialer{
			LocalAddr: &net.TCPAddr{IP: net.ParseIP(sourceIP)},
			Timeout:   30 * time.Second,
		}
	} else {
		p.dialer = &net.Dialer{Timeout: 30 * time.Second}
	}

	p.server = &http.Server{
		Addr:         listenAddr,
		Handler:      p,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return p
}

func (p *HTTPProxy) Start() error {
	slog.Info("HTTP proxy starting", "addr", p.listenAddr, "sourceIP", p.sourceIP)
	return p.server.ListenAndServe()
}

func (p *HTTPProxy) Close() error {
	return p.server.Close()
}

func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.username != "" && !p.authenticate(r) {
		w.Header().Set("Proxy-Authenticate", `Basic realm="Mobile Proxy"`)
		http.Error(w, "Proxy authentication required", http.StatusProxyAuthRequired)
		return
	}

	if r.Method == http.MethodConnect {
		p.handleConnect(w, r)
	} else {
		p.handleHTTP(w, r)
	}
}

func (p *HTTPProxy) authenticate(r *http.Request) bool {
	auth := r.Header.Get("Proxy-Authorization")
	if auth == "" {
		return false
	}

	if !strings.HasPrefix(auth, "Basic ") {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		return false
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return false
	}

	return parts[0] == p.username && parts[1] == p.password
}

func (p *HTTPProxy) handleConnect(w http.ResponseWriter, r *http.Request) {
	destConn, err := p.dialer.Dial("tcp", r.Host)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer destConn.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	done := make(chan struct{}, 2)
	go func() {
		io.Copy(destConn, clientConn)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(clientConn, destConn)
		done <- struct{}{}
	}()
	<-done
}

func (p *HTTPProxy) handleHTTP(w http.ResponseWriter, r *http.Request) {
	transport := &http.Transport{
		DialContext: p.dialer.DialContext,
	}

	r.RequestURI = ""

	resp, err := transport.RoundTrip(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("proxy error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
