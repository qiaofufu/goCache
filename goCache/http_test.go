package goCache

import (
	"net/http"
	"net/url"
	"testing"
)

func TestHTTPPool(t *testing.T) {
	var (
		addr = "http://localhost:8000"
	)
	pool := NewHTTPPool(addr)
	pool.SetPeerGetter(NewHTTPGetter("test", addr, 1))
	pool.StartService(addr[7:])

	getUrl, err := url.JoinPath(addr, "test-group", "test-key")
	if err != nil {
		t.Fatalf("fortmat get url error, err: %v", err)
	}
	resp, err := http.Get(getUrl)
	t.Log(resp.StatusCode, resp.Status)
}
