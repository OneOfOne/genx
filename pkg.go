package genx

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/imports"
)

type ParsedFile struct {
	Name string
	Src  []byte
}
type ParsedPkg []ParsedFile

func (p ParsedPkg) WritePkg(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	for _, f := range p {
		if err := ioutil.WriteFile(filepath.Join(dir, f.Name), f.Src, 0644); err != nil {
			return err
		}
	}
	return nil
}

// TODO: look into doing this with ast
var cleanSrc = regexp.MustCompile(`^package .*|import "|(?s:import \(.*?\)\n)`)

func (p ParsedPkg) MergeAll(tests bool) (ParsedFile, error) {
	var totalLen int
	for _, f := range p {
		isTest := strings.HasSuffix(f.Name, "_test.go")
		if (isTest && !tests) || (!isTest && tests) {
			continue
		}
		totalLen += len(f.Src)
	}

	gotPkg := false

	pf := ParsedFile{
		Name: "all_gen.go",
		Src:  make([]byte, 0, totalLen),
	}

	for _, f := range p {
		isTest := strings.HasSuffix(f.Name, "_test.go")
		if (isTest && !tests) || (!isTest && tests) {
			continue
		}
		if gotPkg {
			f.Src = cleanSrc.ReplaceAll(f.Src, nil)
		} else {
			gotPkg = true
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
		return err
	}
	return ioutil.WriteFile(fname, pf.Src, 0644)
}
