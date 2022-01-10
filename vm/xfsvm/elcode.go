package xfsvm

type elheader struct {
	magic uint32
	mType uint8
}

type symtab struct {
}

type elcode struct {
	*elheader
	*symtab
}
