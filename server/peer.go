package server

import "time"

type Peer interface {
	StartPeerServer(addr string, peerAddr ...string) // 启动服务
	Picker
}

type Picker interface {
	PickerPeer(key string) (Getter, error) // 根据key获取相应的Getter
	AddGetter(getter Getter)               // 添加Getter
	AddGetters(getters ...Getter)          // 添加Getter
	UpdateGetter(getter Getter)            // 更新Getter
	RemoveGetter(getterKey string)         // 根据key删除getter
}

// Getter 用于封装对其他peer发送action
type Getter interface {
	Get(namespace string, key string) ([]byte, error)                           // 根据namespace 和 key 获取数据
	Set(namespace string, key string, value []byte, expire time.Duration) error // 设置数据
	Remove(namespace string, key string) error                                  // 根据namespace 和 key 删除数据
	Addr() string                                                               // 获取getter Addr
	Name() string                                                               // 获取getter Name
}
