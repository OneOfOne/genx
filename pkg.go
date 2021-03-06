package genx

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/imports"
)

var header = []byte(`// This file was automatically generated by genx.
// Any changes will be lost if this file is regenerated.
// see https://github.com/OneOfOne/genx
`)

type ParsedFile struct {
	Name string
	Src  []byte
}

func (f ParsedFile) WriteFile(path string) error {
	return writeFile(path, f.Src)
}

type ParsedPkg []ParsedFile

func (p ParsedPkg) WritePkg(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, f := range p {
		if err := f.WriteFile(filepath.Join(dir, f.Name)); err != nil {
			return err
		}
	}
	return nil
}

func (p ParsedPkg) MergeAll(tests bool) (ParsedFile, error) {
	// TODO: look into doing this with ast
	// var cleanSrc = regexp.MustCompile(`// nolint$`)
	var totalLen int
	for _, f := range p {
		isTest := strings.HasSuffix(f.Name, "_test.go")
		if (isTest && !tests) || (!isTest && tests) {
			continue
		}
		totalLen += len(f.Src)
	}

	pf := ParsedFile{
		Name: "all_gen.go",
		Src:  make([]byte, 0, totalLen),
	}

	for i, f := range p {
		isTest := strings.HasSuffix(f.Name, "_test.go")
		if (isTest && !tests) || (!isTest && tests) {
			continue
		}
		// f.Src = cleanSrc.ReplaceAll(f.Src, []byte("$1"))
		if i > 0 {
			f.Src = removePkgAndImports.ReplaceAll(f.Src, nil)
		}
		pf.Src = append(pf.Src, f.Src...)

	}

	// log.Printf("%s", out)
	out, err := imports.Process(pf.Name, pf.Src, &imports.Options{
		AllErrors: true,
		Comments:  true,
		TabIndent: true,
		TabWidth:  4,
	})

	if err == nil {
		pf.Src = out
	}

	return pf, err
}

func (p ParsedPkg) WriteAllMerged(fname string, tests bool) error {
	pf, err := p.MergeAll(tests)
	if err != nil {
		log.Printf("partial output:\n%s", pf.Src)
		return err
	}
	return writeFile(fname, pf.Src)
}

func writeFile(fp string, data []byte) error {
	dir := filepath.Dir(fp)
	if dir != "" && dir != "./" && dir != "/dev" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	f.Write(header)
	if args := os.Args; len(args) > 0 && args[0] == "genx" {
		fmt.Fprintf(f, "// cmd: %s\n", strings.Join(args, " "))
	}
	fmt.Fprintf(f, "// +build !genx\n\n")
	f.Write(data)
	return f.Close()
}
