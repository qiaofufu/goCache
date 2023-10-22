package consistent

import (
	"testing"
)

func TestConsistentHash(t *testing.T) {
	// 创建一致性哈希对象
	ch := New(50, nil)

	// 添加节点
	node1 := Node{Name: "node1", Addr: "localhost:8001", Weight: 1}
	node2 := Node{Name: "node2", Addr: "localhost:8002", Weight: 1}
	node3 := Node{Name: "node3", Addr: "localhost:8003", Weight: 1}
	ch.AddNodes(node1, node2, node3)

	// 测试 GetNode 方法
	key1 := "user123"
	node, err := ch.GetNode(key1)
	if err != nil {
		t.Errorf("GetNode failed: %v", err)
	}
	if node.Name != "node3" {
		t.Errorf("GetNode returned the wrong node for key1")
	}

	key2 := "product456"
	node, err = ch.GetNode(key2)
	if err != nil {
		t.Errorf("GetNode failed: %v", err)
	}
	if node.Name != "node1" {
		t.Errorf("GetNode returned the wrong node for key2")
	}
}

func TestConsistent_GetNode(t *testing.T) {
	consistent := New(0, nil)
	node, err := consistent.GetNode("test")
	if err == nil {
		t.Fatalf("ring is nil, but get node success, node: %v", node)
	}
}
