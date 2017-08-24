package atomicValue

import "sync"

type T interface{}

// NewAtomicT returns a new atomic value with an initial value.
func NewAtomicT(initial T) *AtomicT { return &AtomicT{v: initial} }

// AtomicT represents a specialized atomic value, more like sync/atomic.Value with Swap/CompareAndSwap support.
// Must not be copied.
type AtomicT struct {
	m sync.RWMutex
	v T
}

func (a *AtomicT) Load() (v T) {
	a.m.RLock()
	v = a.v
	a.m.RUnlock()
	return
}

func (a *AtomicT) Store(v T) {
	a.m.Lock()
	a.v = v
	a.m.Unlock()
}

func (a *AtomicT) Swap(newV T) (oldV T) {
	a.m.Lock()
	oldV, a.v = a.v, newV
	a.m.Unlock()
	return
}

func (a *AtomicT) CompareAndSwap(fn func(oldV T) (newV T, ok bool)) (ok bool) {
	var newV T
	a.m.Lock()
	defer a.m.Unlock()
	if newV, ok = fn(a.v); ok {
		a.v = newV
	}
	return
}
