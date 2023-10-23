package main

import (
	"flag"
	"fmt"
	"goCache/goCache"
	"syscall"
)

var (
	db = map[string]string{
		"Tom": "123",
		"Jak": "354",
	}
	addr = flag.String("addr", "http://localhost:8000", "address")
)

func main() {
	flag.Parse()

	addrs := []string{
		"http://localhost:8000",
		"http://localhost:8001",
		"http://localhost:8002",
	}
	ch := make(chan syscall.Signal)
	peer := goCache.NewHTTPPool(*addr)
	var getters []goCache.PeerGetter
	for _, v := range addrs {
		getters = append(getters, goCache.NewHTTPGetter(v, v, 1))
	}
	peer.SetPeerGetter(getters...)

	peer.StartService((*addr)[7:])

	group := goCache.NewGroup("score", goCache.GetterFunc(func(key string) ([]byte, error) {
		if v, ok := db[key]; ok {
			return []byte(v), nil
		}
		return nil, fmt.Errorf("getter not found, key: %s", key)
	}))
	group.RegisterPeer(peer)
	<-ch
}
