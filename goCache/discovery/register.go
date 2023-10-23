package discovery

import (
	"context"
	"fmt"
	clientv3 "go.etcd.io/etcd/client/v3"
	"goCache/pb"
	"google.golang.org/protobuf/proto"
	"log"
)

type Register struct {
	serviceTarget string
	client        *clientv3.Client
	leaseID       clientv3.LeaseID
	keepaliveChan <-chan *clientv3.LeaseKeepAliveResponse
	value         []byte
}

func NewRegister(endpoints []string, serviceTarget string, serviceNode ServiceNode, leaseExpire int64) *Register {
	client, err := clientv3.New(clientv3.Config{
		Endpoints: endpoints,
	})
	if err != nil {
		panic(fmt.Errorf("failed to new etcd client, err: %v", err))
	}

	// 创建租约
	lease, err := client.Grant(context.TODO(), leaseExpire)
	if err != nil {
		panic(fmt.Errorf("failed grant lease, err: %v", err))
	}

	// 设置租约不过期
	leaseKeepaliveChan, err := client.KeepAlive(context.TODO(), lease.ID)
	if err != nil {
		panic(err)
	}

	// 进行注册
	key := fmt.Sprintf("%s-%d", serviceTarget, lease.ID)
	value, err := proto.Marshal(&pb.ServiceNode{
		Name:   serviceNode.Name,
		Addr:   serviceNode.Addr,
		Weight: serviceNode.Weight,
	})
	if err != nil {
		panic(fmt.Errorf("failed to marshal ServiceNode, err: %v", err))
	}
	_, err = client.Put(context.TODO(), key, string(value), clientv3.WithLease(lease.ID))
	if err != nil {
		panic(fmt.Errorf("register service to etcd failed, err: %v", err))
	}
	rg := &Register{
		client:        client,
		serviceTarget: serviceTarget,
		leaseID:       lease.ID,
		keepaliveChan: leaseKeepaliveChan,
	}

	return rg
}

func (r *Register) ListenLeaseRespChan() {
	for resp := range r.keepaliveChan {
		log.Println("租约续租成功", resp)
	}
	log.Println("关闭租约")
}

func (r *Register) Close() error {
	if _, err := r.client.Revoke(context.TODO(), r.leaseID); err != nil {
		return err
	}
	log.Println("撤销租约")
	return r.client.Close()
}
