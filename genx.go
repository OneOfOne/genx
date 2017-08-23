package genx

import (
	"bytes"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
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
	crepl     *strings.Replacer
	irepl     *strings.Replacer
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
		crepl:     geireplacer(rewriters, false),
		irepl:     geireplacer(rewriters, true),
		BuildTags: []string{"genx"},
	}
	allBuiltinTypes := true
	for k, v := range rewriters {
		name, pkg, sel := parsePackageWithType(v)
		if pkg != "" {
			g.imports[pkg] = name
		}

		if sel == "" {
			sel = v
		}

		idx := strings.Index(k, ":")
		typ, kw := k[:idx], k[idx+1:]

		if v == "-" {
			g.CommentFilters = append(g.CommentFilters, regexp.MustCompile(`\b`+kw+`\b`))
		} else {
			switch typ {
			case "field":
				g.rewriters["selector:."+kw] = sel
			case "type":
				allBuiltinTypes = allBuiltinTypes && builtins[sel] != ""
				g.BuildTags = append(g.BuildTags, "genx_type_"+cleanUpName.ReplaceAllString(sel, ""))
			}

		}

		if allBuiltinTypes {
			g.BuildTags = append(g.BuildTags, "genx_builtin")
		}

		log.Println(g.BuildTags)

		g.rewriters[k] = sel
	}
	g.CommentFilters = append(g.CommentFilters, regexp.MustCompile(`\bnolint\b`))
	g.CommentFilters = append(g.CommentFilters, regexp.MustCompile(`\+build \!?genx.*|go:generate genx`))
	return g
}

func (g *GenX) Parse(fname string, src interface{}) (ParsedFile, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fname, src, parser.ParseComments)
	if err != nil {
		return ParsedFile{Name: fname}, err
	}

	return g.process(0, fset, fname, file)
}

func (g *GenX) ParsePkg(path string, includeTests bool) (out ParsedPkg, err error) {
	//fset := token.NewFileSet()
	ctx := build.Default
	ctx.BuildTags = append(ctx.BuildTags, g.BuildTags...)
	log.Println(path, ctx.BuildTags)
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
	for i, name := range files {
		var file *ast.File
		if file, err = parser.ParseFile(fset, filepath.Join(pkg.Dir, name), nil, parser.ParseComments); err != nil {
			return
		}
		var pf ParsedFile
		if pf, err = g.process(i, fset, name, file); err != nil {
			log.Printf("%s", pf.Src)
			return
		}
		out = append(out, pf)
	}
	return
}

var removePkgAndImports = regexp.MustCompile(`package .*|import ".*|(?s:import \(.*?\)\n)`)

func (g *GenX) process(idx int, fset *token.FileSet, name string, file *ast.File) (pf ParsedFile, err error) {
	for imp, name := range g.imports {
		if name != "" {
			astutil.AddNamedImport(fset, file, name, imp)
		} else {
			astutil.AddImport(fset, file, imp)
		}

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
	if idx > 0 {
		pf.Src = removePkgAndImports.ReplaceAll(pf.Src, nil)
	}
	pf.Name = name
	return
}

func (g *GenX) rewrite(n ast.Node) (ast.Node, bool) {
	if n == nil {
		return deleteNode()
	}

	rewr := g.rewriters

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
			if nn == "-" || nn == "" {
				return deleteNode()
			}
			switch n.Type.(type) {
			case *ast.SelectorExpr, *ast.InterfaceType, *ast.Ident:
				return deleteNode()
			default:
				// dbg.Dump(n)
			}
			t.Name = nn
		}

	case *ast.FuncDecl:
		if t := getIdent(n.Name); t != nil {
			nn := rewr["func:"+t.Name]
			if nn == "-" {
				return deleteNode()
			} else if nn != "" {
				t.Name = nn
				break
			}
		}
		if recv := n.Recv; recv != nil && len(recv.List) == 1 {
			if t := getIdent(recv.List[0].Type); t != nil && !g.isValidKey("type:"+t.Name) {
				return deleteNode()
			}
		}
		if params := n.Type.Params; params != nil {
			for _, p := range params.List {
				if t := getIdent(p.Type); t != nil && !g.isValidKey("type:"+t.Name) {
					return deleteNode()
				}
			}
		}
		if res := n.Type.Results; res != nil {
			for _, p := range res.List {
				if t := getIdent(p.Type); t != nil && !g.isValidKey("type:"+t.Name) {
					return deleteNode()
				}
			}
		}

	case *ast.Ident:
		if t, ok := rewr["type:"+n.Name]; ok {
			if t == "-" {
				break
			}
			n.Name = t
		} else {
			n.Name = g.irepl.Replace(n.Name)
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
			nn := rewr["field:"+n.Name]
			if nn == "-" {
				continue
			}
			if nn != "" {
				n.Name = nn
			}
			names = append(names, n)

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
		n.Text = g.crepl.Replace(n.Text)

	case *ast.KeyValueExpr:
		if key := getIdent(n.Key); key != nil && rewr["field:"+key.Name] == "-" {
			return deleteNode()
		}

	case *ast.SelectorExpr:
		if x := getIdent(n.X); x != nil && n.Sel != nil {
			if nv := g.rewriters["selector:."+n.Sel.Name]; nv != "" {
				n.Sel.Name = nv
				return n, true
			}
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
	switch ex := ex.(type) {
	case *ast.Ident:
		return ex
	case *ast.StarExpr:
		return getIdent(ex.X)
	default:
		return nil
	}
}

var cleanUpName = regexp.MustCompile(`[^\w\d_]+`)

func geireplacer(m map[string]string, ident bool) *strings.Replacer {
	kv := make([]string, 0, len(m)*2)
	for k, v := range m {
		k = k[strings.Index(k, ":")+1:]
		if ident {
			v = cleanUpName.ReplaceAllString(strings.Title(v), "")

			if a := builtins[v]; a != "" {
				v = a
			} else {
				v = cleanUpName.ReplaceAllString(v, "")
			}
		}

		kv = append(kv, k, v)
	}
	return strings.NewReplacer(kv...)
}

var builtins = map[string]string{
	"string":      "String",
	"byte":        "Byte",
	"[]byte":      "Bytes",
	"rune":        "Rune",
	"int":         "Int",
	"uint":        "Uint",
	"int8":        "Int8",
	"uint8":       "Uint8",
	"int16":       "Int16",
	"uint16":      "Uint16",
	"int32":       "Int32",
	"uint32":      "Uint32",
	"int64":       "Int64",
	"uint64":      "Uint64",
	"float32":     "Float32",
	"float64":     "Float64",
	"complex64":   "Cmplx64",
	"complex128":  "Cmplx128",
	"interface{}": "Iface",
	"Interface":   "Iface",
}

func deleteNode() (ast.Node, bool) { return nil, true }
