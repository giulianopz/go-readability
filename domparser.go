/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this file,
 * You can obtain one at http://mozilla.org/MPL/2.0/. */

/**
 * This is a relatively lightweight DOMParser that is safe to use in a web
 * worker. This is far from a complete DOM implementation; however, it should
 * contain the minimal set of functionality necessary for Readability.js.
 *
 * Aside from not implementing the full DOM API, there are other quirks to be
 * aware of when using the JSDOMParser:
 *
 *   1) Properly formed HTML/XML must be used. This means you should be extra
 *      careful when using this parser on anything received directly from an
 *      XMLHttpRequest. Providing a serialized string from an XMLSerializer,
 *      however, should be safe (since the browser's XMLSerializer should
 *      generate valid HTML/XML). Therefore, if parsing a document from an XHR,
 *      the recommended approach is to do the XHR in the main thread, use
 *      XMLSerializer.serializeToString() on the responseXML, and pass the
 *      resulting string to the worker.
 *
 *   2) Live NodeLists are not supported. DOM methods and properties such as
 *      getElementsByTagName() and childNodes return standard arrays. If you
 *      want these lists to be updated when nodes are removed or added to the
 *      document, you must take care to manually update them yourself.
 */

package readability

import (
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const null = '\x00'

// XML only defines these and the numeric ones:

var entityReplacer = strings.NewReplacer(
	"lt", `<`,
	"gt", `>`,
	"amp", `&`,
	"quot", `"`,
	"apos", `'`,
)

var reverseEntitySubsetReplacer = strings.NewReplacer(
	`<`, "&lt;",
	`>`, "&gt;",
	`&`, "&amp;",
)

var reverseEntityReplacer = strings.NewReplacer(
	`<`, "&lt;",
	`>`, "&gt;",
	`&`, "&amp;",
	`"`, "&quot;",
	`'`, "&apos;",
)

func encodeTextContentHTML(text string) string {
	return reverseEntitySubsetReplacer.Replace(text)
}

func encodeHTML(text string) string {
	return reverseEntityReplacer.Replace(text)
}

func decodeHTML(s string) (string, error) {

	s = entityReferencesRgx.ReplaceAllStringFunc(s, func(s string) string {
		return string([]rune(entityReplacer.Replace(s))[1])
	})

	submatches := htmlCharCodesRgx.FindAllStringSubmatch(s, -1)
	for _, submatch := range submatches {
		if len(submatch) == 3 {
			hex, dec := submatch[1], submatch[2]
			if hex != "" {
				codePoint, err := strconv.ParseInt(hex, 16, 64)
				if err != nil {
					return "", err
				}
				s = strings.ReplaceAll(s, submatch[0], string(rune(codePoint)))
			} else if dec != "" {
				codePoint, err := strconv.ParseInt(dec, 10, 64)
				if err != nil {
					return "", err
				}
				s = strings.ReplaceAll(s, submatch[0], string(rune(codePoint)))
			}
		}
	}
	return s, nil
}

// When a style is set in JS, map it to the corresponding CSS attribute
var styleMap = map[string]string{
	"alignmentBaseline":          "alignment-baseline",
	"background":                 "background",
	"backgroundAttachment":       "background-attachment",
	"backgroundClip":             "background-clip",
	"backgroundColor":            "background-color",
	"backgroundImage":            "background-image",
	"backgroundOrigin":           "background-origin",
	"backgroundPosition":         "background-position",
	"backgroundPositionX":        "background-position-x",
	"backgroundPositionY":        "background-position-y",
	"backgroundRepeat":           "background-repeat",
	"backgroundRepeatX":          "background-repeat-x",
	"backgroundRepeatY":          "background-repeat-y",
	"backgroundSize":             "background-size",
	"baselineShift":              "baseline-shift",
	"border":                     "border",
	"borderBottom":               "border-bottom",
	"borderBottomColor":          "border-bottom-color",
	"borderBottomLeftRadius":     "border-bottom-left-radius",
	"borderBottomRightRadius":    "border-bottom-right-radius",
	"borderBottomStyle":          "border-bottom-style",
	"borderBottomWidth":          "border-bottom-width",
	"borderCollapse":             "border-collapse",
	"borderColor":                "border-color",
	"borderImage":                "border-image",
	"borderImageOutset":          "border-image-outset",
	"borderImageRepeat":          "border-image-repeat",
	"borderImageSlice":           "border-image-slice",
	"borderImageSource":          "border-image-source",
	"borderImageWidth":           "border-image-width",
	"borderLeft":                 "border-left",
	"borderLeftColor":            "border-left-color",
	"borderLeftStyle":            "border-left-style",
	"borderLeftWidth":            "border-left-width",
	"borderRadius":               "border-radius",
	"borderRight":                "border-right",
	"borderRightColor":           "border-right-color",
	"borderRightStyle":           "border-right-style",
	"borderRightWidth":           "border-right-width",
	"borderSpacing":              "border-spacing",
	"borderStyle":                "border-style",
	"borderTop":                  "border-top",
	"borderTopColor":             "border-top-color",
	"borderTopLeftRadius":        "border-top-left-radius",
	"borderTopRightRadius":       "border-top-right-radius",
	"borderTopStyle":             "border-top-style",
	"borderTopWidth":             "border-top-width",
	"borderWidth":                "border-width",
	"bottom":                     "bottom",
	"boxShadow":                  "box-shadow",
	"boxSizing":                  "box-sizing",
	"captionSide":                "caption-side",
	"clear":                      "clear",
	"clip":                       "clip",
	"clipPath":                   "clip-path",
	"clipRule":                   "clip-rule",
	"color":                      "color",
	"colorInterpolation":         "color-interpolation",
	"colorInterpolationFilters":  "color-interpolation-filters",
	"colorProfile":               "color-profile",
	"colorRendering":             "color-rendering",
	"content":                    "content",
	"counterIncrement":           "counter-increment",
	"counterReset":               "counter-reset",
	"cursor":                     "cursor",
	"direction":                  "direction",
	"display":                    "display",
	"dominantBaseline":           "dominant-baseline",
	"emptyCells":                 "empty-cells",
	"enableBackground":           "enable-background",
	"fill":                       "fill",
	"fillOpacity":                "fill-opacity",
	"fillRule":                   "fill-rule",
	"filter":                     "filter",
	"cssFloat":                   "float",
	"floodColor":                 "flood-color",
	"floodOpacity":               "flood-opacity",
	"font":                       "font",
	"fontFamily":                 "font-family",
	"fontSize":                   "font-size",
	"fontStretch":                "font-stretch",
	"fontStyle":                  "font-style",
	"fontVariant":                "font-variant",
	"fontWeight":                 "font-weight",
	"glyphOrientationHorizontal": "glyph-orientation-horizontal",
	"glyphOrientationVertical":   "glyph-orientation-vertical",
	"height":                     "height",
	"imageRendering":             "image-rendering",
	"kerning":                    "kerning",
	"left":                       "left",
	"letterSpacing":              "letter-spacing",
	"lightingColor":              "lighting-color",
	"lineHeight":                 "line-height",
	"listStyle":                  "list-style",
	"listStyleImage":             "list-style-image",
	"listStylePosition":          "list-style-position",
	"listStyleType":              "list-style-type",
	"margin":                     "margin",
	"marginBottom":               "margin-bottom",
	"marginLeft":                 "margin-left",
	"marginRight":                "margin-right",
	"marginTop":                  "margin-top",
	"marker":                     "marker",
	"markerEnd":                  "marker-end",
	"markerMid":                  "marker-mid",
	"markerStart":                "marker-start",
	"mask":                       "mask",
	"maxHeight":                  "max-height",
	"maxWidth":                   "max-width",
	"minHeight":                  "min-height",
	"minWidth":                   "min-width",
	"opacity":                    "opacity",
	"orphans":                    "orphans",
	"outline":                    "outline",
	"outlineColor":               "outline-color",
	"outlineOffset":              "outline-offset",
	"outlineStyle":               "outline-style",
	"outlineWidth":               "outline-width",
	"overflow":                   "overflow",
	"overflowX":                  "overflow-x",
	"overflowY":                  "overflow-y",
	"padding":                    "padding",
	"paddingBottom":              "padding-bottom",
	"paddingLeft":                "padding-left",
	"paddingRight":               "padding-right",
	"paddingTop":                 "padding-top",
	"page":                       "page",
	"pageBreakAfter":             "page-break-after",
	"pageBreakBefore":            "page-break-before",
	"pageBreakInside":            "page-break-inside",
	"pointerEvents":              "pointer-events",
	"position":                   "position",
	"quotes":                     "quotes",
	"resize":                     "resize",
	"right":                      "right",
	"shapeRendering":             "shape-rendering",
	"size":                       "size",
	"speak":                      "speak",
	"src":                        "src",
	"stopColor":                  "stop-color",
	"stopOpacity":                "stop-opacity",
	"stroke":                     "stroke",
	"strokeDasharray":            "stroke-dasharray",
	"strokeDashoffset":           "stroke-dashoffset",
	"strokeLinecap":              "stroke-linecap",
	"strokeLinejoin":             "stroke-linejoin",
	"strokeMiterlimit":           "stroke-miterlimit",
	"strokeOpacity":              "stroke-opacity",
	"strokeWidth":                "stroke-width",
	"tableLayout":                "table-layout",
	"textAlign":                  "text-align",
	"textAnchor":                 "text-anchor",
	"textDecoration":             "text-decoration",
	"textIndent":                 "text-indent",
	"textLineThrough":            "text-line-through",
	"textLineThroughColor":       "text-line-through-color",
	"textLineThroughMode":        "text-line-through-mode",
	"textLineThroughStyle":       "text-line-through-style",
	"textLineThroughWidth":       "text-line-through-width",
	"textOverflow":               "text-overflow",
	"textOverline":               "text-overline",
	"textOverlineColor":          "text-overline-color",
	"textOverlineMode":           "text-overline-mode",
	"textOverlineStyle":          "text-overline-style",
	"textOverlineWidth":          "text-overline-width",
	"textRendering":              "text-rendering",
	"textShadow":                 "text-shadow",
	"textTransform":              "text-transform",
	"textUnderline":              "text-underline",
	"textUnderlineColor":         "text-underline-color",
	"textUnderlineMode":          "text-underline-mode",
	"textUnderlineStyle":         "text-underline-style",
	"textUnderlineWidth":         "text-underline-width",
	"top":                        "top",
	"unicodeBidi":                "unicode-bidi",
	"unicodeRange":               "unicode-range",
	"vectorEffect":               "vector-effect",
	"verticalAlign":              "vertical-align",
	"visibility":                 "visibility",
	"whiteSpace":                 "white-space",
	"widows":                     "widows",
	"width":                      "width",
	"wordBreak":                  "word-break",
	"wordSpacing":                "word-spacing",
	"wordWrap":                   "word-wrap",
	"writingMode":                "writing-mode",
	"zIndex":                     "z-index",
	"zoom":                       "zoom",
}

// Elements that can be self-closing
var voidElems = map[string]bool{
	"area":    true,
	"base":    true,
	"br":      true,
	"col":     true,
	"command": true,
	"embed":   true,
	"hr":      true,
	"img":     true,
	"input":   true,
	"link":    true,
	"meta":    true,
	"param":   true,
	"source":  true,
	"wbr":     true,
}

var whitespaces = []rune{' ', '\t', '\n', '\r'}

// See https://developer.mozilla.org/en-US/docs/Web/API/Node/nodeType
const (
	_ = iota
	elementNode
	attributeNode
	textNode
	cdataSectionNode
	entityReferenceNode
	entityNode
	processingInstructionNode
	commentNode
	documentNode
	documentTypeNode
	documentFragmentNode
	notationNode
)

func (n *node) getElementsByTagName(tag string) []*node {

	tag = strings.ToUpper(tag)
	var elems []*node

	var allTags = tag == "*"

	var getElems func(from *node)

	getElems = func(from *node) {
		for i := 0; i < len(from.Children); i++ {
			child := from.Children[i]
			if allTags || child.TagName == tag {
				elems = append(elems, child)
			}
			getElems(child)
		}
	}

	getElems(n)

	return elems
}

type node struct {
	NodeType    uint
	LocalName   string
	nodeName    string
	textContent string
	innerHTML   string
	TagName     string
	Attributes  []*attribute
	style       *style
	// relations
	ParentNode             *node
	NextSibling            *node
	PreviousSibling        *node
	PreviousElementSibling *node
	NextElementSibling     *node
	ChildNodes             []*node
	Children               []*node
	// element
	matchingTag string
	// document
	DocumentURI          string
	baseURI              string
	title                string
	head                 *node
	Body                 *node
	DocumentElement      *node
	ReadabilityNode      *readabilityNode
	ReadabilityDataTable *readabilityDataTable
}

type readabilityDataTable struct {
	value bool
}

type readabilityNode struct {
	ContentScore float64
}

func (n *node) firstChild() *node {
	if len(n.ChildNodes) == 0 {
		return nil
	}
	return n.ChildNodes[0]
}

func (n *node) firstElementChild() *node {
	if len(n.Children) == 0 {
		return nil
	}
	return n.Children[0]
}

func (n *node) lastChild() *node {
	if len(n.ChildNodes) == 0 {
		return nil
	}
	return n.ChildNodes[len(n.ChildNodes)-1]
}

func (n *node) lastElementChild() *node {
	if len(n.ChildNodes) == 0 {
		return nil
	}
	return n.Children[len(n.Children)-1]
}

func (n *node) appendChild(child *node) {
	if child.ParentNode != nil {
		child.ParentNode.removeChild(child)
	}

	last := n.lastChild()
	if last != nil {
		last.NextSibling = child
	}
	child.PreviousSibling = last

	if child.NodeType == elementNode {
		if len(n.Children) != 0 {
			child.PreviousElementSibling = n.Children[len(n.Children)-1]
		}
		n.Children = append(n.Children, child)
		if child.PreviousElementSibling != nil {
			child.PreviousElementSibling.NextElementSibling = child
		}
	}

	n.ChildNodes = append(n.ChildNodes, child)
	child.ParentNode = n
}

func (n *node) removeChild(child *node) (*node, error) {
	childNodes := n.ChildNodes
	childIndex := indexOf(child, childNodes)
	if childIndex == -1 {
		return nil, fmt.Errorf("removeChild: node not found")
	} else {
		child.ParentNode = nil
		prev := child.PreviousSibling
		next := child.NextSibling
		if prev != nil {
			prev.NextSibling = next
		}
		if next != nil {
			next.PreviousSibling = prev
		}

		if child.NodeType == elementNode {
			prev = child.PreviousElementSibling
			next = child.NextElementSibling
			if prev != nil {
				prev.NextElementSibling = next
			}
			if next != nil {
				next.PreviousElementSibling = prev
			}
			n.Children = delete(indexOf(child, n.Children), n.Children)
		}

		child.PreviousSibling, child.NextSibling = nil, nil
		child.PreviousElementSibling, child.NextElementSibling = nil, nil

		n.ChildNodes = delete(childIndex, n.ChildNodes)
		return child, nil
	}
}

func (n *node) replaceChild(newNode, oldNode *node) *node {
	childNodes := n.ChildNodes
	childIndex := indexOf(oldNode, childNodes)
	if childIndex == -1 {
		panic("removeChild: node not found")
	} else {
		// This will take care of updating the new node if it was somewhere else before:
		if newNode.ParentNode != nil {
			newNode.ParentNode.removeChild(newNode)
		}
		childNodes[childIndex] = newNode

		// update the new node's sibling properties, and its new siblings' sibling properties
		newNode.NextSibling = oldNode.NextSibling
		newNode.PreviousSibling = oldNode.PreviousSibling
		if newNode.NextSibling != nil {
			newNode.NextSibling.PreviousSibling = newNode
		}
		if newNode.PreviousSibling != nil {
			newNode.PreviousSibling.NextSibling = newNode
		}

		newNode.ParentNode = n

		// Now deal with elements before we clear out those values for the old node,
		// because it can help us take shortcuts here:
		if newNode.NodeType == elementNode {
			if oldNode.NodeType == elementNode {
				// Both were elements, which makes this easier, we just swap things out:
				newNode.PreviousElementSibling = oldNode.PreviousElementSibling
				newNode.NextElementSibling = oldNode.NextElementSibling
				if newNode.PreviousElementSibling != nil {
					newNode.PreviousElementSibling.NextElementSibling = newNode
				}
				if newNode.NextElementSibling != nil {
					newNode.NextElementSibling.PreviousElementSibling = newNode
				}
				n.Children[indexOf(oldNode, n.Children)] = newNode
			} else {
				// Hard way:
				newNode.PreviousElementSibling = func(childIndex int, childNodes []*node) *node {
					for i := childIndex - 1; i >= 0; i-- {
						if childNodes[i].NodeType == elementNode {
							return childNodes[i]
						}
					}
					return nil
				}(childIndex, childNodes)
				if newNode.PreviousElementSibling != nil {
					newNode.NextElementSibling = newNode.PreviousElementSibling.NextElementSibling
				} else {
					newNode.NextElementSibling = func(childIndex int, childNodes []*node) *node {
						for i := childIndex + 1; i < len(childNodes); i++ {
							if childNodes[i].NodeType == elementNode {
								return childNodes[i]
							}
						}
						return nil
					}(childIndex, childNodes)
				}
				if newNode.PreviousElementSibling != nil {
					newNode.PreviousElementSibling.NextElementSibling = newNode
				}
				if newNode.NextElementSibling != nil {
					newNode.NextElementSibling.PreviousElementSibling = newNode
				}

				if newNode.NextElementSibling != nil {
					n.Children = insert(newNode, indexOf(newNode.NextElementSibling, n.Children), n.Children)
				} else {
					n.Children = append(n.Children, newNode)
				}
			}
		} else if oldNode.NodeType == elementNode {
			// new node is not an element node.
			// if the old one was, update its element siblings:
			if oldNode.PreviousElementSibling != nil {
				oldNode.PreviousElementSibling.NextElementSibling = oldNode.NextElementSibling
			}
			if oldNode.NextElementSibling != nil {
				oldNode.NextElementSibling.PreviousElementSibling = oldNode.PreviousElementSibling
			}
			n.Children = delete(indexOf(oldNode, n.Children), n.Children)

			// If the old node wasn't an element, neither the new nor the old node was an element,
			// and the children array and its members shouldn't need any updating.
		}

		oldNode.ParentNode = nil
		oldNode.PreviousSibling = nil
		oldNode.NextSibling = nil
		if oldNode.NodeType == elementNode {
			oldNode.PreviousElementSibling = nil
			oldNode.NextElementSibling = nil
		}
		return oldNode
	}
}

type attribute struct {
	name, value string
}

func newAttribute(n, v string) *attribute {
	return &attribute{
		name:  n,
		value: v,
	}
}

func (a *attribute) getName() string {
	return a.name
}

func (a *attribute) getValue() string {
	return a.value
}

func (a *attribute) setValue(newValue string) {
	a.value = newValue
}

func (a *attribute) getEncodedValue() string {
	return encodeHTML(a.value)
}

func newComment() *node {
	return &node{
		nodeName: "#comment",
		NodeType: commentNode,
	}
}

func newText() *node {
	return &node{
		nodeName:    "#text",
		NodeType:    textNode,
		innerHTML:   "",
		textContent: "",
	}
}

func (t *node) getTextContentFromTextNode() string {
	if t.textContent == "" {
		decoded, err := decodeHTML(t.getInnerHTML())
		if err != nil {
			slog.Error("cannot decode inner html", "err", err)
			return ""
		}
		t.textContent = decoded
	}
	return t.textContent
}

func (t *node) getInnerHTMLFromTextNode() string {
	if t.innerHTML == "" {
		t.innerHTML = encodeTextContentHTML(t.getTextContent())
	}
	return t.innerHTML
}

func (t *node) setInnerHTMLFromTextNode(newHTML string) {
	t.innerHTML = newHTML
	t.textContent = ""
}

func (t *node) setTextContentFromTextNode(newText string) {
	t.textContent = newText
	t.innerHTML = ""
}

func newDocument(url string) *node {
	return &node{
		DocumentURI: url,
		nodeName:    "#document",
		NodeType:    documentNode,
	}
}

func (n *node) getElementById(id string) *node {

	var getElem func(from *node) *node

	getElem = func(from *node) *node {
		var length = len(from.Children)
		if from.getId() == id {
			return from
		}
		for i := 0; i < length; i++ {
			var el = getElem(from.Children[i])
			if el != nil {
				return el
			}
		}
		return nil
	}

	return getElem(n)
}

func (d *node) createElement(tag string) *node {
	return newElement(tag)
}

func (d *node) createTextNode(text string) *node {
	node := newText()
	node.setTextContent(text)
	return node
}

func (d *node) getBaseURI() string {
	if d.baseURI == "" {
		d.baseURI = d.DocumentURI
		baseElements := d.getElementsByTagName("base")
		if len(baseElements) != 0 {
			href := baseElements[0].getAttribute("href")
			if href != "" {
				base, err := url.Parse(d.baseURI)
				if err != nil {
					// just fall back to documentURI
					return d.DocumentURI
				}
				ref, err := url.Parse(href)
				if err != nil {
					// just fall back to documentURI
					return d.DocumentURI
				}
				u := base.ResolveReference(ref)
				d.baseURI = u.String()
			}
		}
	}
	return d.baseURI
}

func newElement(tag string) *node {
	n := &node{
		// We use this to find the closing tag.
		matchingTag: tag,
		NodeType:    elementNode,
	}
	// We're explicitly a non-namespace aware parser, we just pretend it's all HTML.
	var lastColonIndex = strings.LastIndex(tag, ":")
	if lastColonIndex != -1 {
		substrings := strings.Split(tag, ":")
		tag = substrings[len(substrings)-1]
	}

	n.LocalName = strings.ToLower(tag)
	n.TagName = strings.ToUpper(tag)
	n.style = newStyle(n)

	return n
}

func (n *node) getAttribute(name string) string {
	var i = len(n.Attributes) - 1
	for i >= 0 {
		var attr = n.Attributes[i]
		if attr.name == name {
			return attr.value
		}
		i--
	}
	return ""
}

func (n *node) getAttributeByIndex(idx int) *attribute {
	return n.Attributes[idx]
}

func (n *node) getAttributeLen() int {
	return len(n.Attributes)
}

func (n *node) setAttribute(name, value string) {
	for _, attr := range n.Attributes {
		if attr.name == name {
			attr.setValue(value)
			return
		}
	}
	n.Attributes = append(n.Attributes, newAttribute(name, value))
}

func (n *node) removeAttribute(name string) {
	for idx, attr := range n.Attributes {
		if attr.name == name {
			n.Attributes = delete(idx, n.Attributes)
			break
		}
	}
}

func (n *node) hasAttribute(name string) bool {
	return slices.ContainsFunc[[]*attribute, *attribute](n.Attributes, func(a *attribute) bool {
		return a.name == name
	})
}

type style struct {
	*node
}

func newStyle(n *node) *style {
	return &style{
		node: n,
	}
}

func (s *style) getStyle(jsName string) string {

	var cssName = styleMap[jsName]

	var attr = s.node.getAttribute("style")
	if attr == "" {
		return ""
	}

	var styles = strings.Split(attr, ";")
	for i := 0; i < len(styles); i++ {
		var style = strings.Split(styles[i], ":")
		var name = strings.TrimSpace(style[0])
		if name == cssName {
			return strings.TrimSpace(style[1])
		}
	}
	return ""
}

func (s *style) setStyle(jsName, styleValue string) {

	var cssName = styleMap[jsName]

	var value = s.node.getAttribute("style")
	var index = 0
	for index >= 0 {
		var next = indexOfFrom(value, ";", index)
		var length = next - index - 1
		var style string
		if length > 0 {
			style = substring(value, index, length)
		} else {
			style = substring(value, index, len(style))
		}
		substr := substring(style, 0, strings.IndexRune(style, ':'))
		if strings.TrimSpace(substr) == cssName {
			value = strings.TrimSpace(substring(value, 0, index))
			if next >= 0 {
				value += " " + strings.TrimSpace(substring(value, next, len((value))))
			}
		}
		index = next
	}
	value += " " + cssName + ": " + styleValue + ";"
	s.node.setAttribute("style", strings.TrimSpace(value))
}

func (n *node) getClassName() string {
	return n.getAttribute("class")
}

func (n *node) setClassName(str string) {
	n.setAttribute("class", str)
}

func (n *node) getId() string {
	return n.getAttribute("id")
}

func (n *node) setId(str string) {
	n.setAttribute("id", str)
}

func (n *node) getHref() string {
	return n.getAttribute("href")
}

func (n *node) setHref(str string) {
	n.setAttribute("href", str)
}

func (n *node) getSrc() string {
	return n.getAttribute("src")
}

func (n *node) setSrc(str string) {
	n.setAttribute("src", str)
}

func (n *node) getSrcset() string {
	return n.getAttribute("srcset")
}

func (n *node) setSrcset(str string) {
	n.setAttribute("srcset", str)
}

func (n *node) getNodeName() string {
	return n.TagName
}

func (n *node) getInnerHTML() string {

	if n.NodeType == textNode {
		return n.getInnerHTMLFromTextNode()
	}

	var getHTML func(from *node, a []string) []string
	getHTML = func(from *node, a []string) []string {
		for i := 0; i < len(from.ChildNodes); i++ {
			var child = from.ChildNodes[i]
			if child.LocalName != "" {
				a = append(a, "<"+child.LocalName)

				// serialize attribute list
				for j := 0; j < len(child.Attributes); j++ {
					var attr = child.Attributes[j]
					// the attribute value will be HTML escaped.
					var val = attr.getEncodedValue()
					var quote = `"`
					if strings.Contains(val, `"`) {
						quote = "'"
					}
					a = append(a, " "+attr.name+"="+quote+val+quote)
				}

				if _, found := voidElems[child.LocalName]; found && len(child.ChildNodes) != 0 {
					// if this is a self-closing element, end it here
					a = append(a, "/>")
				} else {
					// otherwise, add its children
					a = append(a, ">")
					a = getHTML(child, a)
					a = append(a, "</"+child.LocalName+">")
				}
			} else {
				// This is a text node, so asking for innerHTML won't recurse.
				a = append(a, child.getInnerHTML())
			}
		}
		return a
	}

	var arr []string
	arr = getHTML(n, arr)
	return strings.Join(arr, "")
}

func (n *node) setInnerHTML(html string) {

	if n.NodeType == textNode {
		n.setInnerHTMLFromTextNode(html)
	} else if n.NodeType == elementNode {
		var parser = newDOMParser()
		var node = parser.parse(html, "")
		for i := len(n.ChildNodes) - 1; i >= 0; i-- {
			n.ChildNodes[i].ParentNode = nil
		}
		n.ChildNodes = node.ChildNodes
		n.Children = node.Children
		for i := len(n.ChildNodes) - 1; i >= 0; i-- {
			n.ChildNodes[i].ParentNode = n
		}
	} else {
		n.innerHTML = html
	}
}

func (n *node) setTextContent(text string) {

	if n.NodeType == textNode {
		n.setTextContentFromTextNode(text)
		return
	} else if n.NodeType == elementNode {
		// clear parentNodes for existing children
		for i := len(n.ChildNodes) - 1; i >= 0; i-- {
			n.ChildNodes[i].ParentNode = nil
		}

		var t = newText()
		n.ChildNodes = []*node{t}
		n.Children = []*node{}
		t.textContent = text
		t.ParentNode = n
	} else {
		n.textContent = text
	}

}

func (n *node) getTextContent() string {

	if n.NodeType == textNode {
		return n.getTextContentFromTextNode()
	} else if n.NodeType == elementNode {
		var getText func(*node, []string) []string
		getText = func(from *node, t []string) []string {
			var nodes = from.ChildNodes
			for i := 0; i < len(nodes); i++ {
				var child = nodes[i]
				if child.NodeType == textNode {
					t = append(t, child.getTextContent())
				} else {
					t = getText(child, t)
				}
			}
			return t
		}

		text := make([]string, 0)
		text = getText(n, text)
		return strings.Join(text, "")
	} else {
		return n.textContent
	}
}

type domParser struct {
	currentChar int
	errorState  string
	html        string
	doc         *node
	logger      *slog.Logger
	options     *Options
}

func newDOMParser(opts ...Option) *domParser {
	p := &domParser{
		options: defaultOpts(),
	}
	// Configurable options
	for _, opt := range opts {
		opt(p.options)
	}

	p.logger = loggerWith(p.options.logLevel)
	return p
}

// Look at the next character without advancing the index.
func (p *domParser) peekNext() (rune, int) {
	if p.currentChar >= len(p.html) {
		return null, 0
	}
	char, width := utf8.DecodeRuneInString(p.html[p.currentChar:])
	if char == utf8.RuneError && width == 0 {
		p.logger.Error("next char is empty", "current_pos", p.currentChar)
		return null, 0
	}
	if char == utf8.RuneError && width == 1 {
		p.logger.Error("next char is utf8-invalid", "current_pos", p.currentChar)
		return null, 0
	}
	return char, width
}

// Get the next character and advance the index.
func (p *domParser) nextChar() rune {
	char, width := p.peekNext()
	p.currentChar += width
	return char
}

// Called after a quote character is read. This finds the next quote
// character and returns the text string in between.
func (p *domParser) readString(quote string) string {
	var str string
	var n = indexOfFrom(p.html, quote, p.currentChar)
	if n == -1 {
		p.currentChar = len(p.html)
	} else {
		str = substring(p.html, p.currentChar, n)
		p.currentChar = n + 1
	}
	return str
}

// Called when parsing a node. This finds the next name/value attribute
// pair and adds the result to the attributes list.
func (p *domParser) readAttribute(n *node) {
	var name string
	num := indexOfFrom(p.html, "=", p.currentChar)
	if num == -1 {
		p.currentChar = len(p.html)
	} else {
		// Read until a '=' character is hit; this will be the attribute key
		name = substring(p.html, p.currentChar, num)
		p.currentChar = num + 1
	}

	if name == "" {
		return
	}
	// After a '=', we should see a '"' for the attribute value
	var c = p.nextChar()
	if c != '"' && c != '\'' {
		p.logger.Error("Error reading attribute " + name + ", expecting '\"'")
		return
	}
	// Read the attribute value (and consume the matching quote)
	var value = p.readString(string(c))

	decoded, err := decodeHTML(value)
	if err != nil {
		p.logger.Error(err.Error())
	} else {
		n.Attributes = append(n.Attributes, newAttribute(name, decoded))
	}
}

// Parses and returns an Element node. This is called after a '<' has been
// read.
// Returns an array; the first index of the array is the parsed node;
// the second index is a boolean indicating whether this is a void Element
func (p *domParser) makeElementNode() (*node, bool) {

	var c = p.nextChar()
	// Read the Element tag name
	var strBuf []rune
	strBuf = []rune{}
	for !slices.Contains(whitespaces, c) && c != '>' && c != '/' {
		if c == null {
			return nil, false
		}
		strBuf = append(strBuf, c)
		c = p.nextChar()
	}
	var tag = string(strBuf)
	if tag == "" {
		return nil, false
	}

	var node = newElement(tag)
	// Read Element attributes
	for c != '/' && c != '>' {
		if c == null {
			return nil, false
		}

		c = p.nextChar()
		for slices.Contains(whitespaces, c) {
			// Advance cursor to first non-whitespace char.
			c = p.nextChar()
		}
		if c != '/' && c != '>' {
			p.currentChar -= len(string(c))
			p.readAttribute(node)
		}
	}

	// If this is a self-closing tag, read '/>'
	var closed bool
	if c == '/' {
		closed = true
		c = p.nextChar()
		if c != '>' {
			p.logger.Error("expected '>' to close " + tag)
			return nil, false
		}
	}

	return node, closed
}

// If the current input matches this string, advance the input index;
// otherwise, do nothing. Returns whether input matched string
func (p *domParser) match(str string) bool {
	var strlen = len(str)
	if strings.EqualFold(substring(p.html, p.currentChar, p.currentChar+strlen), str) {
		p.currentChar += strlen
		return true
	}
	return false
}

// Searches the input until a string is found and discards all input up to
// and including the matched string.
func (p *domParser) discardTo(str string) {
	var index = indexOfFrom(p.html, str, p.currentChar) + len(str)
	if index == -1 {
		p.currentChar = len(p.html)
	}
	p.currentChar = index
}

// Reads child nodes for the given node.
func (p *domParser) readChildren(n *node) {
	var child = p.readNode()
	for child != nil {
		// Don't keep Comment nodes
		if child.NodeType != 8 {
			n.appendChild(child)
		}
		child = p.readNode()
	}
}

func (p *domParser) discardNextComment() *node {
	if p.match("--") {
		p.discardTo("-->")
	} else {
		var c = p.nextChar()
		for c != '>' {
			if c == null {
				return nil
			}
			if c == '"' || c == '\'' {
				p.readString(string(c))
			}
			c = p.nextChar()
		}
	}
	return newComment()
}

// Reads the next child node from the input. If we're reading a closing
// tag, or if we've reached the end of input, return null. Returns the node
func (p *domParser) readNode() *node {
	var c = p.nextChar()
	if c == null {
		return nil
	}
	p.logger.Info("p.Readnode", "currentChar", string(c), "at", p.currentChar)
	// Read any text as Text node
	var textNode *node
	if c != '<' {
		p.currentChar -= len(string(c))
		textNode = newText()
		var n = indexOfFrom(p.html, "<", p.currentChar)
		if n == -1 {
			textNode.setInnerHTML(substring(p.html, p.currentChar, len(p.html)))
			p.currentChar = len(p.html)
		} else {
			textNode.setInnerHTML(substring(p.html, p.currentChar, n))
			p.logger.Info("p.Readnode", "textNode.innerHTML", textNode.innerHTML)
			p.currentChar = n
		}
		return textNode
	}

	if p.match("![CDATA[") {
		var endChar = indexOfFrom(p.html, "]]>", p.currentChar)
		if endChar == -1 {
			p.logger.Error("unclosed CDATA section")
			return nil
		}
		textNode = newText()
		textNode.setTextContent(substring(p.html, p.currentChar, endChar))
		p.currentChar = endChar + len("]]>")
		return textNode
	}

	c, _ = p.peekNext()

	// Read Comment node. Normally, Comment nodes know their inner
	// textContent, but we don't really care about Comment nodes (we throw
	// them away in readChildren()). So just returning an empty Comment node
	// here is sufficient.
	if c == '!' || c == '?' {
		p.currentChar++
		return p.discardNextComment()
	}

	// If we're reading a closing tag, return null. This means we've reached
	// the end of this set of child nodes.
	if c == '/' {
		p.currentChar--
		return nil
	}

	// Otherwise, we're looking at an Element node
	node, closed := p.makeElementNode()
	if node == nil {
		return nil
	}

	var localName = node.LocalName

	// If this isn't a void Element, read its child nodes
	if !closed {
		p.readChildren(node)
		var closingTag = "</" + node.matchingTag + ">"
		p.logger.Debug("matching", "currentChar", p.currentChar, "closingTag", closingTag)
		if !p.match(closingTag) {
			p.logger.Error("expected " + closingTag + " and got " + substring(p.html, p.currentChar, p.currentChar+len(closingTag)))
			return nil
		}
	}
	// Only use the first title, because SVG might have other
	// title elements which we don't care about (medium.com
	// does this, at least).
	if localName == "title" && p.doc.title == "" {
		p.doc.title = strings.TrimSpace(node.getTextContent())
	} else if localName == "head" {
		p.doc.head = node
	} else if localName == "body" {
		p.doc.Body = node
	} else if localName == "html" {
		p.doc.DocumentElement = node
	}

	return node
}

// Parses an HTML string and returns a JS implementation of the Document.
func (p *domParser) parse(html, url string) *node {
	p.html = html
	p.doc = newDocument(url)
	p.readChildren(p.doc)

	// If this is an HTML document, remove root-level children except for the
	// <html> node
	if p.doc.DocumentElement != nil {
		var i = len(p.doc.ChildNodes) - 1
		for i >= 0 {
			var child = p.doc.ChildNodes[i]
			if child != p.doc.DocumentElement {
				p.doc.removeChild(child)
			}
			i--
		}
	}
	return p.doc
}
