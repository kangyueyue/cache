package kamacache

// ByteView 只读的字节视图，用于缓存数据
type ByteView struct {
	b []byte
}

// Len 返回字节长度
func (b ByteView) Len() int {
	return len(b.b)
}

// ByteSLice 返回字节
func (b ByteView) ByteSLice() []byte {
	return cloneBytes(b.b)
}

// String 返回字符串
func (b ByteView) String() string {
	return string(b.b)
}

// cloneBytes 返回字节的副本
func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
