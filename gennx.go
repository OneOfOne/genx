package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"reflect"
	"strings"

	"github.com/fatih/astrewrite"
)

func main() {
	// src, _ := ioutil.ReadFile("/home/oneofone/code/go/src/github.com/OneOfOne/cmap/internal/cmap/lmap.go")

	typeMap := map[string]string{
		"KT":       "string",
		"VT":       "interface{}",
		"RemoveMe": "",
	}

	g := New("stringcmap", typeMap)

	src, err := g.Parse("./all_types.go", nil)

	fmt.Printf("%v\n%s\n", err, src)
	// Output:
	// package main
	//
	// type Bar struct{}
}

func getIndent(ex ast.Expr) *ast.Ident {
	if v, ok := ex.(*ast.Ident); ok {
		return v
	}
	return nil
}

func getReplacer(m map[string]string) *strings.Replacer {
	kv := make([]string, 0, len(m)*2)
	for k, v := range m {
		kv = append(kv, k, v)
	}
	return strings.NewReplacer(kv...)
}

type GenX struct {
	pkgName string
	kvs     map[string]string
	repl    *strings.Replacer
	filters map[reflect.Type]func(n ast.Node) ast.Node
	cfg     *struct{}
}

func New(pkgName string, kvs map[string]string) *GenX {
	return &GenX{
		pkgName: pkgName,
		kvs:     kvs,
		repl:    getReplacer(kvs),
	}
}

func (g *GenX) Parse(fname string, src interface{}) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fname, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = format.Node(&buf, fset, astrewrite.Walk(file, g.rewrite))
	return buf.Bytes(), err
}

func (g *GenX) rewrite(n ast.Node) (ast.Node, bool) {
	if n != nil {
		log.Printf("%T %+v", n, n)
	}
	tm, repl := g.kvs, g.repl.Replace
	switch n := n.(type) {
	case *ast.TypeSpec:
		if t := getIndent(n.Name); t != nil && tm[t.Name] != "" {
			return nil, true
		}
	case *ast.MapType:
		if kt := getIndent(n.Key); kt != nil {
			if t := tm[kt.Name]; t != "" {
				kt.Name = t
			}
		}
		if vt := getIndent(n.Value); vt != nil {
			if t := tm[vt.Name]; t != "" {
				vt.Name = t
			}
		}
	case *ast.FuncDecl:
		if t := getIndent(n.Name); t != nil {
			if v, ok := tm[t.Name]; ok && v == "" {
				return nil, true
			}
		}
	case *ast.Comment:
		n.Text = repl(n.Text)

	case *ast.Ident:
		if t := tm[n.Name]; t != "" {
			n.Name = t
		}

	case *ast.Field:
		if ft := getIndent(n.Type); ft != nil {
			if t := tm[ft.Name]; t != "" {
				ft.Name = t
			}
		}

	case *ast.ArrayType:
		if ft := getIndent(n.Elt); ft != nil {
			if t := tm[ft.Name]; t != "" {
				ft.Name = t
			}
		}
	}

	return n, true
}
