package set

type KeyType interface{}

type KeyTypeSet map[KeyType]struct{}

func NewKeyTypeSet() KeyTypeSet { return KeyTypeSet{} }

func (s KeyTypeSet) Add(vals ...KeyType) {
	for i := range vals {
		s[vals[i]] = struct{}{}
	}
}

func (s KeyTypeSet) Has(val KeyType) (ok bool) {
	_, ok = s[val]
	return
}

func (s KeyTypeSet) Delete(vals ...KeyType) {
	for i := range vals {
		delete(s, vals[i])
	}
}

func (s KeyTypeSet) Keys() (out []KeyType) {
	out = make([]KeyType, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	return
}
