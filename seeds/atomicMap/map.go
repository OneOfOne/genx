// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// atomicMap is a seed based on sync.Map in the stdlib.
package atomicMap

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

type (
	KT interface{}
	VT interface{}
)

// MapKTVT is a concurrent map with amortized-constant-time loads, stores, and deletes.
// It is safe for multiple goroutines to call a MapKTVT's methods concurrently.
//
// It is optimized for use in concurrent loops with keys that are
// stable over time, and either few steady-state stores, or stores
// localized to one goroutine per key.
//
// For use cases that do not share these attributes, it will likely have
// comparable or worse performance and worse type safety than an ordinary
// map paired with a read-write mutex.
//
// The nil MapKTVT is valid and empty.
//
// A MapKTVT must not be copied after first use.
type MapKTVT struct {
	mu sync.Mutex

	// read contains the portion of the map's contents that are safe for
	// concurrent access (with or without mu held).
	//
	// The read field itself is always safe to load, but must only be stored with
	// mu held.
	//
	// Entries stored in read may be updated concurrently without mu, but updating
	// a previously-expungedVT entryVT requires that the entryVT be copied to the dirty
	// map and unexpungedVT with mu held.
	read atomic.Value // readOnly

	// dirty contains the portion of the map's contents that require mu to be
	// held. To ensure that the dirty map can be promoted to the read map quickly,
	// it also includes all of the non-expungedVT entries in the read map.
	//
	// ExpungedVT entries are not stored in the dirty map. An expungedVT entryVT in the
	// clean map must be unexpungedVT and added to the dirty map before a new value
	// can be stored to it.
	//
	// If the dirty map is nil, the next write to the map will initialize it by
	// making a shallow copy of the clean map, omitting stale entries.
	dirty map[KT]*entryVT

	// misses counts the number of loads since the read map was last updated that
	// needed to lock mu to determine whether the key was present.
	//
	// Once enough misses have occurred to cover the cost of copying the dirty
	// map, the dirty map will be promoted to the read map (in the unamended
	// state) and the next store to the map will make a new dirty copy.
	misses int
}

// readOnlyKTVT is an immutable struct stored atomically in the Map.read field.
type readOnlyKTVT struct {
	m       map[KT]*entryVT
	amended bool // true if the dirty map contains some key not in m.
}

// expungedVT is an arbitrary pointer that marks entries which have been deleted
// from the dirty map.
var expungedVT = unsafe.Pointer(new(VT))

// An entryVT is a slot in the map corresponding to a particular key.
type entryVT struct {
	// p points to the interface{} value stored for the entryVT.
	//
	// If p == nil, the entryVT has been deleted and m.dirty == nil.
	//
	// If p == expungedVT, the entryVT has been deleted, m.dirty != nil, and the entryVT
	// is missing from m.dirty.
	//
	// Otherwise, the entryVT is valid and recorded in m.read.m[key] and, if m.dirty
	// != nil, in m.dirty[key].
	//
	// An entryVT can be deleted by atomic replacement with nil: when m.dirty is
	// next created, it will atomically replace nil with expungedVT and leave
	// m.dirty[key] unset.
	//
	// An entryVT's associated value can be updated by atomic replacement, provided
	// p != expungedVT. If p == expungedVT, an entryVT's associated value can be updated
	// only after first setting m.dirty[key] = e so that lookups using the dirty
	// map find the entryVT.
	p unsafe.Pointer // *interface{}
}

func newEntryVT(i VT) *entryVT {
	return &entryVT{p: unsafe.Pointer(&i)}
}

// Load returns the value stored in the map for a key, or nil if no
// value is present.
// The ok result indicates whether value was found in the map.
func (m *MapKTVT) Load(key KT) (value VT, ok bool) {
	read, _ := m.read.Load().(readOnlyKTVT)
	e, ok := read.m[key]
	if !ok && read.amended {
		m.mu.Lock()
		// Avoid reporting a spurious miss if m.dirty got promoted while we were
		// blocked on m.mu. (If further loads of the same key will not miss, it's
		// not worth copying the dirty map for this key.)
		read, _ = m.read.Load().(readOnlyKTVT)
		e, ok = read.m[key]
		if !ok && read.amended {
			e, ok = m.dirty[key]
			// Regardless of whether the entryVT was present, record a miss: this key
			// will take the slow path until the dirty map is promoted to the read
			// map.
			m.missLocked()
		}
		m.mu.Unlock()
	}
	if !ok {
		return nil, false
	}
	return e.load()
}

func (e *entryVT) load() (value VT, ok bool) {
	p := atomic.LoadPointer(&e.p)
	if p == nil || p == expungedVT {
		return nil, false
	}
	return *(*VT)(p), true
}

// Store sets the value for a key.
func (m *MapKTVT) Store(key KT, value VT) {
	read, _ := m.read.Load().(readOnlyKTVT)
	if e, ok := read.m[key]; ok && e.tryStore(&value) {
		return
	}

	m.mu.Lock()
	read, _ = m.read.Load().(readOnlyKTVT)
	if e, ok := read.m[key]; ok {
		if e.unexpungeLocked() {
			// The entryVT was previously expungedVT, which implies that there is a
			// non-nil dirty map and this entryVT is not in it.
			m.dirty[key] = e
		}
		e.storeLocked(&value)
	} else if e, ok := m.dirty[key]; ok {
		e.storeLocked(&value)
	} else {
		if !read.amended {
			// We're adding the first new key to the dirty map.
			// Make sure it is allocated and mark the read-only map as incomplete.
			m.dirtyLocked()
			m.read.Store(readOnlyKTVT{m: read.m, amended: true})
		}
		m.dirty[key] = newEntryVT(value)
	}
	m.mu.Unlock()
}

// tryStore stores a value if the entryVT has not been expungedVT.
//
// If the entryVT is expungedVT, tryStore returns false and leaves the entryVT
// unchanged.
func (e *entryVT) tryStore(i *VT) bool {
	p := atomic.LoadPointer(&e.p)
	if p == expungedVT {
		return false
	}
	for {
		if atomic.CompareAndSwapPointer(&e.p, p, unsafe.Pointer(i)) {
			return true
		}
		p = atomic.LoadPointer(&e.p)
		if p == expungedVT {
			return false
		}
	}
}

// unexpungeLocked ensures that the entryVT is not marked as expungedVT.
//
// If the entryVT was previously expungedVT, it must be added to the dirty map
// before m.mu is unlocked.
func (e *entryVT) unexpungeLocked() (wasExpungedVT bool) {
	return atomic.CompareAndSwapPointer(&e.p, expungedVT, nil)
}

// storeLocked unconditionally stores a value to the entryVT.
//
// The entryVT must be known not to be expungedVT.
func (e *entryVT) storeLocked(i *VT) {
	atomic.StorePointer(&e.p, unsafe.Pointer(i))
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *MapKTVT) LoadOrStore(key KT, value VT) (actual VT, loaded bool) {
	// Avoid locking if it's a clean hit.
	read, _ := m.read.Load().(readOnlyKTVT)
	if e, ok := read.m[key]; ok {
		actual, loaded, ok = e.tryLoadOrStore(value)
		if ok {
			return actual, loaded
		}
	}

	m.mu.Lock()
	read, _ = m.read.Load().(readOnlyKTVT)
	if e, ok := read.m[key]; ok {
		if e.unexpungeLocked() {
			m.dirty[key] = e
		}
		actual, loaded, _ = e.tryLoadOrStore(value)
	} else if e, ok := m.dirty[key]; ok {
		actual, loaded, _ = e.tryLoadOrStore(value)
		m.missLocked()
	} else {
		if !read.amended {
			// We're adding the first new key to the dirty map.
			// Make sure it is allocated and mark the read-only map as incomplete.
			m.dirtyLocked()
			m.read.Store(readOnlyKTVT{m: read.m, amended: true})
		}
		m.dirty[key] = newEntryVT(value)
		actual, loaded = value, false
	}
	m.mu.Unlock()

	return actual, loaded
}

// tryLoadOrStore atomically loads or stores a value if the entryVT is not
// expungedVT.
//
// If the entryVT is expungedVT, tryLoadOrStore leaves the entryVT unchanged and
// returns with ok==false.
func (e *entryVT) tryLoadOrStore(i VT) (actual VT, loaded, ok bool) {
	p := atomic.LoadPointer(&e.p)
	if p == expungedVT {
		return nil, false, false
	}
	if p != nil {
		return *(*VT)(p), true, true
	}

	// Copy the interface after the first load to make this method more amenable
	// to escape analysis: if we hit the "load" path or the entryVT is expungedVT, we
	// shouldn't bother heap-allocating.
	ic := i
	for {
		if atomic.CompareAndSwapPointer(&e.p, nil, unsafe.Pointer(&ic)) {
			return i, false, true
		}
		p = atomic.LoadPointer(&e.p)
		if p == expungedVT {
			return nil, false, false
		}
		if p != nil {
			return *(*VT)(p), true, true
		}
	}
}

// Delete deletes the value for a key.
func (m *MapKTVT) Delete(key KT) {
	read, _ := m.read.Load().(readOnlyKTVT)
	e, ok := read.m[key]
	if !ok && read.amended {
		m.mu.Lock()
		read, _ = m.read.Load().(readOnlyKTVT)
		e, ok = read.m[key]
		if !ok && read.amended {
			delete(m.dirty, key)
		}
		m.mu.Unlock()
	}
	if ok {
		e.delete()
	}
}

func (e *entryVT) delete() (hadValue bool) {
	for {
		p := atomic.LoadPointer(&e.p)
		if p == nil || p == expungedVT {
			return false
		}
		if atomic.CompareAndSwapPointer(&e.p, p, nil) {
			return true
		}
	}
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
//
// Range does not necessarily correspond to any consistent snapshot of the Map's
// contents: no key will be visited more than once, but if the value for any key
// is stored or deleted concurrently, Range may reflect any mapping for that key
// from any point during the Range call.
//
// Range may be O(N) with the number of elements in the map even if f returns
// false after a constant number of calls.
func (m *MapKTVT) Range(f func(key KT, value VT) bool) {
	// We need to be able to iterate over all of the keys that were already
	// present at the start of the call to Range.
	// If read.amended is false, then read.m satisfies that property without
	// requiring us to hold m.mu for a long time.
	read, _ := m.read.Load().(readOnlyKTVT)
	if read.amended {
		// m.dirty contains keys not in read.m. Fortunately, Range is already O(N)
		// (assuming the caller does not break out early), so a call to Range
		// amortizes an entire copy of the map: we can promote the dirty copy
		// immediately!
		m.mu.Lock()
		read, _ = m.read.Load().(readOnlyKTVT)
		if read.amended {
			read = readOnlyKTVT{m: m.dirty}
			m.read.Store(read)
			m.dirty = nil
			m.misses = 0
		}
		m.mu.Unlock()
	}

	for k, e := range read.m {
		v, ok := e.load()
		if !ok {
			continue
		}
		if !f(k, v) {
			break
		}
	}
}

func (m *MapKTVT) missLocked() {
	m.misses++
	if m.misses < len(m.dirty) {
		return
	}
	m.read.Store(readOnlyKTVT{m: m.dirty})
	m.dirty = nil
	m.misses = 0
}

func (m *MapKTVT) dirtyLocked() {
	if m.dirty != nil {
		return
	}

	read, _ := m.read.Load().(readOnlyKTVT)
	m.dirty = make(map[KT]*entryVT, len(read.m))
	for k, e := range read.m {
		if !e.tryExpungeLocked() {
			m.dirty[k] = e
		}
	}
}

func (e *entryVT) tryExpungeLocked() (isExpungedVT bool) {
	p := atomic.LoadPointer(&e.p)
	for p == nil {
		if atomic.CompareAndSwapPointer(&e.p, nil, expungedVT) {
			return true
		}
		p = atomic.LoadPointer(&e.p)
	}
	return p == expungedVT
}
