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
	*sf = append(*sf, value)
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

	inFile, inPkg, outPath string

	getFlags   string
	pkgName    string
	mergeFiles bool
)

func init() {
	log.SetFlags(log.Lshortfile)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, `usage: genx [-t T=type] [-s xx.xx=[yy.]yy] [-fld struct-field-to-remove] [-fn func-to-remove] [-tags "go build tags"]
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

Flags:`)
		flag.PrintDefaults()
	}
	flag.Var(&types, "t", `generic types (ex: -t KV=string -t "KV=interface{}" -t RemoveThisType)`)
	flag.Var(&selectors, "s", `selectors to remove or rename (ex: -s "cm.HashFn=hashers.Fnv32" -s "x.Call=Something")`)
	flag.Var(&fields, "fld", `fields to remove or rename from structs (ex: -fld HashFn -fld priv=Pub)`)
	flag.Var(&funcs, "fn", `funcs to remove or rename (ex: -fn NotNeededFunc -fn New=NewStringIface)`)
	flag.Var(&tags, "tags", `go build tags, used for parsing`)
	flag.StringVar(&inFile, "f", "", "file to parse")
	flag.StringVar(&inPkg, "pkg", "", "package to parse")
	flag.StringVar(&outPath, "o", "/dev/stdin", "output dir if parsing a package or output filename if parsing a file")
	flag.StringVar(&pkgName, "n", "", "new package name")
	flag.BoolVar(&mergeFiles, "m", false, "merge all the files in a package into one")
	flag.StringVar(&getFlags, "getFlags", "", "extra params to pass to go get, build tags are handled automatically.")

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

	log.Printf("%+q", rewriters)
	g := genx.New(pkgName, rewriters)
	g.BuildTags = append(g.BuildTags, tags...)

	switch outPath {
	case "", "-":
		outPath = "/dev/stdout"
		fallthrough
	case "/dev/stdout":
		mergeFiles = true
	}

	if inPkg != "" {
		out, err := execCmd("go", "list", "-f", "{{.Dir}}", inPkg)
		if err != nil && strings.Contains(out, "cannot find package") {
			args := []string{"get", "-tags", strings.Join(g.BuildTags, " ")}
			if gf := strings.Split(getFlags, " "); len(gf) > 0 {
				args = append(args, gf...)
			}
			args = append(args, inPkg)
			log.Printf("executing go %q", args)
			out, err = execCmd("go", args...)
		}

		if err != nil {
			log.Fatalf("error:\n%s\n", out)
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
		out, err := execCmd("go", "list", "-f", "{{.Dir}}", filepath.Dir(inFile))
		if err != nil && strings.Contains(out, "cannot find package") {
			args := []string{"get", "-tags", strings.Join(g.BuildTags, " ")}
			if gf := strings.Split(getFlags, " "); len(gf) > 0 {
				args = append(args, gf...)
			}
			args = append(args, inPkg)
			log.Printf("executing go %q", args)
			out, err = execCmd("go", args...)
		}
		if err != nil {
			log.Fatalf("error:\n%s\n", out)
		}
		inFile = filepath.Join(out, filepath.Base(inFile))
		pf, err := g.Parse(inFile, nil)
		if err != nil {
			log.Fatalf("error parsing file (%s): %v\n", inFile, err)
		}
		if err := pf.WriteFile(outPath); err != nil {
			log.Fatalf("error writing file: %v", err)
		}
	}
}

func execCmd(c string, args ...string) (string, error) {
	cmd := exec.Command(c, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

/*
go run cmd/genx/main.go -t KT=string -t "VT=interface{}" -rm-field HashFn -s "cm.HashFn=hashers.Fnv32" -s "cmap=-" -pkg ../cmap/internal/cmap/ -
	 n stringcmap -m
*/
