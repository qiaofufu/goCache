package consistent

import (
	"fmt"
	"hash/crc32"
	"math"
	"sort"
	"sync"
)

const (
	defaultReplicas = 50
)

type HashFunc func(data []byte) uint32

type Node struct {
	val    int    // val 节点hash值
	Name   string // Name 节点名称
	Addr   string // Addr 节点地址
	Weight int32  // Weight 节点权重
}

type Consistent struct {
	hash     HashFunc        // hash 计算函数
	replicas int             // 虚拟节点个数
	ring     []Node          // hash 环
	mp       map[string]Node // 真实node记录
	mu       sync.RWMutex    // 读写锁控制ring
}

func New(replicas int, fn HashFunc) *Consistent {
	c := &Consistent{
		hash:     fn,
		replicas: replicas,
		mp:       make(map[string]Node),
	}
	if c.replicas <= 0 {
		c.replicas = defaultReplicas
	}
	if c.hash == nil {
		c.hash = crc32.ChecksumIEEE
	}
	return c
}

func (c *Consistent) adjust() {
	var totalWeight int32
	for _, v := range c.mp {
		totalWeight += v.Weight
	}
	totalVirtualSpots := c.replicas * len(c.mp)

	c.ring = make([]Node, 0, totalVirtualSpots)

	for k, node := range c.mp {
		spots := int(math.Floor(float64(node.Weight) / float64(totalWeight) * float64(totalVirtualSpots)))
		for i := 1; i <= spots; i++ {
			hashVal := c.hash([]byte(fmt.Sprintf("%s:%d", k, i)))
			n := Node{
				Name:   k,
				val:    int(hashVal),
				Addr:   node.Addr,
				Weight: node.Weight,
			}
			c.ring = append(c.ring, n)
		}
	}

	sort.Slice(c.ring, func(i, j int) bool {
		return c.ring[i].val < c.ring[j].val
	})
}

func (c *Consistent) AddNodes(nodes ...Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, node := range nodes {
		c.mp[node.Name] = node
	}
	c.adjust()
}

func (c *Consistent) AddNode(node Node) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mp[node.Name] = node
	c.adjust()
}

func (c *Consistent) DelNode(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.mp, name)
	c.adjust()
}

func (c *Consistent) GetNode(key string) (Node, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(key) == 0 {
		return Node{}, fmt.Errorf("key cannot be nil")
	}

	if len(c.ring) == 0 {
		return Node{}, fmt.Errorf("ring is nil")
	}

	hashVal := int(c.hash([]byte(key)))

	idx := sort.Search(len(c.ring), func(mid int) bool {
		return c.ring[mid].val >= hashVal
	})

	return c.ring[idx%len(c.ring)], nil
}
