package set

type T interface{}

type TSet map[T]struct{}

func NewTSet() TSet { return TSet{} }

func (s TSet) Set(vals ...T) {
	for i := range vals {
		s[vals[i]] = struct{}{}
	}
}

func (s TSet) Has(val T) (ok bool) {
	_, ok = s[val]
	return
}

func (s TSet) Unset(vals ...T) {
	for i := range vals {
		delete(s, vals[i])
	}
}

func (s TSet) Merge(o TSet) {
	for k, v := range o {
		s[k] = v
	}
}

func (s TSet) Keys() (out []T) {
	out = make([]T, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	return
}
