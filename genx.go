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
	imports   map[string]struct{}
	// filters   map[reflect.Type]func(n ast.Node) ast.Node
	// cfg       *struct{}

	BuildTags      []string
	CommentFilters []*regexp.Regexp
}

var pkgWithTypeRE = regexp.MustCompile(`^type:(?:(.*)/([\w\d]+)\.)?([*\w\d]+)$`)

func New(pkgName string, rewriters map[string]string) *GenX {
	g := &GenX{
		pkgName:   pkgName,
		rewriters: map[string]string{},
		imports:   map[string]struct{}{},
		repl:      getReplacer(rewriters),
	}
	for k, v := range rewriters {
		parts := pkgWithTypeRE.FindAllStringSubmatch(v, -1)
		if len(parts) == 1 {
			parts := parts[0]
			if parts[3][0] == '*' {
				v = "*" + parts[2] + "." + parts[3][1:]
			} else if parts[2] != "" {
				v = parts[2] + "." + parts[3]
			}
			g.imports[parts[1]+"/"+parts[2]] = struct{}{}
		}
		if strings.HasPrefix(k, "field:") {
			g.CommentFilters = append(g.CommentFilters, regexp.MustCompile(`\b`+k[6:]+`\b`))
		}
		if strings.HasPrefix(k, "func:") {
			g.CommentFilters = append(g.CommentFilters, regexp.MustCompile(`\b`+k[5:]+`\b`))
		}
		g.rewriters[k] = v
	}
	g.CommentFilters = append(g.CommentFilters, regexp.MustCompile(`\bnolint\b`))
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
	ctx.BuildTags = append(ctx.BuildTags, "genx") // allow packages to include/exclude files based on our tags
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

	for _, name := range files {
		var file *ast.File
		if file, err = parser.ParseFile(fset, filepath.Join(pkg.Dir, name), nil, parser.ParseComments); err != nil {
			return
		}
		var pf ParsedFile
		if pf, err = g.process(fset, name, file); err != nil {
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
	case *ast.TypeSpec:
		if t := getIdent(n.Name); t != nil && rewr["type:"+t.Name] != "" {
			return deleteNode()
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
