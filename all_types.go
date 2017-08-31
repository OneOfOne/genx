// +build ignore

package genx

import "github.com/cheekybits/genny/generic"

type T generic.Type

type (
	KT interface{}
	VT interface{}
)
type TypeWithKT struct {
	K KT
	V VT

	Call     func(k KT) VT
	RemoveMe VT // RemoveMe comment

	Iface interface{}
}

// MethodWithPtr comment
func (b *TypeWithKT) MethodWithPtr() {
	b.K = new(KT)
	b.V = new(VT)
}

// MethodWithValue comment
func (b TypeWithKT) MethodWithValue() {
	b.K = new(KT)
	b.V = new(VT)
}

func DoParam(b *TypeWithKT)            {}
func DoRes() *TypeWithKT               { return nil }
func DoBoth(b *TypeWithKT) *TypeWithKT { return nil }

func DoStuff(k ...KT) VT {
	b := &TypeWithKT{
		RemoveMe: nil,
	}
	return b.Call(k[0])
}

func DoStuffTwo(k ...KT) VT {
	var b TypeWithKT
	return b.RemoveMe
}

func ReturnVT() VT {
	return nil
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
	mp   map[*TypeWithKT]interface{}
)

func XXX(vs ...interface{}) interface{} {
	return vs[0]
}
