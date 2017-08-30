package genx

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/OneOfOne/xast"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"
)

type procFunc func(*xast.Node) *xast.Node
type GenX struct {
	pkgName        string
	rewriters      map[string]string
	irepl          *strings.Replacer
	imports        map[string]string
	zeroTypes      map[string]bool
	curReturnTypes []string
	visited        map[ast.Node]bool

	BuildTags      []string
	CommentFilters []func(string) string

	rewriteFuncs map[reflect.Type][]procFunc
}

func New(pkgName string, rewriters map[string]string) *GenX {
	g := &GenX{
		pkgName:   pkgName,
		rewriters: map[string]string{},
		imports:   map[string]string{},
		visited:   map[ast.Node]bool{},
		irepl:     geireplacer(rewriters, true),
		zeroTypes: map[string]bool{},
		BuildTags: []string{"genx"},
		CommentFilters: []func(string) string{
			regexpReplacer(`// \+build [!]?genx.*|//go:generate genx`, ""),
		},
	}

	g.rewriteFuncs = map[reflect.Type][]procFunc{
		reflect.TypeOf((*ast.TypeSpec)(nil)): {g.rewriteTypeSpec},
		reflect.TypeOf((*ast.Ident)(nil)):    {g.rewriteIdent},
		reflect.TypeOf((*ast.Field)(nil)):    {g.rewriteField},
		reflect.TypeOf((*ast.FuncDecl)(nil)): {g.rewriteFuncDecl},
		reflect.TypeOf((*ast.File)(nil)):     {g.rewriteFile},
	}

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
			g.CommentFilters = append(g.CommentFilters, regexpReplacer(`\b`+kw+`\b`, ""))
		} else {
			switch typ {
			case "field":
				g.rewriters["selector:."+kw] = sel
			case "type":
				csel := cleanUpName.ReplaceAllString(sel, "")
				kw = cleanUpName.ReplaceAllString(kw, "")
				if isBuiltin := csel != "interface" && builtins[csel] != ""; isBuiltin {
					g.BuildTags = append(g.BuildTags, "genx_"+strings.ToLower(kw)+"_builtin")
				}
				g.BuildTags = append(g.BuildTags, "genx_"+strings.ToLower(kw)+"_"+csel)
				g.zeroTypes[sel] = false
				g.CommentFilters = append(g.CommentFilters, regexpReplacer(`\b(`+kw+`)\b`, sel))
				g.CommentFilters = append(g.CommentFilters, regexpReplacer(`(`+kw+`)`, strings.Title(csel)))
			}
		}

		g.rewriters[k] = sel
	}

	return g
}

// Parse parses the input file or src and returns a ParsedFile and/or an error.
// For more details about fname/src check `go/parser.ParseFile`
func (g *GenX) Parse(fname string, src interface{}) (ParsedFile, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fname, src, parser.ParseComments)
	if err != nil {
		return ParsedFile{Name: fname}, err
	}

	return g.process(0, fset, fname, file)
}

// ParsePKG will parse the provided package, on success it will then process the files with
// x/tools/imports (goimports) then return the resulting package.
func (g *GenX) ParsePkg(path string, includeTests bool) (out ParsedPkg, err error) {
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
	if err = printer.Fprint(&buf, fset, xast.Walk(file, g.rewrite)); err != nil {
		return
	}

	if idx == 0 && len(g.zeroTypes) > 0 {
		buf.WriteByte('\n')
		for t, used := range g.zeroTypes {
			if used {
				fmt.Fprintf(&buf, "var zero_%s %s\n", cleanUpName.ReplaceAllString(t, ""), t)
			}
		}
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

func (g *GenX) rewrite(node *xast.Node) *xast.Node {
	n := node.Node()
	if g.visited[n] {
		return node
	}
	g.visited[n] = true

	if fns, ok := g.rewriteFuncs[reflect.TypeOf(n)]; ok {
		for _, fn := range fns {
			if node = fn(node); node.Canceled() {
				break
			}
		}
		return node
	}

	rewr := g.rewriters
	switch n := n.(type) {
	case *ast.Field:
		n.Type = g.rewriteExprTypes("type:", n.Type)

		if len(n.Names) == 0 {
			break
		}

		names := n.Names[:0]
		for _, n := range n.Names {
			nn, ok := rewr["field:"+n.Name]
			if nn == "-" {
				continue
			}
			if ok {
				n.Name = nn
			} else {
				n.Name = g.irepl.Replace(n.Name)
			}
			names = append(names, n)

		}

		if n.Names = names; len(n.Names) == 0 {
			return node.Delete()
		}

	case *ast.Comment:
		for _, f := range g.CommentFilters {
			if n.Text = f(n.Text); n.Text == "" {
				return node.Delete()
			}
		}

	case *ast.KeyValueExpr:
		if key := getIdent(n.Key); key != nil && rewr["field:"+key.Name] == "-" {
			return node.Delete()
		}

	case *ast.SelectorExpr:
		if x := getIdent(n.X); x != nil && n.Sel != nil {
			if nv := g.rewriters["selector:."+n.Sel.Name]; nv != "" {
				n.Sel.Name = nv
				break
			}
			nv := g.rewriters["selector:"+x.Name+"."+n.Sel.Name]
			if nv == "" {
				if x.Name == g.pkgName {
					x.Name = n.Sel.Name
					return node.SetNode(x)
				}
				x.Name, n.Sel.Name = g.irepl.Replace(x.Name), g.irepl.Replace(n.Sel.Name)
				break
			}
			if nv == "-" {
				return node.Delete()
			}
			if xsel := strings.Split(nv, "."); len(xsel) == 2 {
				x.Name, n.Sel.Name = xsel[0], xsel[1]
				break
			} else {
				x.Name = nv
				return node.SetNode(x)
			}

		}
	case *ast.InterfaceType:
		if n.Methods != nil && len(n.Methods.List) == 0 {
			if nt := g.rewriters["type:interface{}"]; nt != "" {
				return node.SetNode(&ast.Ident{
					Name: nt,
				})
			}
		}
	case *ast.ReturnStmt:
		for i, r := range n.Results {
			if rt := getIdent(r); rt != nil && rt.Name == "nil" {
				crt := cleanUpName.ReplaceAllString(g.curReturnTypes[i], "")
				if _, ok := g.zeroTypes[crt]; ok {
					g.zeroTypes[crt] = true
					rt.Name = "zero_" + cleanUpName.ReplaceAllString(crt, "")
				}
			}
		}
	case *ast.File:

	}

	return node
}

func indexOf(ss []string, v string) int {
	for i, s := range ss {
		if s == v {
			return i
		}
	}
	return -1
}

func (g *GenX) shouldNukeFuncBody(bs *ast.BlockStmt) (found bool) {
	if bs == nil {
		return
	}
	ast.Inspect(bs, func(n ast.Node) bool {
		if found {
			return false
		}
		switch n := n.(type) {
		case *ast.Ident:
			if found = g.rewriters["type:"+n.Name] == "-"; found {
				return false
			}
			// TODO handle removed fields / funcs
		}
		return true
	})
	return
}

func (g *GenX) rewriteExprTypes(prefix string, ex ast.Expr) ast.Expr {
	if g.visited[ex] {
		return ex
	}
	g.visited[ex] = true

	switch t := ex.(type) {
	case *ast.InterfaceType:
		if nt := g.rewriters[prefix+"interface{}"]; nt != "" {
			if nt == "-" {
				return nil
			}
			ex = &ast.Ident{
				Name: nt,
			}
		}
	case *ast.Ident:
		if nt := g.rewriters[prefix+t.Name]; nt != "" {
			if nt == "-" {
				return nil
			}
			t.Name = nt
		} else {
			t.Name = g.irepl.Replace(t.Name)
		}
	case *ast.StarExpr:
		if t.X = g.rewriteExprTypes(prefix, t.X); t.X == nil {
			return nil
		}
	case *ast.Ellipsis:
		if t.Elt = g.rewriteExprTypes(prefix, t.Elt); t.Elt == nil {
			return nil
		}
	case *ast.ArrayType:
		if t.Elt = g.rewriteExprTypes(prefix, t.Elt); t.Elt == nil {
			return nil
		}
	case *ast.MapType:
		if t.Key = g.rewriteExprTypes(prefix, t.Key); t.Key == nil {
			return nil
		}
		if t.Value = g.rewriteExprTypes(prefix, t.Value); t.Value == nil {
			return nil
		}
	case *ast.FuncType:
		if t.Params != nil {
			for _, p := range t.Params.List {
				if p.Type = g.rewriteExprTypes(prefix, p.Type); p.Type == nil {
					return nil
				}
			}
		}
		if t.Results != nil {
			g.curReturnTypes = g.curReturnTypes[:0]
			for _, p := range t.Results.List {
				if p.Type = g.rewriteExprTypes(prefix, p.Type); p.Type == nil {
					return nil
				}
				if rt := getIdent(p.Type); rt != nil {
					g.curReturnTypes = append(g.curReturnTypes, rt.Name)
				}
			}
		}
	}
	return ex
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

			if a := builtins[v]; a != "" {
				v = a
			} else {
				v = cleanUpName.ReplaceAllString(strings.Title(v), "")
			}
		}

		kv = append(kv, k, v)
	}
	return strings.NewReplacer(kv...)
}

var replRe = regexp.MustCompile(`\b([\w\d_]+)\b`)

func reRepl(m map[string]string, ident bool) func(src string) string {
	kv := map[string]string{}
	for k, v := range m {
		k = k[strings.Index(k, ":")+1:]
		if ident {
			if a := builtins[v]; a != "" {
				v = a
			} else {
				v = cleanUpName.ReplaceAllString(strings.Title(v), "")
			}
		}

		kv[k] = v
	}
	return func(src string) string {
		re := replRe.Copy()
		return re.ReplaceAllStringFunc(src, func(in string) string {
			if v := kv[in]; v != "" {
				return v
			}
			return in
		})
	}
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
