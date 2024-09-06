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
