package buffer

import "sync"

var lineBufPool = sync.Pool{
	New: func() any {
		return &LineBufPooled{LineBuf: NewLineBuf()}
	},
}

type LineBufPooled struct {
	*LineBuf
}

func NewLineBufPooled() *LineBufPooled {
	return lineBufPool.Get().(*LineBufPooled) //nolint:forcetypeassert
}

func (r *LineBufPooled) Release() {
	r.buf = r.buf[:0]
	lineBufPool.Put(r)
}
