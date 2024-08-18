package readability

import (
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"
)

func TestMapDoc(t *testing.T) {

	s := `<p>Links:</p><ul><li><a href="foo">Foo</a><li><a href="/bar/baz">BarBaz</a></ul>`
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		log.Fatal(err)
	}

	got := mapDoc(doc, "http://example.com")
	assert.NotNil(t, got)

	html := got.ChildNodes[0]
	assert.NotNil(t, html)

	head := html.ChildNodes[0]
	assert.NotNil(t, head)

	body := html.ChildNodes[1]
	assert.NotNil(t, body)

	p := body.ChildNodes[0]
	assert.NotNil(t, p)

	assert.Equal(t, "Links:", p.getTextContent())

	ul := body.ChildNodes[1]
	assert.NotNil(t, ul)

	firstLi := ul.ChildNodes[0]
	assert.NotNil(t, firstLi)

	firstA := firstLi.ChildNodes[0]
	assert.NotNil(t, firstA)
	assert.Equal(t, "foo", firstA.getHref())
	assert.Equal(t, "Foo", firstA.getTextContent())

	secondLi := ul.ChildNodes[1]
	assert.NotNil(t, secondLi)

	secondA := secondLi.ChildNodes[0]
	assert.NotNil(t, secondA)
	assert.Equal(t, "/bar/baz", secondA.getHref())
	assert.Equal(t, "BarBaz", secondA.getTextContent())

}
