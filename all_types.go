// +build ignore

package genx

import "github.com/cheekybits/genny/generic"

type KT generic.Type
type VT interface{}

type TypeWithKT struct {
	K KT
	V VT

	Call        func(k KT) VT
	RemoveMeToo VT // comment
}

// RemoveMe comment
func (b *TypeWithKT) RemoveMe() {
	b.K = new(KT)
	b.V = new(VT)
}

// some comment
func (b TypeWithKT) RemoveMe2() {
	b.K = new(KT)
	b.V = new(VT)
}

func DoParam(b *TypeWithKT)            {}
func DoRes() *TypeWithKT               { return nil }
func DoBoth(b *TypeWithKT) *TypeWithKT { return nil }

func DoStuff(k ...KT) VT {
	b := &TypeWithKT{}
	return b.Call(k[0])
}

func DoStuff2(k ...KT) VT {
	var b TypeWithKT
	return b.RemoveMe2
}

var (
	m    map[KT]VT
	ktCh chan KT
	vtCh chan VT
	kvCh chan TypeWithKT
	ktA  [100]KT
	vtA  [100]VT
	a    [100]TypeWithKT
	ktS  []KT
	vtS  []VT
	s    []*TypeWithKT
	m    map[TypeWithKT]int
	mp   map[*TypeWithKT]int
)

func XXX(vs ...interface{}) interface{} {
	return vs[0]
}
