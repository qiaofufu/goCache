package goCache

import clientv3 "go.etcd.io/etcd/client/v3"

type Peer interface {
	Register
	Discovery
	PeerPicker
	StartService()
}

// PeerPicker 对等体选择接口
type PeerPicker interface {
	PickPeer(key string) (PeerGetter, bool)
}

// PeerGetter 对等体交互发送端
type PeerGetter interface {
	Get(group string, key string) ([]byte, error)
	Remove(group string, key string) error
	Name() string // 名字
	Addr() string // 地址
}

type Discovery interface {
	Discovery(prefix string)                   // 服务发现
	watch(cli *clientv3.Client, prefix string) // 对注册中心进行监听
	SetService(key string, value string)       // 设置服务节点
	DelService(key string)                     // 删除服务节点
}

type Register interface {
	Register(prefix string, leaseExpire int64) // 注册服务
}
