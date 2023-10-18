package communication

type Peer interface {
	PickPeer(key string) Peer          // 根据key选择peer
	SetPeerGetter(peers ...PeerGetter) // 设置PeerGetter
	GetAllGetter() []PeerGetter        // 获取所有PeerGetter
	Start()
}

// PeerGetter 用于封装对其他peer发送action
type PeerGetter interface {
	Get(namespace string, key string) ([]byte, error) // 根据namespace 和 key 获取数据
	Remove(namespace string, key string) error        // 根据namespace 和 key 删除数据
}
