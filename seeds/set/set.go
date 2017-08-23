package set

type KT interface{}

type KTSet map[KT]struct{}

func NewKTSet() KTSet { return KTSet{} }

func (s KTSet) Set(vals ...KT) {
	for i := range vals {
		s[vals[i]] = struct{}{}
	}
}

func (s KTSet) Has(val KT) (ok bool) {
	_, ok = s[val]
	return
}

func (s KTSet) Unset(vals ...KT) {
	for i := range vals {
		delete(s, vals[i])
	}
}

func (s KTSet) Merge(o KTSet) {
	for k, v := range o {
		s[k] = v
	}
}

func (s KTSet) Keys() (out []KT) {
	out = make([]KT, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	return
}
