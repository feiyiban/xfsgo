package xfsvm

type mem struct {
	data []byte
}

func NewMemory() *mem {
	return &mem{}
}
func (m *mem) Set(offset uint8, val []byte) {

}
