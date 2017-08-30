package genx

import (
	"go/ast"
	"log"
	"strings"

	"github.com/OneOfOne/xast"
)

func (g *GenX) rewriteField(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.Field)
	n.Type = g.rewriteExprTypes("type:", n.Type)

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
		nn, ok := g.rewriters["type:"+t.Name]
		if !ok {
			return node
		}
		if nn == "-" || nn == "" {
			return node.Delete()
		}
		switch n.Type.(type) {
		case *ast.SelectorExpr, *ast.InterfaceType, *ast.Ident:
			return node.Delete()
		default:
			t.Name = nn
		}

	}
	return node
}

func (g *GenX) rewriteIdent(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.Ident)
	if t, ok := g.rewriters["type:"+n.Name]; ok {
		if t == "-" {
			return node
		}
		n.Name = t
	} else {
		n.Name = g.irepl.Replace(n.Name)
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

	if t, ok := g.rewriteExprTypes("type:", n.Type).(*ast.FuncType); ok {
		n.Type = t
	} else {
		return node.Delete()
	}

	if g.shouldNukeFuncBody(n.Body) {
		return node.Delete()
	}

	return node
}

func (g *GenX) rewriteFile(node *xast.Node) *xast.Node {
	n := node.Node().(*ast.File)
	for _, cg := range n.Comments {
		list := cg.List[:0]
		for _, c := range cg.List {
			for _, f := range g.CommentFilters {
				if c.Text = f(c.Text); c.Text == "" {
					break
				}
			}
			if c.Text != "" && strings.TrimSpace(c.Text) != "//" {
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
		for _, f := range g.CommentFilters {
			if c.Text = f(c.Text); c.Text == "" {
				break
			}
		}

		if c.Text != "" && strings.TrimSpace(c.Text) != "//" {
			list = append(list, c)
		}

	}
	n.Doc.List = list
	return node
}
