package server

type ByteView struct {
	b []byte
}

func (b ByteView) Size() int {
	return len(b.b)
}
