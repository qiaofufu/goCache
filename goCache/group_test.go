package goCache

import (
	"fmt"
	"goCache/goCache/option"
	"log"
	"testing"
)

var db = map[string]string{
	"Tom":  "123",
	"Jack": "456",
}

func TestGroup_Get(t *testing.T) {
	var (
		addrs = []string{
			"http://localhost:8000",
			"http://localhost:8001",
			"http://localhost:8002",
		}
	)
	// 初始化Peer， group
	var getters []PeerGetter
	for i, addr := range addrs {
		getters = append(getters, NewHTTPGetter(fmt.Sprintf("SERVER-%d", i), addr, 1))
	}
	for _, addr := range addrs {
		peer := NewHTTPPool(addr)
		peer.StartService(addr[7:])
		peer.SetPeerGetter(getters...) // 初始化group
		group := NewGroup("score", GetterFunc(func(key string) ([]byte, error) {
			if v, ok := db[key]; ok {
				log.Println("[Slow DB] hit, key:", key, "value:", v)
				return []byte(v), nil
			} else {
				log.Println("[Slow DB] not hit, key:", key)
				return nil, fmt.Errorf("not found")
			}
		}), option.WithCacheOptionsStrategy("lru", 2<<32, nil))

		// 为group注册peer
		group.RegisterPeer(peer)
	}

	group, ok := GetGroup("score")
	if !ok {
		t.Fatalf("score group not exist")
	}
	// 获取可命中缓存
	val, err := group.Get("Tom")
	if err != nil {
		t.Fatalf("failed to get cache")
	}
	log.Println("get val:", val)

	// 获取不可命中缓存
	val, err = group.Get("tim")
	if err == nil {
		t.Fatalf("failed to get not exist cache")
	}

}
