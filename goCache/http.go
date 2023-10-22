package goCache

import (
	"bytes"
	"fmt"
	"goCache/goCache/consistent"
	"goCache/pb"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		HTTPPicker: HTTPPicker{
			self:           self,
			consistentHash: consistent.New(0, nil),
		}}
}

type HTTPPool struct {
	HTTPPicker
}

func (H *HTTPPool) StartService(addr string) {
	var (
		done    = make(chan error)
		success = make(chan struct{})
	)
	go func() {
		done <- http.ListenAndServe(addr, H)
	}()
	go func() {
		http.Get(addr)
		success <- struct{}{}
	}()
	select {
	case err := <-done:
		panic(err)
	case <-success:
		log.Println("goCache start, addr:", addr)
	}
}

func (H *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		H.GetHandler(w, r)
	case http.MethodDelete:
		H.DeleteHandler(w, r)
	case http.MethodPost:
		H.PostHandler(w, r)
	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func (H *HTTPPool) GetHandler(w http.ResponseWriter, r *http.Request) {
	paths := strings.SplitN(r.URL.Path[1:], "/", 2)
	group, key := paths[0], paths[1]
	cache, exist := GetGroup(group)
	if !exist {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	value, err := cache.Get(key)
	if err != nil {
		log.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	body, err := proto.Marshal(&pb.GetResponse{Value: value.Slice()})
	if err != nil {
		log.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func (H *HTTPPool) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	paths := strings.SplitN(r.URL.Path, "/", 2)
	group, key := paths[0], paths[1]
	cache, exist := GetGroup(group)
	if !exist {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := cache.Remove(key); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (H *HTTPPool) PostHandler(w http.ResponseWriter, r *http.Request) {
	paths := strings.SplitN(r.URL.Path, "/", 1)
	group := paths[0]
	cache, exist := GetGroup(group)
	if !exist {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(r.Body)
	req := &pb.SetRequest{}
	if err = proto.Unmarshal(body, req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	cache.Set(req.Key, req.Value, time.Duration(req.Expire))
	w.WriteHeader(http.StatusOK)
}

/* HTTPPicker */

type HTTPPicker struct {
	self           string // 自身地址
	mu             sync.RWMutex
	consistentHash *consistent.Consistent
	getters        map[string]PeerGetter
}

func (H *HTTPPicker) PickPeer(key string) (PeerGetter, bool) {
	H.mu.RLock()
	defer H.mu.RUnlock()
	node, err := H.consistentHash.GetNode(key)
	if err != nil || node.Addr == H.self {
		return nil, false
	}
	log.Println(H.self, node.Addr)
	return H.getters[node.Name], true
}

func (H *HTTPPicker) SetPeerGetter(getters ...PeerGetter) {
	H.mu.Lock()
	defer H.mu.Unlock()
	// 一致性hash节点添加
	var nodes = make([]consistent.Node, 0, len(getters))
	for _, getter := range getters {
		nodes = append(nodes, consistent.Node{
			Name:   getter.Name(),
			Addr:   getter.Addr(),
			Weight: getter.Weight(),
		})
	}
	H.consistentHash = consistent.New(0, nil)
	H.consistentHash.AddNodes(nodes...)

	// PeerGetter 添加
	H.getters = make(map[string]PeerGetter)
	for _, getter := range getters {
		H.getters[getter.Name()] = getter
	}
}

/* HTTP Getter */

func NewHTTPGetter(serverName string, addr string, weight int) *HTTPGetter {
	return &HTTPGetter{
		name:    serverName,
		baseURl: addr,
		weight:  weight,
	}
}

type HTTPGetter struct {
	name    string
	baseURl string
	weight  int
}

func (H HTTPGetter) Addr() string {
	return H.baseURl
}

func (H HTTPGetter) Name() string {
	return H.name
}

func (H HTTPGetter) Weight() int {
	return H.weight
}

func (H HTTPGetter) Get(group string, key string) ([]byte, error) {
	u, err := url.JoinPath(H.baseURl, url.QueryEscape(group), url.QueryEscape(key))
	if err != nil {
		return nil, fmt.Errorf("failed to splicing url, err: %v", err)
	}
	resp, err := Get(u)
	if err != nil {
		return nil, fmt.Errorf("failed to send request, err: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body, err: %v", err)
	}
	respData := pb.GetResponse{}
	if err = proto.Unmarshal(body, &respData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body, err: %v", err)
	}
	return respData.Value, nil
}

func (H HTTPGetter) Remove(namespace string, key string) error {
	u, err := url.JoinPath(H.baseURl, url.QueryEscape(namespace), url.QueryEscape(key))
	if err != nil {
		return fmt.Errorf("failed to splicing url, err: %v", err)
	}
	if _, err = Delete(u); err != nil {
		return err
	}
	return nil
}

func (H HTTPGetter) Set(namespace string, key string, value []byte, expire time.Duration) error {
	u, err := url.JoinPath(H.baseURl, url.QueryEscape(namespace))
	if err != nil {
		return fmt.Errorf("failed to splicing url, err: %v", err)
	}
	req := &pb.SetRequest{
		Namespace: namespace,
		Key:       key,
		Value:     value,
		Expire:    int64(expire),
	}
	body, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal requset body, err: %v", err)
	}

	if _, err = Post(u, body); err != nil {
		return err
	}
	return nil
}

/* HTTP 请求封装 */

func Get(url string) (*http.Response, error) {
	return Request(http.MethodGet, url, nil)
}

func Delete(url string) (*http.Response, error) {
	return Request(http.MethodDelete, url, nil)
}

func Post(url string, body []byte) (*http.Response, error) {
	reader := bytes.NewReader(body)
	return Request(http.MethodPost, url, reader)
}

func Request(method string, url string, body io.Reader) (resp *http.Response, err error) {
	client := http.Client{}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to new request, err: %v", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("failed to sent request, err: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return resp, fmt.Errorf("faield to send request, status: %s", resp.Status)
	}
	return
}
