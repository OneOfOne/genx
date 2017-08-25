package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/OneOfOne/genx"
)

type sflags []string

func (sf *sflags) String() string {
	return strings.Join(*sf, " ")
}

func (sf *sflags) Set(value string) error {
	parts := strings.Split(value, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	*sf = append(*sf, parts...)
	return nil
}

func (sf *sflags) Split(i int) (_, _ string) {
	parts := strings.Split((*sf)[i], "=")
	switch len(parts) {
	case 2:
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	case 1:
		return strings.TrimSpace(parts[0]), ""
	default:
		return
	}
}

var (
	types, fields, selectors, funcs, tags sflags

	seed, inFile, inPkg, outPath string

	goFlags    string
	pkgName    string
	mergeFiles bool
	goGet      bool
	verbose    bool
)

func init() {
	log.SetFlags(log.Lshortfile)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, `usage: genx [-t T=type] [-s xx.xx=[yy.]yy] [-fld struct-field-to-remove] [-fn func-to-remove] [-tags "go build tags"]
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

Flags:`)
		flag.PrintDefaults()
	}
	flag.Var(&types, "t", "generic `type spec`s to remove or rename (ex: -t 'KV=string,KV=interface{}' -t RemoveThisType)")
	flag.Var(&selectors, "s", "`selector spec`s to remove or rename (ex: -s 'cm.HashFn=hashers.Fnv32' -s 'x.Call=Something')")
	flag.Var(&fields, "fld", "struct `field`s to remove or rename (ex: -fld HashFn -fld priv=Pub)")
	flag.Var(&funcs, "fn", "func`s to remove or rename (ex: -fn NotNeededFunc -fn Something=SomethingElse)")
	flag.Var(&tags, "tags", "go build `tags`, used for parsing and automatically passed to go get.")
	flag.StringVar(&seed, "seed", "", "alias for -m -pkg github.com/OneOfOne/seeds/`<seed>`")
	flag.StringVar(&inFile, "f", "", "`file` to parse")
	flag.StringVar(&inPkg, "pkg", "", "`package` to parse")
	flag.StringVar(&outPath, "o", "/dev/stdin", "output dir if parsing a package or output filename if parsing a file")
	flag.StringVar(&pkgName, "n", "", "`package name` sets the output package name, uses input package name if empty.")
	flag.StringVar(&goFlags, "goFlags", "", "extra go get `flags` (ex: -goFlags '-t -race')")
	flag.BoolVar(&verbose, "v", false, "verbose")
	flag.BoolVar(&goGet, "get", false, "go get the package if it doesn't exist")

	flag.Parse()
}

func main() {
	rewriters := map[string]string{}
	for i := range types {
		key, val := types.Split(i)
		if key == "" {
			continue
		}
		if val == "" {
			val = "-"
		}
		rewriters["type:"+key] = val
	}
	for i := range selectors {
		key, val := selectors.Split(i)
		if key == "" {
			continue
		}
		if val == "" {
			val = "-"
		}
		rewriters["selector:"+key] = val
	}
	for i := range fields {
		key, val := fields.Split(i)
		if key == "" {
			continue
		}
		if val == "" {
			val = "-"
		}
		rewriters["field:"+key] = val
	}
	for i := range funcs {
		key, val := funcs.Split(i)
		if key == "" {
			continue
		}
		if val == "" {
			val = "-"
		}
		rewriters["func:"+key] = val
	}

	g := genx.New(pkgName, rewriters)
	g.BuildTags = append(g.BuildTags, tags...)

	if verbose {
		log.Printf("rewriters: %+q", g.OrderedRewriters())
		log.Printf("build tags: %+q", g.BuildTags)
	}

	switch outPath {
	case "", "-", "/dev/stdout":
		outPath = "/dev/stdout"
		mergeFiles = true
	}

	// auto merge files if the output is a file not a dir.
	mergeFiles = !mergeFiles && filepath.Ext(outPath) == ".go"

	if seed != "" {
		inPkg = "github.com/OneOfOne/genx/seeds/" + seed
		mergeFiles = true
	}

	if inPkg != "" {
		out, err := goListThenGet(g.BuildTags, inPkg)
		if err != nil {
			log.Fatalf("error: %s\n", out)
		}
		inPkg = out
		// if !strings.HasPrefix(inpk, prefix string)
		pkg, err := g.ParsePkg(inPkg, false)
		if err != nil {
			log.Fatalf("error parsing package (%s): %v\n", inPkg, err)
		}

		if mergeFiles {
			if err := pkg.WriteAllMerged(outPath, false); err != nil {
				log.Fatalf("error writing merged package: %v", err)
			}
		} else {
			if err := pkg.WritePkg(outPath); err != nil {
				log.Fatalf("error writing merged package: %v", err)
			}
		}
		return
	}

	switch inFile {
	case "", "-":
	default:
		out, err := goListThenGet(g.BuildTags, inFile)
		if err != nil {
			log.Fatalf("error:\n%s\n", out)
		}

		pf, err := g.Parse(out, nil)
		if err != nil {
			log.Fatalf("error parsing file (%s): %v\n%s", inFile, err, pf.Src)
		}
		if err := pf.WriteFile(outPath); err != nil {
			log.Fatalf("error writing file: %v", err)
		}
	}
}

func execCmd(c string, args ...string) (string, error) {
	cmd := exec.Command(c, args...)
	if verbose {
		log.Printf("executing: %s %s", c, strings.Join(args, " "))
	}
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func goListThenGet(tags []string, path string) (out string, err error) {
	if _, err = os.Stat(path); err == nil {
		return path, nil
	}

	isFile := filepath.Ext(path) == ".go"
	dir := path
	if isFile {
		dir = filepath.Dir(path)
	}

	args := []string{"-tags", strings.Join(tags, " ")}
	if goFlags != "" {
		args = append(args, strings.Split(goFlags, " ")...)
	}

	args = append(args, dir)

	listArgs := append([]string{"list", "-f", "{{.Dir}}"}, args...)

	if out, err = execCmd("go", listArgs...); err != nil && strings.Contains(out, "cannot find package") {
		if !goGet {
			out = fmt.Sprintf("`%s` not found and `-get` isn't specified.", path)
			return
		}
		if out, err = execCmd("go", append([]string{"get", "-u", "-v"}, args...)...); err == nil && isFile {
			out, err = execCmd("go", listArgs...)
		}
	}

	if err == nil && isFile {
		out = filepath.Join(out, filepath.Base(path))
	}
	return
}

/*
go run cmd/genx/main.go -t KT=string -t "VT=interface{}" -rm-field HashFn -s "cm.HashFn=hashers.Fnv32" -s "cmap=-" -pkg ../cmap/internal/cmap/ -
	 n stringcmap -m
*/
