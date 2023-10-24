package goCache

import "testing"

func TestGrpcPeer_StartService(t *testing.T) {
	var (
		etcdAddr = "http://162.14.115.114:2379"
		addr     = "localhost:8000"
	)
	peer := NewGrpcPeer(addr, etcdAddr)
	peer.StartService()
}
