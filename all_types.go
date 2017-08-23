// +build ignore

package genx

import "github.com/cheekybits/genny/generic"

type KT generic.Type
type VT interface{}

type Both struct {
	K KT
	V VT

	Call        func(k KT) VT
	RemoveMeToo int
}

func (b *Both) RemoveMe() {
	b.K = new(KT)
	b.V = new(VT)
}

func (b Both) RemoveMe2() {
	b.K = new(KT)
	b.V = new(VT)
}

func DoStuff(k KT) VT {
	var b Both
	return b.Call(k)
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
