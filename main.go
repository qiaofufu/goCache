package main

import (
	"flag"
	"fmt"
	"goCache/goCache"
	"net/http"
	"syscall"
)

var (
	db = map[string]string{
		"Tom": "123",
		"Jak": "354",
	}
	addr     = flag.String("addr", "http://localhost:8000", "address")
	api      = flag.Bool("api", false, "enable flag")
	etcdAddr = "http://162.14.115.114:2379"
)

func main() {
	flag.Parse()
	ch := make(chan syscall.Signal)
	peer := goCache.NewGrpcPeer(*addr, etcdAddr)
	peer.StartService()

	group := goCache.NewGroup("score", goCache.GetterFunc(func(key string) ([]byte, error) {
		if v, ok := db[key]; ok {
			return []byte(v), nil
		}
		return nil, fmt.Errorf("getter not found, key: %s", key)
	}))
	group.RegisterPeer(peer)
	if *api {
		StartAPI(group)
	}
	<-ch
}

func StartAPI(cache *goCache.Group) {
	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		key := request.URL.Query().Get("key")
		value, err := cache.Get(key)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			writer.Write([]byte(err.Error()))
			return
		}
		writer.WriteHeader(http.StatusOK)
		writer.Write(value.Slice())
	})
	http.ListenAndServe(":9999", nil)
}
