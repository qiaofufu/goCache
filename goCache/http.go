package goCache

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"goCache/goCache/consistent"
	"goCache/goCache/utls"
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

const (
	serviceTarget = "cache_service_prefix"
)

func NewHTTPPool(addr string, endpoints ...string) *HTTPPool {
	cli1, err := clientv3.New(clientv3.Config{
		Endpoints: endpoints,
	})
	if err != nil {
		panic(fmt.Errorf("failed to new etcd client, err: %v", err))
	}
	cli2, err := clientv3.New(clientv3.Config{
		Endpoints: endpoints,
	})
	if err != nil {
		panic(fmt.Errorf("failed to new etcd client, err: %v", err))
	}
	return &HTTPPool{
		self:           addr,
		weight:         1,
		registerCli:    cli1,
		discoveryCli:   cli2,
		consistentHash: consistent.New(0, nil),
		getters:        make(map[string]PeerGetter),
	}
}

type HTTPPool struct {
	registerCli    *clientv3.Client // 注册中心Client
	discoveryCli   *clientv3.Client
	leaseRespChan  <-chan *clientv3.LeaseKeepAliveResponse
	self           string // 自身地址
	weight         int32  // 该节点权重
	mu             sync.RWMutex
	consistentHash *consistent.Consistent // 一致性hash
	getters        map[string]PeerGetter
}

func (H *HTTPPool) StartService() {
	var (
		done    = make(chan error)
		success = make(chan struct{})
	)
	// 启动http服务
	go func() {
		done <- http.ListenAndServe(H.self[7:], H)
	}()
	// 检测http是否启动成功
	go func() {
		http.Get(H.self)
		success <- struct{}{}
	}()

	select {
	case err := <-done:
		panic(err)
	case <-success:
		// 进行服务注册
		H.Register(serviceTarget, 5)
		// 进行服务发现
		H.Discovery(serviceTarget)
	}
}

func (H *HTTPPool) Register(prefix string, leaseExpire int64) {
	// 创建租约
	lease, err := H.registerCli.Grant(context.TODO(), leaseExpire)
	if err != nil {
		panic(fmt.Errorf("failed grant lease, err: %v", err))
	}

	// 设置租约不过期
	leaseKeepaliveResp, err := H.registerCli.KeepAlive(context.TODO(), lease.ID)
	if err != nil {
		panic(err)
	}
	H.leaseRespChan = leaseKeepaliveResp
	// 进行注册
	key := fmt.Sprintf("%s-%d", prefix, lease.ID)
	value, err := proto.Marshal(&pb.ServiceNode{
		Name:   H.self,
		Addr:   H.self,
		Weight: H.weight,
	})
	if err != nil {
		panic(fmt.Errorf("failed to marshal ServiceNode, err: %v", err))
	}
	_, err = H.registerCli.Put(context.TODO(), key, string(value), clientv3.WithLease(lease.ID))
	if err != nil {
		panic(fmt.Errorf("register service to etcd failed, err: %v", err))
	}
	go H.ListenLeaseResp()
}

func (H *HTTPPool) ListenLeaseResp() {
	for _ = range H.leaseRespChan {
		//log.Println("租约续租成功", resp)
	}
	log.Println("关闭租约")
}

func (H *HTTPPool) Discovery(prefix string) {

	// 初始获取服务节点
	resp, err := H.discoveryCli.Get(context.TODO(), prefix, clientv3.WithPrefix())
	if err != nil {
		panic(err)
	}
	for _, kv := range resp.Kvs {
		H.SetService(string(kv.Key), string(kv.Value))
	}

	// 开启监听注册中心
	go H.watch(H.discoveryCli, prefix)
}

func (H *HTTPPool) watch(cli *clientv3.Client, prefix string) {
	watchCh := cli.Watch(context.TODO(), prefix, clientv3.WithPrefix())
	for resp := range watchCh {
		for _, ev := range resp.Events {
			switch ev.Type {
			case mvccpb.PUT:
				H.SetService(string(ev.Kv.Key), string(ev.Kv.Value))
			case mvccpb.DELETE:
				H.DelService(string(ev.Kv.Key))
			}
		}
	}
}

func (H *HTTPPool) SetService(key string, value string) {
	H.mu.Lock()
	defer H.mu.Unlock()

	// 一致性hash节点添加
	t := &pb.ServiceNode{}
	err := proto.Unmarshal([]byte(value), t)
	if err != nil {
		panic(err)
	}
	H.consistentHash.AddNode(consistent.Node{
		Name:   t.GetName(),
		Addr:   t.GetAddr(),
		Weight: t.GetWeight(),
	})
	// PeerGetter 添加
	H.getters[t.GetName()] = NewHTTPGetter(t.GetName(), t.GetAddr())
}

func (H *HTTPPool) DelService(key string) {
	H.mu.Lock()
	defer H.mu.Unlock()
	H.consistentHash.DelNode(key)
	delete(H.getters, key)
}

func (H *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	H.mu.RLock()
	defer H.mu.RUnlock()
	node, err := H.consistentHash.GetNode(key)
	if err != nil || node.Addr == H.self {
		return nil, false
	}
	return H.getters[node.Name], true
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
	var (
		resp pb.GetResponse
		in   pb.GetRequest
	)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		resp.Msg = "failed to read request body"
		data, _ := proto.Marshal(&resp)
		w.Write(data)
		return
	}
	if err = proto.Unmarshal(data, &in); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		resp.Msg = "failed to unmarshal request body"
		data, _ := proto.Marshal(&resp)
		w.Write(data)
		return
	}
	cache, exist := GetGroup(in.GetGroup())
	if !exist {
		w.WriteHeader(http.StatusNotFound)
		resp.Msg = fmt.Sprintf("failed to get group, group: %s, key: %s", in.Group, in.Key)
		data, _ := proto.Marshal(&resp)
		w.Write(data)
		return
	}
	value, err := cache.Get(in.GetKey())
	if err != nil {
		log.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	resp.Value = value.Slice()
	body, err := proto.Marshal(&resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		resp.Msg = "failed to marshal response body"
		data, _ := proto.Marshal(&resp)
		w.Write(data)
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

/* HTTP Getter */

func NewHTTPGetter(serverName string, addr string) *HTTPGetter {
	return &HTTPGetter{
		name:    serverName,
		baseURl: addr,
	}
}

type HTTPGetter struct {
	name    string
	baseURl string
}

func (H HTTPGetter) Addr() string {
	return H.baseURl
}

func (H HTTPGetter) Name() string {
	return H.name
}

func (H HTTPGetter) Get(group string, key string) ([]byte, error) {
	data, err := proto.Marshal(&pb.GetRequest{
		Group: group,
		Key:   key,
	})
	if err != nil {
		return nil, err
	}
	resp, err := utls.Get(H.baseURl, data)
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
	if _, err = utls.Delete(u); err != nil {
		return err
	}
	return nil
}

func (H HTTPGetter) Set(group string, key string, value []byte, expire time.Duration) error {
	u, err := url.JoinPath(H.baseURl, url.QueryEscape(group))
	if err != nil {
		return fmt.Errorf("failed to splicing url, err: %v", err)
	}
	req := &pb.SetRequest{
		Group:  group,
		Key:    key,
		Value:  value,
		Expire: int64(expire),
	}
	body, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal requset body, err: %v", err)
	}

	if _, err = utls.Post(u, body); err != nil {
		return err
	}
	return nil
}
