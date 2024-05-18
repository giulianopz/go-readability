package readability

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var baseTestCase = `<html><body><p>Some text and <a class="someclass" href="#">a link</a></p>` +
	`<div id="foo">With a <script>With &lt; fancy " characters in it because` +
	`</script> that is fun.<span>And another node to make it harder</span></div><form><input type="text"/><input type="number"/>Here\'s a form</form></body></html>`

func TestDecodeHTML(t *testing.T) {

	// https://www.toptal.com/designers/htmlarrows/symbols/
	testCases := []struct {
		input string
		want  string
	}{
		{
			input: "&#xa7;",
			want:  "§",
		},
		{
			input: "&#167;",
			want:  "§",
		},
		{
			input: "&#x2766;",
			want:  "❦",
		},
		{
			input: "&#10086;",
			want:  "❦",
		},
		{
			input: `With &lt; fancy " characters in it because`,
			want:  `With < fancy " characters in it because`,
		},
	}

	for _, tc := range testCases {
		got, err := decodeHTML(tc.input)
		assert.NoError(t, err)
		if got != tc.want {
			t.Errorf("got %v want %v", got, tc.want)
		}
	}

}

func TestJSDOM_Functionality(t *testing.T) {

	t.Run("should work for basic operations using the parent child hierarchy and innerHTML", func(t *testing.T) {
		baseDoc := newDOMParser().parse(baseTestCase, "http://fakehost/")

		assert.Equal(t, 1, len(baseDoc.ChildNodes))
		assert.Equal(t, 10, len(baseDoc.getElementsByTagName("*")))

		var foo = baseDoc.getElementById("foo")
		assert.Equal(t, "body", foo.ParentNode.LocalName)
		assert.Equal(t, baseDoc.Body, foo.ParentNode)
		assert.Equal(t, baseDoc.Body.ParentNode, baseDoc.DocumentElement)
		assert.Equal(t, 3, len(baseDoc.Body.ChildNodes))

		var generatedHTML = baseDoc.getElementsByTagName("p")[0].getInnerHTML()
		assert.Equal(t, `Some text and <a class="someclass" href="#">a link</a>`, generatedHTML)
		var scriptNode = baseDoc.getElementsByTagName("script")[0]
		generatedHTML = scriptNode.getInnerHTML()
		assert.Equal(t, `With &lt; fancy " characters in it because`, generatedHTML)
		assert.Equal(t, `With < fancy " characters in it because`, scriptNode.getTextContent())
	})

	t.Run("should have basic URI information", func(t *testing.T) {
		baseDoc := newDOMParser().parse(baseTestCase, "http://fakehost/")
		assert.Equal(t, "http://fakehost/", baseDoc.DocumentURI)
		assert.Equal(t, "http://fakehost/", baseDoc.getBaseURI())
	})

	t.Run("should deal with script tags", func(t *testing.T) {
		// Check our script parsing worked:
		baseDoc := newDOMParser().parse(baseTestCase, "http://fakehost/")
		var scripts = baseDoc.getElementsByTagName("script")
		assert.Equal(t, 1, len(scripts))
		assert.Equal(t, "With < fancy \" characters in it because", scripts[0].getTextContent())
	})

	t.Run("should have working sibling/first+lastChild properties", func(t *testing.T) {
		baseDoc := newDOMParser().parse(baseTestCase, "http://fakehost/")

		var foo = baseDoc.getElementById("foo")
		assert.Equal(t, foo.PreviousSibling.NextSibling, foo)
		assert.Equal(t, foo.NextSibling.PreviousSibling, foo)
		assert.Equal(t, foo.NextSibling, foo.NextElementSibling)
		assert.Equal(t, foo.PreviousSibling, foo.PreviousElementSibling)

		var beforeFoo = foo.PreviousSibling
		var afterFoo = foo.NextSibling

		assert.Equal(t, baseDoc.Body.lastChild(), afterFoo)
		assert.Equal(t, baseDoc.Body.firstChild(), beforeFoo)
	})

	t.Run("should have working removeChild and appendChild functionality", func(t *testing.T) {
		baseDoc := newDOMParser().parse(baseTestCase, "http://fakehost/")

		var foo = baseDoc.getElementById("foo")
		var beforeFoo = foo.PreviousSibling
		var afterFoo = foo.NextSibling

		var removedFoo, err = foo.ParentNode.removeChild(foo)
		assert.NoError(t, err)
		assert.Equal(t, foo, removedFoo)
		assert.Nil(t, foo.ParentNode)
		assert.Nil(t, foo.PreviousSibling)
		assert.Nil(t, foo.NextSibling)
		assert.Nil(t, foo.PreviousElementSibling)
		assert.Nil(t, foo.NextElementSibling)

		assert.Equal(t, "p", beforeFoo.LocalName)
		assert.Equal(t, beforeFoo.NextSibling, afterFoo)
		assert.Equal(t, afterFoo.PreviousSibling, beforeFoo)
		assert.Equal(t, beforeFoo.NextElementSibling, afterFoo)
		assert.Equal(t, afterFoo.PreviousElementSibling, beforeFoo)

		assert.Equal(t, 2, len(baseDoc.Body.ChildNodes))

		baseDoc.Body.appendChild(foo)

		assert.Equal(t, 3, len(baseDoc.Body.ChildNodes))
		assert.Equal(t, afterFoo.NextSibling, foo)
		assert.Equal(t, foo.PreviousSibling, afterFoo)
		assert.Equal(t, afterFoo.NextElementSibling, foo)
		assert.Equal(t, foo.PreviousElementSibling, afterFoo)

		// This should reorder back to sanity:
		baseDoc.Body.appendChild(afterFoo)
		assert.Equal(t, foo.PreviousSibling, beforeFoo)
		assert.Equal(t, foo.NextSibling, afterFoo)
		assert.Equal(t, foo.PreviousElementSibling, beforeFoo)
		assert.Equal(t, foo.NextElementSibling, afterFoo)

		assert.Equal(t, foo.PreviousSibling.NextSibling, foo)
		assert.Equal(t, foo.NextSibling.PreviousSibling, foo)
		assert.Equal(t, foo.NextSibling, foo.NextElementSibling)
		assert.Equal(t, foo.PreviousSibling, foo.PreviousElementSibling)
	})

	t.Run("should handle attributes", func(t *testing.T) {
		baseDoc := newDOMParser().parse(baseTestCase, "http://fakehost/")

		var link = baseDoc.getElementsByTagName("a")[0]
		assert.Equal(t, "#", link.getAttribute("href"))
		assert.Equal(t, link.getClassName(), link.getAttribute("class"))
		var foo = baseDoc.getElementById("foo")
		assert.Equal(t, foo.getAttribute("id"), foo.getId())
	})

	t.Run("should have a working replaceChild", func(t *testing.T) {
		baseDoc := newDOMParser().parse(baseTestCase, "http://fakehost/")

		var parent = baseDoc.getElementsByTagName("div")[0]
		var p = baseDoc.createElement("p")
		p.setAttribute("id", "my-replaced-kid")
		var childCount = len(parent.ChildNodes)
		var childElCount = len(parent.Children)

		for i := 0; i < len(parent.ChildNodes); i++ {

			var replacedNode = parent.ChildNodes[i]
			var replacedAnElement = replacedNode.NodeType == elementNode
			var oldNext = replacedNode.NextSibling
			var oldNextEl = replacedNode.NextElementSibling
			var oldPrev = replacedNode.PreviousSibling
			var oldPrevEl = replacedNode.PreviousElementSibling

			parent.replaceChild(p, replacedNode)

			// Check siblings and parents on both nodes were set:
			assert.Equal(t, p.NextSibling, oldNext)
			assert.Equal(t, p.PreviousSibling, oldPrev)
			assert.Equal(t, p.ParentNode, parent)

			assert.Nil(t, replacedNode.ParentNode)
			assert.Nil(t, replacedNode.NextSibling)
			assert.Nil(t, replacedNode.PreviousSibling)

			// if the old node was an element, element siblings should now be null
			if replacedAnElement {
				assert.Nil(t, replacedNode.NextElementSibling)
				assert.Nil(t, replacedNode.PreviousElementSibling)
			}

			// Check the siblings were updated
			if oldNext != nil {
				assert.Equal(t, oldNext.PreviousSibling, p)
			}
			if oldPrev != nil {
				assert.Equal(t, oldPrev.NextSibling, p)
			}

			// check the array was updated
			assert.Equal(t, parent.ChildNodes[i], p)

			// Now check element properties/lists:
			var kidElementIndex = slices.IndexFunc(parent.Children, func(n *node) bool {
				return n == p
			})

			// should be in the list:
			assert.NotEqual(t, -1, kidElementIndex)

			if kidElementIndex > 0 {
				assert.Equal(t, parent.Children[kidElementIndex-1], p.PreviousElementSibling)
				assert.Equal(t, p.PreviousElementSibling.NextElementSibling, p)
			} else {
				assert.Nil(t, p.PreviousElementSibling)
			}

			if kidElementIndex < len(parent.Children)-1 {
				assert.Equal(t, parent.Children[kidElementIndex+1], p.NextElementSibling)
				assert.Equal(t, p.NextElementSibling.PreviousElementSibling, p)
			} else {
				assert.Nil(t, p.NextElementSibling)
			}

			if replacedAnElement {
				assert.Equal(t, oldNextEl, p.NextElementSibling)
				assert.Equal(t, oldPrevEl, p.PreviousElementSibling)
			}

			assert.Equal(t, childCount, len(parent.ChildNodes))
			if replacedAnElement {
				assert.Equal(t, childElCount, len(parent.Children))
			} else {
				assert.Equal(t, childElCount+1, len(parent.Children))
			}

			parent.replaceChild(replacedNode, p)

			assert.Equal(t, oldNext, replacedNode.NextSibling)
			assert.Equal(t, oldNextEl, replacedNode.NextElementSibling)
			assert.Equal(t, oldPrev, replacedNode.PreviousSibling)
			assert.Equal(t, oldPrevEl, replacedNode.PreviousElementSibling)
			if replacedNode.NextSibling != nil {
				assert.Equal(t, replacedNode.NextSibling.PreviousSibling, replacedNode)
			}
			if replacedNode.PreviousSibling != nil {
				assert.Equal(t, replacedNode.PreviousSibling.NextSibling, replacedNode)
			}
			if replacedAnElement {
				if replacedNode.PreviousElementSibling != nil {
					assert.Equal(t, replacedNode.PreviousElementSibling.NextElementSibling, replacedNode)
				}
				if replacedNode.NextElementSibling != nil {
					assert.Equal(t, replacedNode.NextElementSibling.PreviousElementSibling, replacedNode)
				}
			}
		}
	})
}

func TestHTML_Escaping(t *testing.T) {
	var baseStr = "<p>Hello, everyone &amp; all their friends, &lt;this&gt; is a &quot; test with &apos; quotes.</p>"
	var doc = newDOMParser().parse(baseStr, "")
	var p = doc.getElementsByTagName("p")[0]
	var txtNode = p.firstChild()

	t.Run("should handle encoding HTML correctly", func(t *testing.T) {
		// This /should/ just be cached straight from reading it:
		assert.Equal(t, baseStr, "<p>"+p.getInnerHTML()+"</p>")
		assert.Equal(t, baseStr, "<p>"+txtNode.getInnerHTML()+"</p>")
	})

	t.Run("should have decoded correctly", func(t *testing.T) {
		// This /should/ just be cached straight from reading it:
		assert.Equal(t, "Hello, everyone & all their friends, <this> is a \" test with ' quotes.", p.getTextContent())
		assert.Equal(t, "Hello, everyone & all their friends, <this> is a \" test with ' quotes.", txtNode.getTextContent())

	})

	t.Run("should handle updates via textContent correctly", func(t *testing.T) {
		// Because the initial tests might be based on cached innerHTML values,
		// let's manipulate via textContent in order to test that it alters
		// the innerHTML correctly.
		txtNode.setTextContent(txtNode.getTextContent() + " ")
		txtNode.setTextContent(strings.TrimSpace(txtNode.getTextContent()))
		var expectedHTML = strings.NewReplacer(`&quot;`, `"`, `&apos;`, `'`).Replace(baseStr)
		assert.Equal(t, expectedHTML, "<p>"+txtNode.getInnerHTML()+"</p>")
		assert.Equal(t, expectedHTML, "<p>"+p.getInnerHTML()+"</p>")
	})
}

func TestScript_Parsing(t *testing.T) {

	t.Run("should strip ?-based comments within script tags", func(t *testing.T) {
		var html = `<script><?Silly test <img src="test"></script>`
		var doc = newDOMParser().parse(html, "")
		assert.Equal(t, "SCRIPT", doc.firstChild().TagName)
		assert.Equal(t, "", doc.firstChild().getTextContent())
		assert.Equal(t, 0, len(doc.firstChild().Children))
		assert.Equal(t, 0, len(doc.firstChild().ChildNodes))
	})

	t.Run("should strip !-based comments within script tags", func(t *testing.T) {
		var html = `<script><!--Silly test > <script src="foo.js"></script>--></script>`
		var doc = newDOMParser().parse(html, "")
		assert.Equal(t, "SCRIPT", doc.firstChild().TagName)
		assert.Equal(t, "", doc.firstChild().getTextContent())
		assert.Equal(t, 0, len(doc.firstChild().Children))
		assert.Equal(t, 0, len(doc.firstChild().ChildNodes))
	})

	t.Run("should strip any other nodes within script tags", func(t *testing.T) {
		var html = `<script>&lt;div>Hello, I'm not really in a &lt;/div></script>`
		var doc = newDOMParser().parse(html, "")
		assert.Equal(t, "SCRIPT", doc.firstChild().TagName)
		assert.Equal(t, `<div>Hello, I'm not really in a </div>`, doc.firstChild().getTextContent())
		assert.Equal(t, 0, len(doc.firstChild().Children))
		assert.Equal(t, 1, len(doc.firstChild().ChildNodes))
	})

	t.Run("should strip any other invalid script nodes within script tags", func(t *testing.T) {
		var html = `<script>&lt;script src="foo.js">&lt;/script></script>`
		var doc = newDOMParser().parse(html, "")
		assert.Equal(t, "SCRIPT", doc.firstChild().TagName)
		assert.Equal(t, "<script src=\"foo.js\"></script>", doc.firstChild().getTextContent())
		assert.Equal(t, 0, len(doc.firstChild().Children))
		assert.Equal(t, 1, len(doc.firstChild().ChildNodes))
	})

	t.Run("should not be confused by partial closing tags", func(t *testing.T) {
		var html = "<script>var x = '&lt;script>Hi&lt;' + '/script>';</script>"
		var doc = newDOMParser().parse(html, "")
		assert.Equal(t, "SCRIPT", doc.firstChild().TagName)
		assert.Equal(t, "var x = '<script>Hi<' + '/script>';", doc.firstChild().getTextContent())
		assert.Equal(t, 0, len(doc.firstChild().Children))
		assert.Equal(t, 1, len(doc.firstChild().ChildNodes))
	})
}

func TestTagName_LocalName_Handling(t *testing.T) {
	t.Run("should lowercase tag names", func(t *testing.T) {
		var html = "<DIV><svG><clippath/></svG></DIV>"
		var doc = newDOMParser().parse(html, "")
		assert.Equal(t, "DIV", doc.firstChild().TagName)
		assert.Equal(t, "div", doc.firstChild().LocalName)
		assert.Equal(t, "SVG", doc.firstChild().firstChild().TagName)
		assert.Equal(t, "svg", doc.firstChild().firstChild().LocalName)
		assert.Equal(t, "CLIPPATH", doc.firstChild().firstChild().firstChild().TagName)
		assert.Equal(t, "clippath", doc.firstChild().firstChild().firstChild().LocalName)
	})
}

func TestRecovery_From_SelfClosing_Tags_That_Have_Close_Tags(t *testing.T) {
	t.Run("should handle delayed closing of a tag", func(t *testing.T) {
		var html = "<div><input><p>I'm in an input</p></input></div>"
		var doc = newDOMParser().parse(html, "")
		assert.Equal(t, "div", doc.firstChild().LocalName)
		assert.Equal(t, 1, len(doc.firstChild().ChildNodes))
		assert.Equal(t, "input", doc.firstChild().firstChild().LocalName)
		assert.Equal(t, 1, len(doc.firstChild().firstChild().ChildNodes))
		assert.Equal(t, "p", doc.firstChild().firstChild().firstChild().LocalName)
	})
}

func TestBaseURI_Parsing(t *testing.T) {

	t.Run("should handle various types of relative and absolute base URIs", func(t *testing.T) {

		var checkBase = func(base, expectedResult string) {
			var html = "<html><head><base href='" + base + "'></base></head><body/></html>"
			var doc = newDOMParser().parse(html, "http://fakehost/some/dir/")
			assert.Equal(t, expectedResult, doc.getBaseURI())
		}

		checkBase("relative/path", "http://fakehost/some/dir/relative/path")
		checkBase("/path", "http://fakehost/path")
		checkBase("http://absolute/", "http://absolute/")
		checkBase("//absolute/path", "http://absolute/path")
	})
}

func TestNamespace_Workarounds(t *testing.T) {

	t.Run("should handle random namespace information in the serialized DOM", func(t *testing.T) {
		var html = "<a0:html><a0:body><a0:DIV><a0:svG><a0:clippath/></a0:svG></a0:DIV></a0:body></a0:html>"
		var doc = newDOMParser().parse(html, "")
		var div = doc.getElementsByTagName("div")[0]

		assert.Equal(t, "DIV", div.TagName)
		assert.Equal(t, "div", div.LocalName)
		assert.Equal(t, "SVG", div.firstChild().TagName)
		assert.Equal(t, "svg", div.firstChild().LocalName)
		assert.Equal(t, "CLIPPATH", div.firstChild().firstChild().TagName)
		assert.Equal(t, "clippath", div.firstChild().firstChild().LocalName)
		assert.Equal(t, doc.firstChild(), doc.DocumentElement)
		assert.Equal(t, doc.DocumentElement.firstChild(), doc.Body)
	})
}
