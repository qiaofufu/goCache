package discovery

import (
	"context"
	"fmt"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"goCache/pb"
	"google.golang.org/protobuf/proto"
	"sync"
	"time"
)

type ServiceNode struct {
	Name   string
	Addr   string
	Weight int32
}

func (s *ServiceNode) String() string {
	node := &pb.ServiceNode{
		Name:   s.Name,
		Addr:   s.Addr,
		Weight: s.Weight,
	}
	data, err := proto.Marshal(node)
	if err != nil {
		panic(fmt.Errorf("failed to marshal pb.ServiceNode, err: %v", err))
	}
	return string(data)
}

type Discovery struct {
	client       *clientv3.Client
	lock         sync.Mutex
	serviceList  map[string]ServiceNode
	onSetService func(key string, value string)
	onDelService func(key string)
}

func NewDiscovery(endpoints []string, onSetService func(string, string), onDelService func(string)) *Discovery {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		panic(fmt.Errorf("failed to new etcd client, err: %v", err))
	}
	return &Discovery{
		client:       cli,
		serviceList:  make(map[string]ServiceNode),
		onSetService: onSetService,
		onDelService: onDelService,
	}
}

func (d *Discovery) WatchService(serviceTarget string) error {
	resp, err := d.client.Get(context.TODO(), serviceTarget, clientv3.WithPrefix())
	if err != nil {
		return err
	}

	for _, kv := range resp.Kvs {
		d.lock.Lock()
		d.onSetService(string(kv.Key), string(kv.Value))
		d.lock.Unlock()
	}
	go d.watch(serviceTarget)
	return nil
}

func (d *Discovery) watch(serviceTarget string) {
	watchCh := d.client.Watch(context.TODO(), serviceTarget, clientv3.WithPrefix())
	for resp := range watchCh {
		for _, ev := range resp.Events {
			switch ev.Type {
			case mvccpb.PUT:
				d.lock.Lock()
				d.onSetService(string(ev.Kv.Key), string(ev.Kv.Value))
				d.lock.Unlock()
			case mvccpb.DELETE:
				d.lock.Lock()
				d.onDelService(string(ev.Kv.Key))
				d.lock.Unlock()
			}
		}
	}
}
