package content

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"math"
	"strings"
	"unicode"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	ghtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

const wordsPerMinute = 200

// focusablePreWrapper wraps chroma's highlighted output in a <pre tabindex="0">
// instead of the library default <pre>: chroma v2.27.0 has no built-in option
// for this, but exposes WithPreWrapper for exactly this kind of customization.
// A focusable <pre> lets keyboard users scroll code blocks that overflow
// horizontally (the same axe scrollable-region-focusable rule TableWrap
// satisfies for tables).
type focusablePreWrapper struct{}

func (focusablePreWrapper) Start(code bool, styleAttr string) string {
	if code {
		return fmt.Sprintf(`<pre tabindex="0"%s><code>`, styleAttr)
	}
	return fmt.Sprintf(`<pre tabindex="0"%s>`, styleAttr)
}

func (focusablePreWrapper) End(code bool) string {
	if code {
		return `</code></pre>`
	}
	return `</pre>`
}

func typographer(lang string) goldmark.Extender {
	if lang == "ro" {
		return extension.NewTypographer(extension.WithTypographicSubstitutions(extension.TypographicSubstitutions{
			extension.LeftDoubleQuote:  []byte("„"),
			extension.RightDoubleQuote: []byte("”"),
			extension.LeftSingleQuote:  []byte("‚"),
			extension.RightSingleQuote: []byte("’"),
		}))
	}
	return extension.Typographer
}

func newMarkdown(lang string, lookup func(string) *ImageInfo, tableLabel string) goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			// Column alignment would need style= (blocked by the hash CSP) or the obsolete align= (Nu error); neither is emitted.
			extension.NewTable(extension.WithTableCellAlignMethod(extension.TableCellAlignNone)),
			extension.Footnote,
			typographer(lang),
			highlighting.NewHighlighting(
				highlighting.WithFormatOptions(
					chromahtml.WithClasses(true),
					chromahtml.WithPreWrapper(focusablePreWrapper{}),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(util.Prioritized(figureTransformer{}, 100), util.Prioritized(tableTransformer{}, 200)),
		),
		goldmark.WithRendererOptions(
			ghtml.WithUnsafe(), // inline SVG diagrams and the occasional raw block; check --dist polices scripts and style=
			renderer.WithNodeRenderers(util.Prioritized(&nodeRenderer{lookup: lookup, tableLabel: tableLabel}, 100)),
		),
	)
}

// ParseBody parses Markdown into p.doc and collects images, links and word count. Alt text is mandatory.
func ParseBody(p *Piece, body []byte) Problems {
	var probs Problems
	p.src = body
	md := newMarkdown(p.Lang, nil, "")
	p.doc = md.Parser().Parse(text.NewReader(body))
	p.Images, p.Links = nil, nil
	words := 0
	prevLevel := 1 // of the last heading seen; the template's h1 (title or masthead) precedes every body
	_ = ast.Walk(p.doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.Heading:
			// The title is the page's only h1 (spec §5.2), so the first body heading is ## and the outline may only
			// descend one level at a time after it (html-validate strict heading-level, row 2): no gaps for assistive
			// tech or the outline algorithm.
			if v.Level == 1 {
				probs.Add(p.File, lineOf(body, v, p.bodyLine), "level-1 heading in body; the title is the page's h1 — use ## and below")
			} else if v.Level > prevLevel+1 {
				probs.Add(p.File, lineOf(body, v, p.bodyLine), "heading level jumps from h%d to h%d", prevLevel, v.Level)
			}
			prevLevel = v.Level
		case *ast.Image:
			alt := imageAltText(body, v)
			line := lineOf(body, v, p.bodyLine)
			if strings.TrimSpace(alt) == "" {
				probs.Add(p.File, line, "image %q has no alt text", string(v.Destination))
			}
			p.Images = append(p.Images, ImageRef{Dest: string(v.Destination), Alt: alt, Line: line})
		case *ast.Link:
			if d := string(v.Destination); strings.HasPrefix(d, "/") {
				p.Links = append(p.Links, LinkRef{Dest: strings.SplitN(d, "#", 2)[0], Line: lineOf(body, v, p.bodyLine)})
			}
		case *ast.FencedCodeBlock:
			// chroma adds tabindex="0" to every <pre> it renders; a block without a language would be a plain, unfocusable <pre>.
			if v.Language(body) == nil {
				probs.Add(p.File, lineOf(body, v, p.bodyLine)-1, "fenced code needs a language (```text for plain text)")
			}
		case *ast.CodeBlock:
			probs.Add(p.File, lineOf(body, v, p.bodyLine), "indented code block; use a fenced block with a language")
		case *ast.Text:
			words += countWords(v.Segment.Value(body))
		case *ast.String:
			words += countWords(v.Value)
		}
		return ast.WalkContinue, nil
	})
	p.Words = words
	p.ReadingMinutes = int(math.Max(1, math.Ceil(float64(words)/wordsPerMinute)))
	probs = append(probs, CheckDiacritics(p.File, body, p.bodyLine)...)
	return probs
}

// RenderBody renders the parsed document with image information injected; tableLabel names scrollable table regions.
func RenderBody(p *Piece, lookup func(string) *ImageInfo, tableLabel string) (template.HTML, error) {
	if p.doc == nil {
		return "", fmt.Errorf("%s: ParseBody must run before RenderBody", p.File)
	}
	if lookup == nil {
		lookup = func(string) *ImageInfo { return nil }
	}
	var buf bytes.Buffer
	md := newMarkdown(p.Lang, lookup, tableLabel)
	if err := md.Renderer().Render(&buf, p.src, p.doc); err != nil {
		return "", fmt.Errorf("%s: render: %w", p.File, err)
	}
	return template.HTML(buf.String()), nil
}

type nodeRenderer struct {
	lookup     func(string) *ImageInfo
	tableLabel string
}

func (r *nodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, r.renderImage)
	reg.Register(KindFigure, r.renderFigure)
	reg.Register(KindTableWrap, r.renderTableWrap)
}

func (r *nodeRenderer) renderTableWrap(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<div class="table" role="region" aria-label="` + html.EscapeString(r.tableLabel) + `" tabindex="0">`)
	} else {
		_, _ = w.WriteString("</div>\n")
	}
	return ast.WalkContinue, nil
}

func (r *nodeRenderer) renderFigure(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	fig := n.(*Figure)
	if !entering {
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString("<figure>")
	r.writeImg(w, source, fig.Image)
	if t := string(fig.Image.Title); t != "" {
		_, _ = w.WriteString("<figcaption>" + html.EscapeString(t) + "</figcaption>")
	}
	_, _ = w.WriteString("</figure>\n")
	return ast.WalkSkipChildren, nil
}

func (r *nodeRenderer) renderImage(w util.BufWriter, source []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	r.writeImg(w, source, n.(*ast.Image))
	return ast.WalkSkipChildren, nil
}

func (r *nodeRenderer) writeImg(w util.BufWriter, source []byte, img *ast.Image) {
	dest := string(img.Destination)
	alt := html.EscapeString(imageAltText(source, img))
	info := r.lookup(dest)
	if info == nil { // unknown image: emit as written so check --dist / link checks catch it
		_, _ = w.WriteString(`<img src="` + html.EscapeString(dest) + `" alt="` + alt + `">`)
		return
	}
	imgTag := fmt.Sprintf(`<img src="%s" alt="%s" width="%d" height="%d" loading="lazy" decoding="async">`, html.EscapeString(info.Src), alt, info.Width, info.Height)
	if info.IsSVG || info.Srcset == "" {
		_, _ = w.WriteString(imgTag)
		return
	}
	_, _ = w.WriteString(`<picture><source type="image/webp" srcset="` + html.EscapeString(info.Srcset) + `" sizes="(max-width: 40rem) 100vw, 38rem">` + imgTag + `</picture>`)
}

// imageAltText returns an image's alt text from its inline children.
// (*ast.Image).Text is deprecated in this goldmark version; this mirrors
// goldmark's own unexported renderTexts, which the HTML renderer uses for
// exactly this attribute, so the value it produces is identical.
func imageAltText(source []byte, n ast.Node) string {
	var buf bytes.Buffer
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch v := c.(type) {
		case *ast.String:
			buf.Write(v.Value)
		case *ast.Text:
			buf.Write(v.Value(source))
			if v.SoftLineBreak() || v.HardLineBreak() {
				buf.WriteByte('\n')
			}
		default:
			buf.WriteString(imageAltText(source, c))
		}
	}
	return buf.String()
}

func lineOf(src []byte, n ast.Node, base int) int {
	// Inline nodes carry no position; use the enclosing block's first segment.
	for b := n; b != nil; b = b.Parent() {
		if b.Type() == ast.TypeBlock && b.Lines() != nil && b.Lines().Len() > 0 {
			return base + bytes.Count(src[:b.Lines().At(0).Start], []byte("\n"))
		}
	}
	return base
}

func countWords(b []byte) int {
	return len(bytes.FieldsFunc(b, func(r rune) bool { return unicode.IsSpace(r) }))
}
