# Seeds for GenX.

## Usage
```
➤ genx -seed set -t T=string -n main -o ./string_set.go
➤ genx -seed set -t T=uint64 -n main -o ./string_uint64.go

# or
➤ genx -seed atomicMap -t KT=string,VT=uint64 -n main -o ./amap_string_uint64.go
```
## Available Seeds

### **[set](https://github.com/OneOfOne/genx/tree/master/seeds/set)**
* A very simple `set` with `Set/Unset/Has/Merge/Keys()` support.
* Generate with: `genx -seeds set -t T=YourType -n package-name -o ./set_YourType.go`
* Usage: `s := NewYourTypeSet()`

### **[atomicValue](https://github.com/OneOfOne/genx/tree/master/seeds/atomicValue)**
* Typed `sync/atomic.Value` with `Swap`/`CompareAndSwap` support (using a `sync.RWMutex`).
* Generate with: `genx -seeds atomicValue -t T=YourType -n package-name -o ./atomic_YourType.go`
* Usage: `v := NewAtomicYourType(some initial value) or v := &AtomicYourType{}; old := v.Swap(new value)`

### **[atomicMap](https://github.com/OneOfOne/genx/tree/master/seeds/atomicMap)**
* The code generated from this seed is under The Go BSD-style [license](https://github.com/OneOfOne/genx/tree/master/seeds/atomicMap/LICENSE).
* A modified version of [sync.Map](https://tip.golang.org/pkg/sync/#Map) to support code gen.
* Generate with: `genx -seeds atomicMap -t KT=YourKeyType,VT=YourValueType -n package-name -o ./map_YourKeyType_YourValueType.go`
* Usage: `var m MapYourKeyTypeYourValueType; v := m.LoadOrStore(some key, some default value)`

### **[sort](https://github.com/OneOfOne/genx/tree/master/seeds/sort)**
* The code generated from this seed is under The Go BSD-style [license](https://github.com/OneOfOne/genx/tree/master/seeds/atomicMap/LICENSE).
* Shows how to target native types vs other types with tags.
* For builtin types, it uses [builtin-types.go](https://github.com/OneOfOne/genx/tree/master/seeds/sort/builtin-types.go).
* For other types, it expects a `func(i, j int) bool` func, and uses [other-types.go](https://github.com/OneOfOne/genx/tree/master/seeds/sort/other-types.go).
* A modified version of [sort.Slice/sort.SliceStable](https://tip.golang.org/pkg/sort/#Slice) to support code gen.
* Generate with: `genx -seeds sort -t T=YourType -n package-name -o ./sort_YourType.go`
* Usage with a builtin comparable type (`genx -seeds sort -t T=string ...`): `SortStrings(stringSlice, true or false for reverse sort)`.
* Usage with other types (`genx -seeds sort -t T=*YourType ...`):

```
SortYourType(yourSlice, func(i, j) bool { return yourSlice[i].X < yourSlice[j].X })
// for reverse sort
SortYourType(yourSlice, func(j, i) bool { return yourSlice[i].X < yourSlice[j].X })

```
## Todo

* Documention fort each seed for Godoc.
