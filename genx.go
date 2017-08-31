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
	}

	g.rewriteFuncs = map[reflect.Type][]procFunc{
		reflect.TypeOf((*ast.TypeSpec)(nil)):      {g.rewriteTypeSpec},
		reflect.TypeOf((*ast.Ident)(nil)):         {g.rewriteIdent},
		reflect.TypeOf((*ast.Field)(nil)):         {g.rewriteField},
		reflect.TypeOf((*ast.FuncDecl)(nil)):      {g.rewriteFuncDecl},
		reflect.TypeOf((*ast.File)(nil)):          {g.rewriteFile},
		reflect.TypeOf((*ast.Comment)(nil)):       {g.rewriteComment},
		reflect.TypeOf((*ast.SelectorExpr)(nil)):  {g.rewriteSelectorExpr},
		reflect.TypeOf((*ast.KeyValueExpr)(nil)):  {g.rewriteKeyValueExpr},
		reflect.TypeOf((*ast.InterfaceType)(nil)): {g.rewriteInterfaceType},
		reflect.TypeOf((*ast.ReturnStmt)(nil)):    {g.rewriteReturnStmt},
		reflect.TypeOf((*ast.ArrayType)(nil)):     {g.rewriteArrayType},
		reflect.TypeOf((*ast.ChanType)(nil)):      {g.rewriteChanType},
		reflect.TypeOf((*ast.MapType)(nil)):       {g.rewriteMapType},
		reflect.TypeOf((*ast.FuncType)(nil)):      {g.rewriteFuncType},
		reflect.TypeOf((*ast.StarExpr)(nil)):      {g.rewriteStarExpr},
		reflect.TypeOf((*ast.Ellipsis)(nil)):      {g.rewriteEllipsis},
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
		//dbg.DumpWithDepth(4, n)
		// log.Printf("%T %#+v", n, node.Parent().Node())
		return node
	}
	g.visited[n] = true

	if fns, ok := g.rewriteFuncs[reflect.TypeOf(n)]; ok {
		for _, fn := range fns {
			if node = fn(node); node.Canceled() {
				break
			}
		}
	}

	return node
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
		// BUG: maybe? should we delete the func if we remove a field?
		case *ast.KeyValueExpr:
			x := getIdent(n.Key)
			if x == nil {
				break
			}
			if found = g.rewriters["field:"+x.Name] == "-"; found {
				return false
			}
		case *ast.SelectorExpr:
			x := getIdent(n.X)
			if x == nil {
				break
			}
			if x.Obj != nil && x.Obj.Type != nil {
				ot := getIdent(x.Obj.Type)
				if found = ot != nil && g.rewriters["type:"+ot.Name] == "-"; found {
					return false
				}
			}
			if found = g.rewriters["field:"+n.Sel.Name] == "-"; found {
				return false
			}
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

func getIdent(ex interface{}) *ast.Ident {
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
