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
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"

	"github.com/fatih/astrewrite"
)

type GenX struct {
	pkgName        string
	rewriters      map[string]string
	crepl          func(string) string
	irepl          func(string) string
	imports        map[string]string
	zero_types     []string
	curReturnTypes []string
	visited        map[ast.Node]bool
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
		visited:   map[ast.Node]bool{},
		crepl:     reRepl(rewriters, false),
		irepl:     reRepl(rewriters, true),
		BuildTags: []string{"genx"},
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
			g.CommentFilters = append(g.CommentFilters, regexp.MustCompile(`\b`+kw+`\b`))
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
				g.zero_types = append(g.zero_types, sel)
			}
		}

		g.rewriters[k] = sel
	}

	g.CommentFilters = append(g.CommentFilters, regexp.MustCompile(`\+build \!?genx.*|go:generate genx`))
	sort.Strings(g.zero_types)
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
	if err = printer.Fprint(&buf, fset, astrewrite.Walk(file, g.rewrite)); err != nil {
		return
	}

	if idx == 0 && len(g.zero_types) > 0 {
		buf.WriteByte('\n')
		for _, t := range g.zero_types {
			fmt.Fprintf(&buf, "var zero_%s %s\n", cleanUpName.ReplaceAllString(t, ""), t)
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

func (g *GenX) rewrite(n ast.Node) (ast.Node, bool) {
	if n == nil {
		return deleteNode()
	}
	if g.visited[n] {
		return n, true
	}
	g.visited[n] = true
	rewr := g.rewriters
	//
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
				//
			}
			t.Name = nn
		}

	case *ast.FuncDecl:
		if t := getIdent(n.Name); t != nil {
			nn := rewr["func:"+t.Name]
			if nn == "-" {
				nukeComments(n.Doc)
				return deleteNode()
			} else if nn != "" {
				t.Name = nn
				break
			}
		}
		if recv := n.Recv; recv != nil && len(recv.List) == 1 {
			t := getIdent(recv.List[0].Type)
			if t == nil {
				log.Panicf("hmm... %#+v", recv.List[0].Type)
			}
			nn, ok := rewr["func:"+t.Name]
			if nn == "-" {
				nukeComments(n.Doc)
				return deleteNode()
			}
			if ok {
				t.Name = nn
			} else {
				t.Name = g.irepl(t.Name)
			}
		}

		if t, ok := g.rewriteExprTypes("type:", n.Type).(*ast.FuncType); ok {
			n.Type = t
		} else {
			nukeComments(n.Doc)
			return deleteNode()
		}

	case *ast.Ident:
		if t, ok := rewr["type:"+n.Name]; ok {
			if t == "-" {
				break
			}
			n.Name = t
		} else {
			n.Name = g.irepl(n.Name)
		}

	case *ast.Field:
		//
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
				n.Name = g.irepl(n.Name)
			}
			names = append(names, n)

		}

		if n.Names = names; len(n.Names) == 0 {
			nukeComments(n.Doc, n.Comment)
			return deleteNode()
		}

	case *ast.Comment:
		for _, f := range g.CommentFilters {
			if f.MatchString(n.Text) {
				return deleteNode()
			}
		}
		n.Text = g.crepl(n.Text)

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
				x.Name, n.Sel.Name = g.irepl(x.Name), g.irepl(n.Sel.Name)
				break
			}
			if nv == "-" {
				return deleteNode()
			}
			if xsel := strings.Split(nv, "."); len(xsel) == 2 {
				x.Name, n.Sel.Name = xsel[0], xsel[1]
				break
			} else {
				x.Name = nv
				return x, true
			}

		}
	case *ast.InterfaceType:
		if n.Methods != nil && len(n.Methods.List) == 0 {
			if nt := g.rewriters["type:interface{}"]; nt != "" {
				return &ast.Ident{
					Name: nt,
				}, true
			}
		}
	case *ast.ReturnStmt:
		for i, r := range n.Results {
			//			log.Printf("%#+v %s", getIdent(r), g.curReturnTypes)
			if rt := getIdent(r); rt != nil && rt.Name == "nil" {
				crt := cleanUpName.ReplaceAllString(g.curReturnTypes[i], "")
				if indexOf(g.zero_types, crt) > -1 {
					log.Println(g.zero_types)
					rt.Name = "zero_" + cleanUpName.ReplaceAllString(crt, "")
				}
			}
		}
	}

	return n, true
}

func indexOf(ss []string, v string) int {
	for i, s := range ss {
		if s == v {
			return i
		}
	}
	return -1
}

func nukeComments(cgs ...*ast.CommentGroup) {
	for _, cg := range cgs {
		if cg != nil {
			cg.List = nil
		}
	}
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
			t.Name = g.irepl(t.Name)
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

func deleteNode() (ast.Node, bool) { return nil, true }
