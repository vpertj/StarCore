package provider

import (
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var (
	globalProxy   string
	globalNoProxy string
	proxyMu       sync.RWMutex
)

// SetGlobalProxy configures the HTTP proxy for all providers.
// proxyURL should be like "http://host:port" or "socks5://host:port".
// noProxy is a comma-separated list of hosts that should bypass the proxy.
func SetGlobalProxy(proxyURL, noProxy string) {
	proxyMu.Lock()
	globalProxy = proxyURL
	globalNoProxy = noProxy
	proxyMu.Unlock()
}

// GetGlobalProxy returns the current proxy configuration.
func GetGlobalProxy() (proxyURL, noProxy string) {
	proxyMu.RLock()
	defer proxyMu.RUnlock()
	return globalProxy, globalNoProxy
}

// NewHTTPClient creates an http.Client with the global proxy configuration.
func NewHTTPClient(timeout time.Duration) *http.Client {
	proxyMu.RLock()
	proxyURL := globalProxy
	noProxy := globalNoProxy
	proxyMu.RUnlock()

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}

	if proxyURL != "" {
		proxyParsed, err := url.Parse(proxyURL)
		if err == nil {
			transport.Proxy = http.ProxyURL(proxyParsed)
		}
	}

	if noProxy != "" {
		originalProxy := transport.Proxy
		noProxyList := parseNoProxy(noProxy)
		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			host := req.URL.Hostname()
			for _, np := range noProxyList {
				if host == np || matchNoProxyPattern(host, np) {
					return nil, nil
				}
			}
			if originalProxy != nil {
				return originalProxy(req)
			}
			return http.ProxyFromEnvironment(req)
		}
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func parseNoProxy(noProxy string) []string {
	if noProxy == "" {
		return nil
	}
	var result []string
	for _, s := range splitComma(noProxy) {
		s = trimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

func matchNoProxyPattern(host, pattern string) bool {
	if len(pattern) > 0 && pattern[0] == '.' {
		return len(host) > len(pattern) && host[len(host)-len(pattern):] == pattern
	}
	return false
}

func splitComma(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
