package handlers

import (
	"net"
	"net/http"
	"sync"
	"time"

	"llm-gateway/config"
)

var (
	httpClient       *http.Client
	streamHttpClient *http.Client
	httpClientOnce   sync.Once
	streamClientOnce sync.Once
)

const (
	upstreamRequestTimeout        = 600 * time.Second
	upstreamDialTimeout           = 30 * time.Second
	upstreamKeepAlive             = 60 * time.Second
	upstreamTLSHandshakeTimeout   = 30 * time.Second
	upstreamResponseHeaderTimeout = 600 * time.Second
	upstreamExpectContinueTimeout = 10 * time.Second
	upstreamIdleConnTimeout       = 300 * time.Second
	defaultUpstreamMaxIdleConns        = 8000
	defaultUpstreamMaxIdleConnsPerHost = 5000
	defaultUpstreamMaxConnsPerHost     = 5000
)

type upstreamPoolSettings struct {
	maxIdleConns        int
	maxIdleConnsPerHost int
	maxConnsPerHost     int
}

func getUpstreamPoolSettings() upstreamPoolSettings {
	if config.AppConfig == nil {
		return upstreamPoolSettings{
			maxIdleConns:        defaultUpstreamMaxIdleConns,
			maxIdleConnsPerHost: defaultUpstreamMaxIdleConnsPerHost,
			maxConnsPerHost:     defaultUpstreamMaxConnsPerHost,
		}
	}

	settings := upstreamPoolSettings{
		maxIdleConns:        config.AppConfig.UpstreamMaxIdleConns,
		maxIdleConnsPerHost: config.AppConfig.UpstreamMaxIdleConnsPerHost,
		maxConnsPerHost:     config.AppConfig.UpstreamMaxConnsPerHost,
	}
	if settings.maxIdleConns <= 0 {
		settings.maxIdleConns = defaultUpstreamMaxIdleConns
	}
	if settings.maxIdleConnsPerHost <= 0 {
		settings.maxIdleConnsPerHost = defaultUpstreamMaxIdleConnsPerHost
	}
	if settings.maxConnsPerHost <= 0 {
		settings.maxConnsPerHost = defaultUpstreamMaxConnsPerHost
	}
	return settings
}

func GetHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		pool := getUpstreamPoolSettings()
		httpClient = &http.Client{
			Timeout: upstreamRequestTimeout,
			Transport: &http.Transport{
				MaxIdleConns:          pool.maxIdleConns,
				MaxIdleConnsPerHost:   pool.maxIdleConnsPerHost,
				MaxConnsPerHost:       pool.maxConnsPerHost,
				IdleConnTimeout:       upstreamIdleConnTimeout,
				TLSHandshakeTimeout:   upstreamTLSHandshakeTimeout,
				ResponseHeaderTimeout: upstreamResponseHeaderTimeout,
				ExpectContinueTimeout: upstreamExpectContinueTimeout,
				DisableCompression:    false,
				DialContext: (&net.Dialer{
					Timeout:   upstreamDialTimeout,
					KeepAlive: upstreamKeepAlive,
				}).DialContext,
			},
		}
	})
	return httpClient
}

func GetStreamHTTPClient() *http.Client {
	streamClientOnce.Do(func() {
		pool := getUpstreamPoolSettings()
		streamHttpClient = &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        pool.maxIdleConns,
				MaxIdleConnsPerHost: pool.maxIdleConnsPerHost,
				MaxConnsPerHost:     pool.maxConnsPerHost,
				IdleConnTimeout:     upstreamIdleConnTimeout,
				DisableCompression:  false,
				DialContext: (&net.Dialer{
					Timeout:   upstreamDialTimeout,
					KeepAlive: upstreamKeepAlive,
				}).DialContext,
				TLSHandshakeTimeout:   upstreamTLSHandshakeTimeout,
				ResponseHeaderTimeout: upstreamResponseHeaderTimeout,
				ExpectContinueTimeout: upstreamExpectContinueTimeout,
			},
		}
	})
	return streamHttpClient
}
