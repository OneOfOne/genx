# Seeds for GenX.

## Usage
```
➤ genx -seed set -t KT=string -n main -o ./string_set.go
➤ genx -seed set -t KT=uint64 -n main -o ./string_uint64.go

# or
➤ genx -seed atomicMap -t KT=string,VT=uint64 -n main -o ./amap_string_uint64.go
```
## Available Seeds

### **[set](https://github.com/OneOfOne/genx/tree/master/seeds/set)**
* Very simple `set` with `Set/Unset/Has/Merge/Keys()` support.

### **[atomicMap](https://github.com/OneOfOne/genx/tree/master/seeds/atomicMap)**
* A modified version of [sync.Map](https://tip.golang.org/pkg/sync/#Map) to support code gen.
* The code generated from this seed is under The Go BSD-style [license](https://github.com/OneOfOne/genx/tree/master/seeds/atomicMap/LICENSE).

### **[sort](https://github.com/OneOfOne/genx/tree/master/seeds/sort)**
* Shows how to target native types vs other types with tags.
* For types with native `< (less than)` support (string, int, float*), it uses [builtin-types.go](https://github.com/OneOfOne/genx/tree/master/seeds/sort/builtin-types.go).
* For other types, it expects a `func(i, j int) bool` func.
* A modified version of [sort.Slice/sort.SliceStable](https://tip.golang.org/pkg/sort/#Slice) to support code gen.
* The code generated from this seed is under The Go BSD-style [license](https://github.com/OneOfOne/genx/tree/master/seeds/atomicMap/LICENSE).

### More to come.
