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
			Timeout: 120 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
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
				MaxIdleConns:          100,
				MaxIdleConnsPerHost:   20,
				IdleConnTimeout:       90 * time.Second,
				DisableCompression:    false,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ResponseHeaderTimeout: 60 * time.Second,
			},
		}
	})
	return streamHttpClient
}
