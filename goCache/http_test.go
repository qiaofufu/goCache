package goCache

import (
	"net/http"
	"net/url"
	"testing"
)

func TestHTTPPool(t *testing.T) {
	var (
		etcdAddr = "http://162.14.115.114:2379"
		addr     = "http://localhost:8000"
	)
	pool := NewHTTPPool(addr, etcdAddr)
	pool.StartService()

	getUrl, err := url.JoinPath(addr, "test-group", "test-key")
	if err != nil {
		t.Fatalf("fortmat get url error, err: %v", err)
	}
	resp, err := http.Get(getUrl)
	t.Log(resp.StatusCode, resp.Status)
}
