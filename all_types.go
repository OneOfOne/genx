// +build ignore

package genx

import "github.com/cheekybits/genny/generic"

type KT generic.Type
type VT interface{}

type BothKT struct {
	K KT
	V VT

	Call        func(k KT) VT
	RemoveMeToo int // comment
}

// RemoveMe comment
func (b *BothKT) RemoveMe() {
	b.K = new(KT)
	b.V = new(VT)
}

// some comment
func (b BothKT) RemoveMe2() {
	b.K = new(KT)
	b.V = new(VT)
}

func DoStuff(k ...KT) VT {
	var b BothKT
	return b.Call(k[0])
}

var (
	m    map[KT]VT
	ktCh chan KT
	vtCh chan VT
	kvCh chan Both
	ktA  [100]KT
	vtA  [100]VT
	a    [100]Both
	ktS  []KT
	vtS  []VT
	s    []Both
)

func XXX(vs ...interface{}) interface{} {
	return vs[0]
}
