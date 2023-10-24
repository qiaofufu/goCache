package goCache

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"goCache/goCache/consistent"
	"goCache/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"sync"
	"time"
)

type GrpcPeer struct {
	registerCli    *clientv3.Client // 注册中心Client
	discoveryCli   *clientv3.Client
	leaseRespChan  <-chan *clientv3.LeaseKeepAliveResponse
	self           string // 自身地址
	weight         int32  // 该节点权重
	mu             sync.RWMutex
	consistentHash *consistent.Consistent // 一致性hash
	getters        map[string]PeerGetter
	pb.UnimplementedPeerServer
}

func (g *GrpcPeer) Hello(ctx context.Context, request *pb.HelloRequest) (*pb.HelloResponse, error) {
	return &pb.HelloResponse{}, nil
}

func (g *GrpcPeer) Get(ctx context.Context, request *pb.GetRequest) (*pb.GetResponse, error) {
	cache, ok := GetGroup(request.GetGroup())
	if !ok {
		return nil, fmt.Errorf("failed to get cache group, group:%s key:%s", request.GetGroup(), request.GetKey())
	}
	value, err := cache.Get(request.GetKey())
	if err != nil {
		return nil, err
	}

	return &pb.GetResponse{
		Value: value.Slice(),
		Msg:   "success",
	}, nil
}

func (g *GrpcPeer) Set(ctx context.Context, request *pb.SetRequest) (*pb.SetResponse, error) {
	cache, ok := GetGroup(request.GetGroup())
	if !ok {
		return nil, fmt.Errorf("failed to get cache group, group: %s", request.GetGroup())
	}

	cache.Set(request.GetKey(), request.GetValue(), time.Duration(request.GetExpire()))

	return &pb.SetResponse{Msg: ""}, nil
}

func (g *GrpcPeer) Del(ctx context.Context, request *pb.DelRequest) (*pb.DelResponse, error) {
	cache, ok := GetGroup(request.GetGroup())
	if !ok {
		return nil, fmt.Errorf("failed to get cache group, group: %s", request.GetGroup())
	}
	err := cache.Remove(request.GetKey())
	if err != nil {
		return nil, err
	}
	return &pb.DelResponse{}, nil
}

func NewGrpcPeer(addr string, endpoints ...string) *GrpcPeer {
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

	return &GrpcPeer{
		registerCli:    cli1,
		discoveryCli:   cli2,
		self:           addr,
		weight:         1,
		consistentHash: consistent.New(0, nil),
		getters:        make(map[string]PeerGetter),
	}
}

func (g *GrpcPeer) Register(prefix string, leaseExpire int64) {
	// 创建租约
	lease, err := g.registerCli.Grant(context.TODO(), leaseExpire)
	if err != nil {
		panic(fmt.Errorf("failed grant lease, err: %v", err))
	}

	// 设置租约不过期
	leaseKeepaliveResp, err := g.registerCli.KeepAlive(context.TODO(), lease.ID)
	if err != nil {
		panic(err)
	}
	g.leaseRespChan = leaseKeepaliveResp
	// 进行注册
	key := fmt.Sprintf("%s-%d", prefix, lease.ID)
	value, err := proto.Marshal(&pb.ServiceNode{
		Name:   g.self,
		Addr:   g.self,
		Weight: g.weight,
	})
	if err != nil {
		panic(fmt.Errorf("failed to marshal ServiceNode, err: %v", err))
	}
	_, err = g.registerCli.Put(context.TODO(), key, string(value), clientv3.WithLease(lease.ID))
	if err != nil {
		panic(fmt.Errorf("register service to etcd failed, err: %v", err))
	}
	go g.ListenLeaseResp()
}

func (g *GrpcPeer) ListenLeaseResp() {
	for _ = range g.leaseRespChan {
		//log.Println("租约续租成功", resp)
	}
	log.Println("关闭租约")
}

func (g *GrpcPeer) Discovery(prefix string) {
	// 初始获取服务节点
	resp, err := g.discoveryCli.Get(context.TODO(), prefix, clientv3.WithPrefix())
	if err != nil {
		panic(err)
	}
	for _, kv := range resp.Kvs {
		g.SetService(string(kv.Key), string(kv.Value))
	}

	// 开启监听注册中心
	go g.watch(g.discoveryCli, prefix)
}

func (g *GrpcPeer) watch(cli *clientv3.Client, prefix string) {
	watchCh := cli.Watch(context.TODO(), prefix, clientv3.WithPrefix())
	for resp := range watchCh {
		for _, ev := range resp.Events {
			switch ev.Type {
			case mvccpb.PUT:
				g.SetService(string(ev.Kv.Key), string(ev.Kv.Value))
			case mvccpb.DELETE:
				g.DelService(string(ev.Kv.Key))
			}
		}
	}
}

func (g *GrpcPeer) SetService(key string, value string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	log.Println("add server node, ", key)
	// 一致性hash节点添加
	t := &pb.ServiceNode{}
	err := proto.Unmarshal([]byte(value), t)
	if err != nil {
		panic(err)
	}
	g.consistentHash.AddNode(consistent.Node{
		Name:   t.GetName(),
		Addr:   t.GetAddr(),
		Weight: t.GetWeight(),
	})
	// PeerGetter 添加
	g.getters[t.GetName()] = NewGrpcGetter(t.GetName(), t.GetAddr())
}

func (g *GrpcPeer) DelService(key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	log.Println("del server node, ", key)
	g.consistentHash.DelNode(key)
	delete(g.getters, key)
}

func (g *GrpcPeer) PickPeer(key string) (PeerGetter, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	node, err := g.consistentHash.GetNode(key)
	if err != nil || node.Addr == g.self {
		return nil, false
	}
	return g.getters[node.Name], true
}

func (g *GrpcPeer) StartService() {
	var (
		done    = make(chan error)
		success = make(chan struct{})
	)
	go func() {
		listen, err := net.Listen("tcp", g.self)
		if err != nil {
			panic(fmt.Errorf("failed to listen %s, err: %v", g.self, err))
		}
		svr := grpc.NewServer()
		pb.RegisterPeerServer(svr, g)
		done <- svr.Serve(listen)
	}()
	go func() {
		for {
			conn, err := grpc.Dial(g.self, grpc.WithInsecure())
			if err != nil {
				panic(err)
			}
			_, err = pb.NewPeerClient(conn).Hello(context.TODO(), &pb.HelloRequest{})
			if err == nil {
				success <- struct{}{}
				return
			}
		}
	}()

	select {
	case err := <-done:
		panic(err)
	case <-success:
		// 进行服务注册
		g.Register(serviceTarget, 5)
		// 进行服务发现
		g.Discovery(serviceTarget)
		log.Println("start grpc server", g.self)
	}
}

type GrpcGetter struct {
	addr string
	name string
}

func NewGrpcGetter(addr string, name string) *GrpcGetter {
	return &GrpcGetter{
		addr: addr,
		name: name,
	}
}

func (g *GrpcGetter) Get(group string, key string) ([]byte, error) {
	conn, err := grpc.Dial(g.addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	client := pb.NewPeerClient(conn)
	response, err := client.Get(context.TODO(), &pb.GetRequest{
		Group: group,
		Key:   key,
	})
	if err != nil {
		return nil, err
	}
	return response.GetValue(), nil
}

func (g *GrpcGetter) Remove(group string, key string) error {
	var (
		req = &pb.DelRequest{
			Group: group,
			Key:   key,
		}
	)
	conn, err := grpc.Dial(g.addr)
	if err != nil {
		return err
	}
	client := pb.NewPeerClient(conn)
	_, err = client.Del(context.TODO(), req)
	if err != nil {
		return fmt.Errorf("failed to send grpc request, err: %v", err)
	}
	return nil
}

func (g *GrpcGetter) Name() string {
	return g.name
}

func (g *GrpcGetter) Addr() string {
	return g.addr
}
