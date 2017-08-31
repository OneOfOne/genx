package genx

import (
	"go/ast"
	"log"
	"strings"

	"github.com/OneOfOne/xast"
)

func (g *GenX) rewriteField(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.Field)
	nn := g.rewrite(xast.NewNode(node, n.Type))
	if nn.Canceled() {
		return node.Delete()
	}
	n.Type = nn.Node().(ast.Expr)

	if len(n.Names) == 0 {
		return node
	}

	names := n.Names[:0]
	for _, n := range n.Names {
		nn, ok := g.rewriters["field:"+n.Name]
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

	return node
}

func (g *GenX) rewriteTypeSpec(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.TypeSpec)
	if t := getIdent(n.Name); t != nil {
		nn := g.rewrite(xast.NewNode(node, n.Type))
		if nn.Canceled() {
			g.rewriters["type:"+t.Name] = "-"
			return node.Delete()
		}
		n.Type = nn.Node().(ast.Expr)

		tn, ok := g.rewriters["type:"+t.Name]
		if !ok {
			return node
		}
		if tn == "-" {
			return node.Delete()
		}
		t.Name = tn

	}
	return node
}

func (g *GenX) rewriteIdent(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.Ident)
	if t, ok := g.rewriters["type:"+n.Name]; ok {
		if t == "-" {
			return deleteWithParent(node)
		}
		n.Name = t
	} else {
		n.Name = g.irepl.Replace(n.Name)
	}
	return node
}

func (g *GenX) rewriteArrayType(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.ArrayType)
	nn := g.rewrite(xast.NewNode(node, n.Elt))
	if nn.Canceled() {
		return deleteWithParent(node)
	}
	n.Elt = nn.Node().(ast.Expr)
	return node

}

func (g *GenX) rewriteEllipsis(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.Ellipsis)
	nn := g.rewrite(xast.NewNode(node, n.Elt))
	if nn.Canceled() {
		return deleteWithParent(node)
	}
	n.Elt = nn.Node().(ast.Expr)
	return node

}

func (g *GenX) rewriteStarExpr(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.StarExpr)
	nn := g.rewrite(xast.NewNode(node, n.X))
	if nn.Canceled() {
		return deleteWithParent(node)
	}
	n.X = nn.Node().(ast.Expr)
	return node

}

func (g *GenX) rewriteChanType(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.ChanType)
	if x := getIdent(n.Value); x != nil && g.rewriters["type:"+x.Name] == "-" {
		return deleteWithParent(node)
	}
	return node
}

func (g *GenX) rewriteFuncType(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.FuncType)
	if n.Params != nil {
		for _, p := range n.Params.List {
			nn := g.rewrite(xast.NewNode(node, p.Type))
			if nn.Canceled() {
				return node.Delete()
			}
			p.Type = nn.Node().(ast.Expr)
		}
	}

	if n.Results != nil {
		g.curReturnTypes = g.curReturnTypes[:0]
		for _, p := range n.Results.List {
			nn := g.rewrite(xast.NewNode(node, p.Type))
			if nn.Canceled() {
				return node.Delete()
			}
			p.Type = nn.Node().(ast.Expr)
			if rt := getIdent(p.Type); rt != nil {
				g.curReturnTypes = append(g.curReturnTypes, rt.Name)
			}
		}
	}

	return node
}

func (g *GenX) rewriteMapType(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.MapType)
	nn := g.rewrite(xast.NewNode(node, n.Key))
	if nn.Canceled() {
		return deleteWithParent(node)
	}
	n.Key = nn.Node().(ast.Expr)
	nn = g.rewrite(xast.NewNode(node, n.Value))
	if nn.Canceled() {
		return deleteWithParent(node)
	}
	n.Value = nn.Node().(ast.Expr)
	return node
}

func (g *GenX) rewriteComment(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.Comment)
	for _, f := range g.CommentFilters {
		if n.Text = f(n.Text); n.Text == "" {
			return node.Delete()
		}
	}
	return node
}

func (g *GenX) rewriteKeyValueExpr(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.KeyValueExpr)
	if t := getIdent(n.Key); t != nil {
		if g.rewriters["type:"+t.Name] == "-" || g.rewriters["field:"+t.Name] == "-" {
			return node.Delete()
		}
	}

	nn := g.rewrite(xast.NewNode(node, n.Value))
	if nn.Canceled() {
		return node.Delete()
	}
	n.Value = nn.Node().(ast.Expr)

	return node
}

func (g *GenX) rewriteReturnStmt(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.ReturnStmt)
	for i, r := range n.Results {
		if rt := getIdent(r); rt != nil && rt.Name == "nil" {
			crt := cleanUpName.ReplaceAllString(g.curReturnTypes[i], "")
			if _, ok := g.zeroTypes[crt]; ok {
				g.zeroTypes[crt] = true
				rt.Name = "zero_" + cleanUpName.ReplaceAllString(crt, "")
			}
		}
	}
	return node
}

func (g *GenX) rewriteInterfaceType(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.InterfaceType)
	if n.Methods != nil && len(n.Methods.List) == 0 {
		if nt, ok := g.rewriters["type:interface{}"]; ok {
			if nt == "-" {
				return deleteWithParent(node)
			}
			return node.SetNode(&ast.Ident{
				Name: nt,
			})
		}
	}
	return node
}

func (g *GenX) rewriteSelectorExpr(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.SelectorExpr)
	x := getIdent(n.X)

	if x == nil || n.Sel == nil {
		return node
	}
	if nv := g.rewriters["selector:."+n.Sel.Name]; nv != "" {
		n.Sel.Name = nv
		return node
	}

	nv := g.rewriters["selector:"+x.Name+"."+n.Sel.Name]
	if nv == "" {
		if x.Name == g.pkgName {
			x.Name = n.Sel.Name
			return node.SetNode(x)
		}
		x.Name, n.Sel.Name = g.irepl.Replace(x.Name), g.irepl.Replace(n.Sel.Name)
		return node
	}

	if nv == "-" {
		return node.Delete()
	}

	if xsel := strings.Split(nv, "."); len(xsel) == 2 {
		x.Name, n.Sel.Name = xsel[0], xsel[1]
	} else {
		x.Name = nv
		return node.SetNode(x)
	}

	return node
}

func (g *GenX) rewriteFuncDecl(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.FuncDecl)
	if t := getIdent(n.Name); t != nil {
		nn := g.rewriters["func:"+t.Name]
		if nn == "-" {
			return node.Delete()
		} else if nn != "" {
			t.Name = nn
		}
	}

	if recv := n.Recv; recv != nil && len(recv.List) == 1 {
		t := getIdent(recv.List[0].Type)
		if t == nil {
			log.Panicf("hmm... %#+v", recv.List[0].Type)
		}
		nn, ok := g.rewriters["type:"+t.Name]

		if nn == "-" {
			return node.Delete()
		}
		if ok {
			t.Name = nn
		} else {
			t.Name = g.irepl.Replace(t.Name)
		}
	}

	nn := g.rewrite(xast.NewNode(node, n.Type))
	if nn.Canceled() {
		return node.Delete()
	}
	n.Type = nn.Node().(*ast.FuncType)

	if g.shouldNukeFuncBody(n.Body) {
		return node.Delete()
	}

	return node
}

var nukeGenxComments = regexpReplacer(`// \+build [!]?genx.*|//go:generate genx`, "")

func (g *GenX) rewriteFile(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.File)
	for _, cg := range n.Comments {
		list := cg.List[:0]
		for _, c := range cg.List {
			if c.Text = nukeGenxComments(c.Text); c.Text != "" {
				list = append(list, c)
			}
		}
		cg.List = list
	}

	if n.Doc == nil {
		return node
	}

	list := n.Doc.List[:0]
	for _, c := range n.Doc.List {
		if c.Text = nukeGenxComments(c.Text); c.Text != "" {
			list = append(list, c)
		}
	}
	n.Doc.List = list
	return node
}
