package readability

import (
	"strings"

	"golang.org/x/net/html"
)

func mapDoc(doc *html.Node, url string) *node {

	ret := newDocument(url)

	var f func(*html.Node, *node)
	f = func(from *html.Node, to *node) {

		for c := from.FirstChild; c != nil; c = c.NextSibling {

			mapped := mapNode(c)

			if mapped == nil {
				continue
			}

			to.appendChild(mapped)

			// set doc elements
			if mapped.NodeType == elementNode {
				if mapped.LocalName == "title" && ret.title == "" {
					ret.title = strings.TrimSpace(mapped.getTextContent())
				} else if mapped.LocalName == "head" {
					ret.head = mapped
				} else if mapped.LocalName == "body" {
					ret.Body = mapped
				} else if mapped.LocalName == "html" {
					ret.DocumentElement = mapped
				}
			}
			f(c, mapped)
		}
	}
	f(doc, ret)

	return ret
}

func mapNode(from *html.Node) *node {

	var to *node

	switch from.Type {

	case html.ElementNode:
		to = newElement(from.Data)
		for _, a := range from.Attr {
			to.setAttribute(a.Key, a.Val)
		}

	case html.TextNode:
		to = newText()
		to.setInnerHTML(from.Data)
		to.setTextContent(from.Data)
	}

	return to
}
