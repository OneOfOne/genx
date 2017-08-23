# genx : Generics For Go, Yet Again. [![GoDoc](https://godoc.org/github.com/OneOfOne/genx?status.svg)](https://godoc.org/github.com/OneOfOne/genx) [![Build Status](https://travis-ci.org/OneOfOne/genx.svg?branch=master)](https://travis-ci.org/OneOfOne/genx)

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

## Usage:
```
➤ genx -h
usage: genx [-t T=type] [-s xx.xx=[yy.]yy] [-fld struct-field-to-remove] [-fn func-to-remove] [-tags "go build tags"]
  [-m] [-n package-name] [-pkg input package] [-f input file] [-o output file or dir]

Types:
  The -t flag supports full package paths or short ones and letting goimports handle it.
  -t "KV=string
  -t "M=*cmap.CMap"
  -t "M=github.com/OneOfOne/cmap.*CMap"
  -s "cm.HashFn=github.com/OneOfOne/cmap/bad_pkg_name#hashers.Fnv32"
  -s "cm.HashFn=github.com/OneOfOne/cmap/hashers.Fnv32"
  -s "cm.HashFn=hashers.Fnv32"

Examples:
  genx -pkg github.com/OneOfOne/cmap/internal/cmap -t KT=interface{} -t VT=interface{} -m -n cmap -o ./cmap.go

  genx -pkg github.com/OneOfOne/cmap/internal/cmap -n stringcmap -t KT=string -t VT=interface{} -fld HashFn \
  -fn DefaultKeyHasher -s "cm.HashFn=hashers.Fnv32" -m -o ./stringcmap/cmap.go

Flags:
  -f string
        file to parse
  -fld value
        fields to remove from structs (ex: -fld HashFn)
  -fn value
        funcs to remove (ex: -fn NotNeededFunc)
  -getFlags string
        extra params to pass to go get, build tags are handled automatically.
  -m    merge all the files in a package into one
  -n string
        new package name
  -o string
        output dir if parsing a package or output filename if parsing a file
  -pkg string
        package to parse
  -s value
        selectors to remove or rename (ex: -s "cm.HashFn=hashers.Fnv32" -s "x.Call=Something")
  -t value
        generic types (ex: -t KV=string -t "KV=interface{}" -t RemoveThisType)
  -tags value
        go build tags, used for parsing
```

## Examples:
### cmap

```
//go:generate genx -pkg ./internal/cmap -t KT=interface{} -t VT=interface{} -m -o ./cmap.go
//go:generate genx -pkg ./internal/cmap -n stringcmap -t KT=string -t VT=interface{} -fld HashFn -fn DefaultKeyHasher -s "cm.HashFn=hashers.Fnv32" -m -o ./stringcmap/cmap.go
```
* Input: [package](https://github.com/OneOfOne/cmap/tree/9f7d890077bc1925df41083777b5c448d2931a9a/internal/cmap)
* Merged output: [`map[interface{}]interface{}`](https://github.com/OneOfOne/cmap/blob/9f7d890077bc1925df41083777b5c448d2931a9a/cmap.go)
* Merged output: [`map[string]interface{}`](https://github.com/OneOfOne/cmap/blob/9f7d890077bc1925df41083777b5c448d2931a9a/stringcmap/cmap.go)

## TODO
* Specialized functions (~ `func (t *T) XXIfKTIsString() -> func(t *T) XX()` if KT is string)
* Documentation.
* Add proper examples.
* Support package tests.
* Handle removing comments properly rather than using regexp.
* Support removing structs and their methods.

## Credits
* The excellent [astrewrite](https://github.com/fatih/astrewrite) library by [Fatih](https://github.com/fatih).

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
