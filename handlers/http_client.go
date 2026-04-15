package handlers

import (
	"net"
	"net/http"
	"sync"
	"time"
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
)

func GetHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		httpClient = &http.Client{
			Timeout: upstreamRequestTimeout,
			Transport: &http.Transport{
				MaxIdleConns:          200,
				MaxIdleConnsPerHost:   50,
				MaxConnsPerHost:       100,
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
		streamHttpClient = &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        200,
				MaxIdleConnsPerHost: 50,
				MaxConnsPerHost:     100,
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
