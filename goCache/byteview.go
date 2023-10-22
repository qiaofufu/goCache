package goCache

type ByteView struct {
	b []byte
}

func (b ByteView) Size() int {
	return len(b.b)
}

func (b ByteView) String() string {
	return string(b.b)
}

func (b ByteView) Slice() []byte {
	t := make([]byte, b.Size())
	copy(t, b.b)
	return t
}
