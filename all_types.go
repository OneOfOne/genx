// +build ignore
package all_types

type KT interface{}
type VT interface{}

type Both struct {
	K KT
	V VT
}

func (b *Both) RemoveMe() {
	b.K = KT{}
	b.V = VT{}
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
