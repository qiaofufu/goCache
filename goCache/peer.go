package goCache

type Peer interface {
	PeerPicker
	StartService(addr string)
}

// PeerPicker 对等体选择接口
type PeerPicker interface {
	PickPeer(key string) (PeerGetter, bool)
	SetPeerGetter(getters ...PeerGetter)
}

// PeerGetter 对等体交互发送端
type PeerGetter interface {
	Get(group string, key string) ([]byte, error)
	Remove(group string, key string) error
	Name() string // 名字
	Weight() int  // 权重
	Addr() string // 地址
}
