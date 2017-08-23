package genx

import (
	"bytes"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"

	"github.com/fatih/astrewrite"
)

type GenX struct {
	pkgName   string
	rewriters map[string]string
	repl      *strings.Replacer
	imports   map[string]string
	// filters   map[reflect.Type]func(n ast.Node) ast.Node
	// cfg       *struct{}

	BuildTags      []string
	CommentFilters []*regexp.Regexp
}

func New(pkgName string, rewriters map[string]string) *GenX {
	g := &GenX{
		pkgName:   pkgName,
		rewriters: map[string]string{},
		imports:   map[string]string{},
		repl:      getReplacer(rewriters),
		BuildTags: []string{"genx"},
	}
	for k, v := range rewriters {
		typ, name, pkg, sel := parsePackageWithType(v)
		if pkg != "" {
			g.imports[pkg] = name
		}
		if sel == "" {
			sel = v
		}
		switch typ {
		case "field", "func":
			g.CommentFilters = append(g.CommentFilters, regexp.MustCompile(`\b`+sel+`\b`))
		}
		g.rewriters[k] = sel
	}
	g.CommentFilters = append(g.CommentFilters, regexp.MustCompile(`\bnolint\b`))
	g.CommentFilters = append(g.CommentFilters, regexp.MustCompile(`\+build genx|go:generate genx`))
	return g
}

func (g *GenX) Parse(fname string, src interface{}) (ParsedFile, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fname, src, parser.ParseComments)
	if err != nil {
		return ParsedFile{Name: fname}, err
	}

	return g.process(fset, fname, file)
}

func (g *GenX) ParsePkg(path string, includeTests bool) (out ParsedPkg, err error) {
	//fset := token.NewFileSet()
	ctx := build.Default
	ctx.BuildTags = append(ctx.BuildTags, g.BuildTags...)

	pkg, err := ctx.ImportDir(path, build.IgnoreVendor)
	if err != nil {
		return nil, err
	}

	out = make(ParsedPkg, 0, len(pkg.GoFiles))
	fset := token.NewFileSet()

	files := append([]string{}, pkg.GoFiles...)
	if includeTests {
		files = append(files, pkg.TestGoFiles...)
	}

	// TODO: process multiple files in the same time.
	for _, name := range files {
		var file *ast.File
		if file, err = parser.ParseFile(fset, filepath.Join(pkg.Dir, name), nil, parser.ParseComments); err != nil {
			return
		}
		var pf ParsedFile
		if pf, err = g.process(fset, name, file); err != nil {
			// log.Printf("%s", pf.Src)
			return
		}
		out = append(out, pf)
	}
	return
}

func (g *GenX) process(fset *token.FileSet, name string, file *ast.File) (pf ParsedFile, err error) {
	for ip := range g.imports {
		astutil.AddImport(fset, file, ip)
	}

	if g.pkgName != "" && g.pkgName != file.Name.Name {
		file.Name.Name = g.pkgName
	} else {
		g.pkgName = file.Name.Name
	}

	var buf bytes.Buffer
	if err = printer.Fprint(&buf, fset, astrewrite.Walk(file, g.rewrite)); err != nil {
		return
	}
	if pf.Src, err = imports.Process(name, buf.Bytes(), &imports.Options{
		AllErrors: true,
		Comments:  true,
		TabIndent: true,
		TabWidth:  4,
	}); err != nil {
		pf.Src = buf.Bytes()
	}
	pf.Name = name
	return
}

func (g *GenX) rewrite(n ast.Node) (ast.Node, bool) {
	if n == nil {
		return deleteNode()
	}

	// log.Printf("%T %+v", n, n)

	rewr, repl := g.rewriters, g.repl.Replace

	switch n := n.(type) {
	case *ast.File: // handle comments here
		comments := n.Comments[:0]
	L:
		for _, cg := range n.Comments {
			txt := cg.Text()
			for _, f := range g.CommentFilters {
				if f.MatchString(txt) {
					continue L
				}
			}
			comments = append(comments, cg)
		}
		n.Comments = comments

	case *ast.TypeSpec:
		if t := getIdent(n.Name); t != nil {
			nn, ok := rewr["type:"+t.Name]
			if !ok {
				break
			}
			if i, _ := n.Type.(*ast.InterfaceType); i != nil && i.Methods != nil && i.Methods.NumFields() == 0 {
				return deleteNode()
			}
			t.Name = nn
		}

	case *ast.FuncDecl:
		if t := getIdent(n.Name); t != nil && rewr["func:"+t.Name] == "-" {
			return deleteNode()
		}

	case *ast.Ident:
		if t := rewr["type:"+n.Name]; t != "" {
			n.Name = t
		}

	case *ast.Field:
		if ft := getIdent(n.Type); ft != nil {
			if t := rewr[ft.Name]; t != "" {
				ft.Name = t
			}
		}
		if len(n.Names) == 0 {
			break
		}

		names := n.Names[:0]
		for _, n := range n.Names {
			// TODO: allow renaming fields
			if g.isValidKey("field:" + n.Name) {
				names = append(names, n)
			}
		}
		// TODO (BUG):doesn't remove associated comments for some reason.
		if n.Names = names; len(n.Names) == 0 {
			return deleteNode()
		}

	case *ast.Comment:
		for _, f := range g.CommentFilters {
			if f.MatchString(n.Text) {
				return deleteNode()
			}
		}
		n.Text = repl(n.Text)

	case *ast.KeyValueExpr:
		if key := getIdent(n.Key); key != nil && rewr["field:"+key.Name] == "-" {
			return deleteNode()
		}

	case *ast.SelectorExpr:
		if x := getIdent(n.X); x != nil && n.Sel != nil {
			nv := g.rewriters["selector:"+x.Name+"."+n.Sel.Name]
			if nv == "" {
				if x.Name == g.pkgName {
					x.Name = n.Sel.Name
					return x, true
				}
				break
			}
			if nv == "-" {
				return deleteNode()
			}
			if xsel := strings.Split(nv, "."); len(xsel) == 2 {
				x.Name, n.Sel.Name = xsel[0], xsel[1]
			} else {
				x.Name = nv
				return x, true
			}
		}
	}

	return n, true
}

func (g *GenX) isValidKey(n string) bool {
	v, ok := g.rewriters[n]
	if !ok {
		return true
	}
	return v != "-" && v != ""
}

func getIdent(ex ast.Expr) *ast.Ident {
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

func deleteNode() (ast.Node, bool) { return nil, true }
