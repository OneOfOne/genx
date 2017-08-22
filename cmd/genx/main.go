package main

import (
	"flag"
	"fmt"
	"log"
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

	pkgName    string
	mergeFiles bool
)

func init() {
	log.SetFlags(log.Lshortfile)
	flag.Usage = func() {
		log.Println(`genx -pkg ../cmap/internal/cmap/ -n stringcmap -t KT=string -t "VT=interface{}" -rm-field HashFn \
	-s "cm.HashFn=hashers.Fnv32" -s "cmap.DefaultShardCount=DefaultShardCount" -m`)
	}
	flag.Var(&types, "t", `generic types (ex: -t KV=string -t "KV=interface{}" -t RemoveThisType)`)
	flag.Var(&selectors, "s", `selectors to remove or rename (ex: -s "cm.HashFn=hashers.Fnv32" -s "x.Call=Something")`)
	flag.Var(&fields, "rm-field", `fields to remove from structs (ex: -rm-field HashFn)`)
	flag.Var(&funcs, "rm-func", `funcs to remove (ex: -rm-func NotNeededFunc)`)
	flag.Var(&tags, "tags", `go build tags, used for parsing`)
	flag.StringVar(&inFile, "f", "", "file to parse")
	flag.StringVar(&inPkg, "pkg", "", "package to parse")
	flag.StringVar(&outPath, "o", "", "output dir if parsing a package or output filename if parsing a file")
	flag.StringVar(&pkgName, "n", "", "new package name")
	flag.BoolVar(&mergeFiles, "m", false, "merge all the files in a package into one")

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
		key, _ := fields.Split(i)
		if key == "" {
			continue
		}
		rewriters["field:"+key] = "-"
	}
	for i := range funcs {
		key, _ := funcs.Split(i)
		if key == "" {
			continue
		}
		rewriters["func:"+key] = "-"
	}

	log.Printf("%+q", rewriters)

	g := genx.New(pkgName, rewriters)
	g.BuildTags = []string(tags)

	if inPkg != "" {
		pkg, err := g.ParsePkg(inPkg, false)
		if err != nil {
			log.Fatalf("error parsing package (%s): %v", inPkg, err)
		}

		switch {
		case outPath == "", outPath == "-":
			pf, err := pkg.MergeAll(false)
			if err != nil {
				log.Fatalf("error merging files: %v\n%s", err, pf.Src)
			}
			fmt.Printf("%s\n", pf.Src)
		case mergeFiles:
			if err := pkg.WriteAllMerged(outPath, false); err != nil {
				log.Fatalf("error writing merged package: %v", err)
			}
		default:
			if err := pkg.WritePkg(outPath); err != nil {
				log.Fatalf("error writing merged package: %v", err)
			}
		}
	}
}

/*
go run cmd/genx/main.go -t KT=string -t "VT=interface{}" -rm-field HashFn -s "cm.HashFn=hashers.Fnv32" -s "cmap=-" -pkg ../cmap/internal/cmap/ -
	 n stringcmap -m
*/
