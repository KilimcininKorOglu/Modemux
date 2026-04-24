package proxy

import (
	"fmt"
	"log/slog"
	"sync"
)

type ModemProxy struct {
	ModemIndex int
	SourceIP   string
	HTTP       *HTTPProxy
	SOCKS5     *SOCKS5Proxy
	HTTPPort   int
	SOCKS5Port int
}

type Manager struct {
	mu             sync.RWMutex
	proxies        map[int]*ModemProxy
	httpPortStart  int
	socks5PortStart int
	username       string
	password       string
	authRequired   bool
}

func NewManager(httpPortStart, socks5PortStart int, authRequired bool, username, password string) *Manager {
	return &Manager{
		proxies:         make(map[int]*ModemProxy),
		httpPortStart:   httpPortStart,
		socks5PortStart: socks5PortStart,
		username:        username,
		password:        password,
		authRequired:    authRequired,
	}
}

func (m *Manager) StartProxy(modemIndex int, sourceIP string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.proxies[modemIndex]; exists {
		return fmt.Errorf("proxy already running for modem %d", modemIndex)
	}

	httpPort := m.httpPortStart + modemIndex
	socks5Port := m.socks5PortStart + modemIndex

	user, pass := "", ""
	if m.authRequired {
		user = m.username
		pass = m.password
	}

	httpAddr := fmt.Sprintf("0.0.0.0:%d", httpPort)
	socks5Addr := fmt.Sprintf("0.0.0.0:%d", socks5Port)

	httpProxy := NewHTTPProxy(httpAddr, sourceIP, user, pass)
	socks5Proxy := NewSOCKS5Proxy(socks5Addr, sourceIP, user, pass)

	mp := &ModemProxy{
		ModemIndex: modemIndex,
		SourceIP:   sourceIP,
		HTTP:       httpProxy,
		SOCKS5:     socks5Proxy,
		HTTPPort:   httpPort,
		SOCKS5Port: socks5Port,
	}

	go func() {
		if err := httpProxy.Start(); err != nil {
			slog.Error("HTTP proxy stopped", "modem", modemIndex, "error", err)
		}
	}()

	go func() {
		if err := socks5Proxy.Start(); err != nil {
			slog.Error("SOCKS5 proxy stopped", "modem", modemIndex, "error", err)
		}
	}()

	m.proxies[modemIndex] = mp

	slog.Info("proxy started",
		"modem", modemIndex,
		"sourceIP", sourceIP,
		"httpPort", httpPort,
		"socks5Port", socks5Port,
	)

	return nil
}

func (m *Manager) StopProxy(modemIndex int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mp, exists := m.proxies[modemIndex]
	if !exists {
		return nil
	}

	if mp.HTTP != nil {
		mp.HTTP.Close()
	}
	if mp.SOCKS5 != nil {
		mp.SOCKS5.Close()
	}

	delete(m.proxies, modemIndex)
	slog.Info("proxy stopped", "modem", modemIndex)
	return nil
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for idx, mp := range m.proxies {
		if mp.HTTP != nil {
			mp.HTTP.Close()
		}
		if mp.SOCKS5 != nil {
			mp.SOCKS5.Close()
		}
		slog.Info("proxy stopped", "modem", idx)
	}

	m.proxies = make(map[int]*ModemProxy)
}

func (m *Manager) GetProxy(modemIndex int) *ModemProxy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.proxies[modemIndex]
}

func (m *Manager) GetAllProxies() map[int]*ModemProxy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[int]*ModemProxy, len(m.proxies))
	for k, v := range m.proxies {
		result[k] = v
	}
	return result
}

func (m *Manager) GetPorts(modemIndex int) (httpPort, socks5Port int) {
	return m.httpPortStart + modemIndex, m.socks5PortStart + modemIndex
}
