package server

import (
	"bytes"
	"fmt"
	"goCache/pb"
	"goCache/server/consistent"
	"google.golang.org/protobuf/proto"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPPool struct {
	HTTPPicker
}

func (H *HTTPPool) ServerHTTP(w http.ResponseWriter, r *http.Request) {
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
	paths := strings.SplitN(r.URL.Path, "/", 2)
	namespace, key := paths[0], paths[1]
	cache := GetCache(namespace)
	value, err := cache.Get(key)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	body, err := proto.Marshal(&pb.GetResponse{Value: value.Slice()})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(body)
	w.WriteHeader(http.StatusOK)
}

func (H *HTTPPool) DeleteHandler(w http.ResponseWriter, r *http.Request) {
	paths := strings.SplitN(r.URL.Path, "/", 2)
	namespace, key := paths[0], paths[1]
	cache := GetCache(namespace)
	if err := cache.Remove(key); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (H *HTTPPool) PostHandler(w http.ResponseWriter, r *http.Request) {
	paths := strings.SplitN(r.URL.Path, "/", 1)
	namespace := paths[0]
	cache := GetCache(namespace)
	body, err := io.ReadAll(r.Body)
	req := &pb.SetRequest{}
	if err = proto.Unmarshal(body, req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	cache.Set(req.Key, req.Value, time.Duration(req.Expire))
	w.WriteHeader(http.StatusOK)
}

func (H *HTTPPool) StartPeerServer(addr string, peerAddr ...string) {
	var getters = make([]Getter, len(peerAddr))
	for i := range peerAddr {
		getters = append(getters, NewHTTPGetter(peerAddr[i], peerAddr[i]))
	}
	H.AddGetters(getters...)
	panic(http.ListenAndServe(addr, nil))
}

/* HTTPPicker */

type HTTPPicker struct {
	consistentHash consistent.Consistent
	getters        map[string]Getter
}

func (H *HTTPPicker) PickerPeer(key string) (Getter, error) {
	node, err := H.consistentHash.GetNode(key)
	if err != nil {
		return nil, err
	}
	return H.getters[node.Name], nil
}

func (H *HTTPPicker) AddGetter(getter Getter) {
	H.getters[getter.Name()] = getter
	H.consistentHash.AddNode(consistent.Node{
		Name:   getter.Name(),
		Addr:   getter.Addr(),
		Weight: 1,
	})
}

func (H *HTTPPicker) AddGetters(getters ...Getter) {
	nodes := make([]consistent.Node, 0, len(getters))
	for _, getter := range getters {
		nodes = append(nodes, consistent.Node{
			Name:   getter.Name(),
			Addr:   getter.Addr(),
			Weight: 1,
		})
		H.getters[getter.Name()] = getter
	}

	H.consistentHash.AddNodes(nodes...)
}

func (H *HTTPPicker) UpdateGetter(getter Getter) {
	H.getters[getter.Name()] = getter
	H.consistentHash.UpdateNode(consistent.Node{
		Name:   getter.Name(),
		Addr:   getter.Addr(),
		Weight: 1,
	})
}

func (H *HTTPPicker) RemoveGetter(name string) {
	delete(H.getters, name)
	H.consistentHash.Remove(name)
}

/* HTTP Getter */

type HTTPGetter struct {
	name    string
	baseURl string
}

func NewHTTPGetter(name string, addr string) *HTTPGetter {
	return &HTTPGetter{
		name:    name,
		baseURl: addr,
	}
}

func (H HTTPGetter) Addr() string {
	return H.baseURl
}

func (H HTTPGetter) Name() string {
	return H.name
}

func (H HTTPGetter) Get(namespace string, key string) ([]byte, error) {
	u, err := url.JoinPath(H.baseURl, url.QueryEscape(namespace), url.QueryEscape(key))
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
