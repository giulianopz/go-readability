package readability

import (
	"bytes"
	"slices"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

func indexOf[T any](el *T, a []*T) int {
	return slices.IndexFunc(a, func(ell *T) bool {
		return ell == el
	})
}

func delete[T any](idx int, a []*T) []*T {
	copy(a[idx:], a[idx+1:])
	a[len(a)-1] = nil
	a = a[:len(a)-1]
	return a
}

func insert(newNode *Node, idx int, nodes []*Node) []*Node {
	nodes = append(nodes[:idx], append([]*Node{newNode}, nodes[idx:]...)...)
	return nodes
}

func substring(str string, s, e int) string {
	// If indexStart is greater than indexEnd, then the effect of substring() is as if the two arguments were swapped.
	// Any argument value that is less than 0 or greater than str.length is treated as if it were 0 and str.length, respectively.
	// See: https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/String/substring
	var indexStart, indexEnd, max int = s, e, len(str)
	if indexStart < 0 {
		indexStart = 0
	}
	if indexEnd > max {
		indexEnd = max
	}
	if indexStart > indexEnd {
		indexStart, indexEnd = indexEnd, indexStart
	}
	ret := str[indexStart:indexEnd]
	return ret
}

func anyOf(strings ...string) string {
	for _, s := range strings {
		if s != "" {
			return s
		}
	}
	return ""
}

func querySelectorAll(n *html.Node, query string) []*html.Node {
	sel, err := cascadia.ParseGroup(query)
	if err != nil {
		return nil
	}
	return cascadia.QueryAll(n, sel)
}

func matches(n *html.Node, query string) bool {
	sel, err := cascadia.Parse(query)
	if err != nil {
		return false
	}
	return cascadia.Query(n, sel) != nil
}

func attr(n *html.Node, attrName string) string {
	for _, a := range n.Attr {
		if a.Key == attrName {
			return a.Val
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	var buf bytes.Buffer

	var getText func(*html.Node)
	getText = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			getText(child)
		}
	}
	getText(n)

	return buf.String()
}
