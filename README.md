# GenX : Generics For Go, Yet Again. [![GoDoc](https://godoc.org/github.com/OneOfOne/genx?status.svg)](https://godoc.org/github.com/OneOfOne/genx) [![Build Status](https://travis-ci.org/OneOfOne/genx.svg?branch=master)](https://travis-ci.org/OneOfOne/genx)

## Install

	go get github.com/OneOfOne/genx/...

## Features
* It can be *easily* used with `go generate`, from the command line or as a library.
* Uses local files, packages, and automatically uses `go get` if the remote package doesn't exist.
* You can rewrite, remove and change pretty much everything.
* Allows you to merge a package of multiple files into a single one.
* *Safely* remove functions and struct fields.
* Automatically passes all code through `x/tools/imports` (aka `goimports`).
* If you intend on generating files in the same package, you may add `// +build genx` to your template(s).
* Transparently handles [genny](https://github.com/cheekybits/genny)'s `generic.Type`.
* Supports a few [seeds](https://github.com/OneOfOne/genx/tree/master/seeds/).
* Adds build tags based on the types you pass, so you can target specifc types (ex: `// +build genx_kt_string` or `// +build genx_vt_builtin` )

## Examples:
### Package:

```
//go:generate genx -pkg ./internal/cmap -t KT=interface{} -t VT=interface{} -m -o ./cmap.go
//go:generate genx -pkg ./internal/cmap -n stringcmap -t KT=string -t VT=interface{} -fld HashFn -fn DefaultKeyHasher -s "cm.HashFn=hashers.Fnv32" -m -o ./stringcmap/cmap.go
```
* Input [cmap](https://github.com/OneOfOne):  [cmap.go](https://github.com/OneOfOne/cmap/blob/master/cmap.go) / [lmap.go](https://github.com/OneOfOne/cmap/blob/master/lmap.go)
* Merged output `map[interface{}]interface{}`: [cmap_iface_iface.go](https://github.com/OneOfOne/cmap/blob/master/cmap_iface_iface.go)
* Merged output `map[string]interface{}`: [stringcmap/cmap_string_iface.go](https://github.com/OneOfOne/cmap/blob/master/stringcmap/cmap_string_iface.go)

### Single File:
```bash
➤ genx -f github.com/OneOfOne/cmap/lmap.go -t "KT=string,VT=int" -fn "NewLMap,NewLMapSize=NewStringInt" -n main -v -o ./lmap_string_int.go
```


### Target native types with a fallback: [seeds/sort](https://github.com/OneOfOne/genx/tree/master/seeds/sort)

```
➤ genx -seed sort -t KT=string -n main
...
func StringSlice(s []string, reverse bool) { ... }
...
➤ genx -seed sort -t KT=*pkg.OtherType -n main
...
func PkgTypeSlice(s []string, less func(i, j int) bool) { ... }
...
```

### Sets: [seeds/set](https://github.com/OneOfOne/genx/tree/master/seeds/set)
```
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
```

* Command: `genx -seed set -t KeyType=string -fn Keys`

* Output:
```go
package set

type StringSet map[string]struct{}

func NewStringSet() StringSet { return StringSet{} }

func (s StringSet) Add(vals ...string) {
	for i := range vals {
		s[vals[i]] = struct{}{}
	}
}

func (s StringSet) Has(val string) (ok bool) {
	_, ok = s[val]
	return
}

func (s StringSet) Delete(vals ...string) {
	for i := range vals {
		delete(s, vals[i])
	}
}
```
## TODO
* Documentation.
* Add proper examples.
* Support package tests.
* Handle removing comments properly rather than using regexp.
* Common seeds.
* ~~Support specalized functions by type.~~
* ~~Support removing structs and their methods.~~

## Credits
* The excellent [astrewrite](https://github.com/fatih/astrewrite) library by [Fatih](https://github.com/fatih).

## BUGS
* Removing types / funcs doesn't always properly remove their comments.

## FAQ

### Why?
Mostly a learning experience, also I needed it and the other options available didn't do what I needed.

For Example I needed to remove a field from the struct and change all usage of it for [stringcmap](https://github.com/OneOfOne/cmap/tree/master/stringcmap).

## Usage:
```
➤ genx -h
usage: genx [-t T=type] [-s xx.xx=[yy.]yy] [-fld struct-field-to-remove] [-fn func-to-remove] [-tags "go build tags"]
  [-m] [-n package-name] [-pkg input package] [-f input file] [-o output file or dir]

Types:
  The -t/-s flags supports full package paths or short ones and letting goimports handle it.
  -t "KV=string
  -t "M=*cmap.CMap"
  -t "M=github.com/OneOfOne/cmap.*CMap"
  -s "cm.HashFn=github.com/OneOfOne/cmap/hashers#H.Fnv32"
  -s "cm.HashFn=github.com/OneOfOne/cmap/hashers.Fnv32"
  -s "cm.HashFn=hashers.Fnv32"
  -t "RemoveThisType"
  -fld "RemoveThisStructField,OtherField=NewFieldName"
  -fn "RemoveThisFunc,OtherFunc=NewFuncName"

Examples:
  genx -pkg github.com/OneOfOne/cmap -t "KT=interface{},VT=interface{}" -m -n cmap -o ./cmap.go
  genx -f github.com/OneOfOne/cmap/lmap.go -t "KT=string,VT=int" -fn "NewLMap,NewLMapSize=NewStringInt" -n main -o ./lmap_string_int.go

  genx -pkg github.com/OneOfOne/cmap -n stringcmap -t KT=string -t VT=interface{} -fld HashFn \
  -fn DefaultKeyHasher -s "cm.HashFn=hashers.Fnv32" -m -o ./stringcmap/cmap.go

Flags:
  -f file
    	file to parse
  -fld field
    	struct fields to remove or rename (ex: -fld HashFn -fld priv=Pub)
  -fn value
    	func`s to remove or rename (ex: -fn NotNeededFunc -fn Something=SomethingElse)
  -goFlags flags
    	extra go get flags (ex: -goFlags '-t -race')
  -m	merge all the files in a package into one
  -n package name
    	package name sets the output package name, uses input package name if empty.
  -o string
    	output dir if parsing a package or output filename if parsing a file (default "/dev/stdin")
  -pkg package
    	package to parse
  -s selector spec
    	selector specs to remove or rename (ex: -s 'cm.HashFn=hashers.Fnv32' -s 'x.Call=Something')
  -seed <seed>
    	alias for -m -pkg github.com/OneOfOne/seeds/<seed>
  -t type spec
    	generic type specs to remove or rename (ex: -t 'KV=string,KV=interface{}' -t RemoveThisType)
  -tags tags
    	go build tags, used for parsing and automatically passed to go get.
  -v	verbose
```

## Contributions
* All contributions are welcome, just open a pull request.

## License

Apache v2.0 (see [LICENSE](https://github.com/OneOfOne/genx/blob/master/LICENSE) file).

Copyright 2016-2017 Ahmed <[OneOfOne](https://github.com/OneOfOne/)> W.

	Licensed under the Apache License, Version 2.0 (the "License");
	you may not use this file except in compliance with the License.
	You may obtain a copy of the License at

		http://www.apache.org/licenses/LICENSE-2.0

	Unless required by applicable law or agreed to in writing, software
	distributed under the License is distributed on an "AS IS" BASIS,
	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
	See the License for the specific language governing permissions and
	limitations under the License.
