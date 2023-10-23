package goCache

import (
	"context"
	clientv3 "go.etcd.io/etcd/client/v3"
	"goCache/pb"
	"google.golang.org/grpc"
)

type GrpcPeer struct {
}

func (g *GrpcPeer) Register(prefix string, leaseExpire int64) {
	//TODO implement me
	panic("implement me")
}

func (g *GrpcPeer) ListenLeaseResp() {
	//TODO implement me
	panic("implement me")
}

func (g *GrpcPeer) Discovery(prefix string) {
	//TODO implement me
	panic("implement me")
}

func (g *GrpcPeer) watch(cli *clientv3.Client, prefix string) {
	//TODO implement me
	panic("implement me")
}

func (g *GrpcPeer) SetService(key string, value string) {
	//TODO implement me
	panic("implement me")
}

func (g *GrpcPeer) DelService(key string) {
	//TODO implement me
	panic("implement me")
}

func (g *GrpcPeer) PickPeer(key string) (PeerGetter, bool) {
	//TODO implement me
	panic("implement me")
}

func (g *GrpcPeer) StartService() {
	//TODO implement me
	panic("implement me")
}

type GrpcGetter struct {
	addr string
	name string
}

func (g *GrpcGetter) Get(group string, key string) ([]byte, error) {
	conn, err := grpc.Dial(g.addr)
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
	conn, err := grpc.Dial(g.addr)
	if err != nil {
		return err
	}
	client := pb.NewPeerClient(conn)
	client.Set()
}

func (g *GrpcGetter) Name() string {
	//TODO implement me
	panic("implement me")
}

func (g *GrpcGetter) Addr() string {
	//TODO implement me
	panic("implement me")
}
