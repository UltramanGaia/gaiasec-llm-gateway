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

func GetHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		httpClient = &http.Client{
			Timeout: 300 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     300 * time.Second,
				DisableCompression:  false,
			},
		}
	})
	return httpClient
}

func GetStreamHTTPClient() *http.Client {
	streamClientOnce.Do(func() {
		streamHttpClient = &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     300 * time.Second,
				DisableCompression:  false,
				DialContext: (&net.Dialer{
					Timeout:   300 * time.Second,
					KeepAlive: 300 * time.Second,
				}).DialContext,
				ResponseHeaderTimeout: 3000 * time.Second,
			},
		}
	})
	return streamHttpClient
}
