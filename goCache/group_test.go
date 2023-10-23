package goCache

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
)

var db = map[string]string{
	"Tom":  "123",
	"Jack": "456",
}

// TestGroup_Breakdown 测试single flight解决缓存击穿
func TestGroup_Breakdown(t *testing.T) {
	var (
		addr     = "http://localhost:8000"
		etcdAddr = ""
		cnt      atomic.Int32
	)
	// 初始化peer
	peer := NewHTTPPool(addr, etcdAddr)
	peer.StartService()
	// 初始化group
	group := NewGroup("score", GetterFunc(func(key string) ([]byte, error) {
		if v, ok := db[key]; ok {
			log.Println("[Slow DB] hit")
			cnt.Add(1)
			return []byte(v), nil
		}
		return nil, fmt.Errorf("[Slow DB] not have")
	}))
	// 为group 注册 peer
	group.RegisterPeer(peer)

	u, err := url.JoinPath(addr, "score", "Tom")
	if err != nil {
		t.Fatalf("join get url failed, err: %v", err)
	}
	wg := sync.WaitGroup{}
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			http.Get(u)
			wg.Done()
		}()
	}
	wg.Wait()
	if cnt.Load() != 1 {
		t.Fatalf("singleflight not impl, len: %d", cnt.Load())
	}
	t.Log(cnt.Load())

}

// TestGroup_Penetration
// 测试缓存穿透问题， 缓存穿透问题的解决办法，
//
//	1.设置空对象缓存记录（容易被污染缓存） 2.设置布隆过滤器（只能知道当前节点的存在情况）
//
// 缓存穿透问题应该在业务层解决
func TestGroup_Penetration(t *testing.T) {
	var (
		addr     = "http://localhost:8000"
		etcdAddr = ""
		cnt      atomic.Int32
	)
	// 初始化peer
	peer := NewHTTPPool(addr, etcdAddr)
	peer.StartService()
	// 初始化group
	group := NewGroup("score", GetterFunc(func(key string) ([]byte, error) {
		cnt.Add(1)
		if v, ok := db[key]; ok {
			log.Println("[Slow DB] hit")
			return []byte(v), nil
		}
		return nil, fmt.Errorf("[Slow DB] not have")
	}))
	// 为group 注册 peer
	group.RegisterPeer(peer)

	wg := sync.WaitGroup{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			u, err := url.JoinPath(addr, "score", fmt.Sprintf("TEST-%d", i))
			if err != nil {
				t.Fatalf("join get url failed, err: %v", err)
			}
			http.Get(u)
			wg.Done()
		}(i)
	}
	wg.Wait()

	if cnt.Load() == 10 {
		t.Fatalf("singleflight not impl, len: %d", cnt.Load())
	}

	t.Log(cnt.Load())

}
