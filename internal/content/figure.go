package content

import (
	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Figure is a block node: an image that stood alone in its paragraph, with an optional caption (the image title).
type Figure struct {
	ast.BaseBlock
	Image *ast.Image
}

var KindFigure = ast.NewNodeKind("Figure")

func (n *Figure) Kind() ast.NodeKind { return KindFigure }
func (n *Figure) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type figureTransformer struct{}

func (figureTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	var targets []*ast.Paragraph
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if p, ok := n.(*ast.Paragraph); ok && p.ChildCount() == 1 {
			if _, isImg := p.FirstChild().(*ast.Image); isImg {
				targets = append(targets, p)
			}
		}
		return ast.WalkContinue, nil
	})
	for _, p := range targets {
		img := p.FirstChild().(*ast.Image)
		fig := &Figure{Image: img}
		p.RemoveChild(p, img)
		fig.SetLines(p.Lines())
		p.Parent().ReplaceChild(p.Parent(), p, fig)
		fig.AppendChild(fig, img)
	}
}

// TableWrap is a block around a table: the div scrolls horizontally on narrow screens and carries tabindex so keyboard users can scroll it (axe: scrollable-region-focusable).
type TableWrap struct{ ast.BaseBlock }

var KindTableWrap = ast.NewNodeKind("TableWrap")

func (n *TableWrap) Kind() ast.NodeKind { return KindTableWrap }
func (n *TableWrap) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, nil, nil)
}

type tableTransformer struct{}

func (tableTransformer) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	var tables []*extast.Table
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if t, ok := n.(*extast.Table); ok && entering {
			tables = append(tables, t)
		}
		return ast.WalkContinue, nil
	})
	for _, t := range tables {
		wrap := &TableWrap{}
		parent := t.Parent()
		parent.ReplaceChild(parent, t, wrap)
		wrap.AppendChild(wrap, t)
	}
}
