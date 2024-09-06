package readability

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/yosssi/gohtml"
)

type testPage struct {
	dir              string
	source           []byte
	expectedContent  []byte
	expectedMetadata *expectedMetadata
}

type expectedMetadata struct {
	Title         string `json:"title,omitempty"`
	Byline        string `json:"byline,omitempty"`
	Dir           string `json:"dir,omitempty"`
	Lang          string `json:"lang,omitempty"`
	Excerpt       string `json:"excerpt,omitempty"`
	SiteName      string `json:"siteName,omitempty"`
	Readerable    bool   `json:"readerable,omitempty"`
	PublishedTime string `json:"publishedTime,omitempty"`
}

func getTestPages() []*testPage {

	var testPages []*testPage
	err := fs.WalkDir(os.DirFS("testdata/test-pages"), ".", func(p string, d fs.DirEntry, err error) error {

		if err != nil {
			return err
		}

		if p == "." || p == ".." {
			return nil
		}

		fileInfo, err := os.Stat("testdata/test-pages/" + p)
		if err != nil {
			return err
		}

		if fileInfo.IsDir() {
			p = "testdata/test-pages/" + p
			tp := &testPage{
				dir: p,
			}
			source, err := os.ReadFile(path.Join(p, "source.html"))
			if err != nil {
				return err
			}
			tp.source = source
			expected, err := os.ReadFile(path.Join(p, "expected.html"))
			if err != nil {
				return err
			}
			tp.expectedContent = expected
			expectedMetadataRaw, err := os.ReadFile(path.Join(p, "expected-metadata.json"))
			if err != nil {
				return err
			}
			m := &expectedMetadata{}
			err = json.Unmarshal(expectedMetadataRaw, m)
			if err != nil {
				return err
			}
			tp.expectedMetadata = m
			testPages = append(testPages, tp)
		}

		return nil
	})
	if err != nil {
		panic(err)
	}
	return testPages
}

func TestParse(t *testing.T) {

	const uri = "http://fakehost/test/page.html"

	for _, testPage := range getTestPages() {

		t.Run(testPage.dir, func(t *testing.T) {

			reader, err := New(string(testPage.source), uri,
				ClassesToPreserve("caption"),
			)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}
			result, err := reader.Parse()
			if err != nil {
				t.Error(err)
				t.FailNow()
			}

			t.Run("should extract expected content", func(t *testing.T) {

				var actualDOM = domGenerationFn(prettyPrint(result.Content), uri)
				var expectedDOM = domGenerationFn(string(prettyPrint(string(testPage.expectedContent))), uri)
				failed := traverseDOM(func(actualNode, expectedNode *node) bool {

					if actualNode != nil && expectedNode != nil {

						var actualDesc = nodeStr(actualNode)
						var expectedDesc = nodeStr(expectedNode)
						if diff := cmp.Diff(expectedDesc, actualDesc); diff != "" {
							t.Errorf("diff=%s\n", diff)
							return false
						}
						// Compare text for text nodes:
						if actualNode.NodeType == textNode {
							var actualText = htmlTransform(actualNode.getTextContent())
							var expectedText = htmlTransform(expectedNode.getTextContent())
							if diff := cmp.Diff(expectedText, actualText); diff != "" {
								t.Errorf("diff=%s\n", diff)
								return false
							}
							// Compare attributes for element nodes:
						} else if actualNode.NodeType == elementNode {
							var actualNodeDesc = attributesForNode(actualNode)
							var expectedNodeDesc = attributesForNode(expectedNode)
							var desc = "node " + nodeStr(actualNode) + " attributes (" + actualNodeDesc + ") should match (" + expectedNodeDesc + ") "
							if len(actualNode.Attributes) != len(expectedNode.Attributes) {
								t.Errorf("got %d want %d; desc=%s", len(actualNode.Attributes), len(expectedNode.Attributes), desc)
							}
							for i := 0; i < len(actualNode.Attributes); i++ {
								var attr = actualNode.Attributes[i].getName()
								var actualValue = actualNode.getAttribute(attr)
								var expectedValue = expectedNode.getAttribute(attr)
								if diff := cmp.Diff(expectedValue, actualValue); diff != "" {
									t.Errorf("diff=%s\n", diff)
								}
							}
						}
					} else {
						if nodeStr(actualNode) != nodeStr(expectedNode) {
							t.Error("Should have a node from both DOMs")
						}
						return false
					}
					return true
				}, expectedDOM, actualDOM)

				if failed {
					os.WriteFile(testPage.dir+"/failed.html", []byte(result.Content), os.ModePerm)
				}
			})

			t.Run("should extract expected title", func(t *testing.T) {
				assert.Equal(t, testPage.expectedMetadata.Title, result.Title)
			})

			t.Run("should extract expected byline", func(t *testing.T) {
				assert.Equal(t, testPage.expectedMetadata.Byline, result.Byline)
			})

			t.Run("should extract expected excerpt", func(t *testing.T) {
				assert.Equal(t, testPage.expectedMetadata.Excerpt, result.Excerpt)
			})

			t.Run("should extract expected site name", func(t *testing.T) {
				assert.Equal(t, testPage.expectedMetadata.SiteName, result.SiteName)
			})

			t.Run("should extract expected direction", func(t *testing.T) {
				assert.Equal(t, testPage.expectedMetadata.Dir, result.Dir)
			})

			t.Run("should extract expected language", func(t *testing.T) {
				assert.Equal(t, testPage.expectedMetadata.Lang, result.Lang)
			})

			t.Run("should extract expected published time", func(t *testing.T) {
				assert.Equal(t, testPage.expectedMetadata.PublishedTime, result.PublishedTime)
			})

			t.Run("should infer if the article is readerable", func(t *testing.T) {
				assert.Equal(t, testPage.expectedMetadata.Readerable, IsProbablyReaderable(string(testPage.source)))
			})
		})
	}
}

func prettyPrint(html string) string {
	return gohtml.Format(html)
}

func domGenerationFn(source, uri string) *node {
	return newDOMParser().parse(source, uri)
}

func nodeStr(n *node) string {
	if n == nil {
		return "(no node)"
	}
	if n.NodeType == textNode {
		return "#text(" + htmlTransform(n.getTextContent()) + ")"
	}
	if n.NodeType != elementNode {
		return "some other node type: " + strconv.Itoa(int(n.NodeType)) + " with data " + n.getInnerHTML()
	}
	var rv = n.LocalName
	if n.getId() != "" {
		rv += "#" + n.getId()
	}
	if n.getClassName() != "" {
		rv += ".(" + n.getClassName() + ")"
	}
	return rv
}

func attributesForNode(n *node) string {
	var attrs []string
	for _, a := range n.Attributes {
		attrs = append(attrs, a.getName()+"="+a.getValue())
	}
	return strings.Join(attrs, ",")
}

func inOrderTraverse(fromNode *node) *node {
	fc := fromNode.firstChild()
	if fc != nil {
		return fc
	}
	for fromNode != nil && fromNode.NextSibling == nil {
		fromNode = fromNode.ParentNode
	}
	if fromNode != nil {
		return fromNode.NextSibling
	}
	return nil
}

func inOrderIgnoreEmptyTextNodes(fromNode *node) *node {
	fromNode = inOrderTraverse(fromNode)
	for fromNode != nil && fromNode.NodeType == textNode && strings.TrimSpace(fromNode.getTextContent()) == "" {
		fromNode = inOrderTraverse(fromNode)
	}
	return fromNode
}

func traverseDOM(callback func(*node, *node) bool, expectedDOM, actualDOM *node) bool {
	var actualNode = actualDOM.DocumentElement
	if actualNode == nil {
		actualNode = actualDOM.ChildNodes[0]
	}
	var expectedNode = expectedDOM.DocumentElement
	if expectedNode == nil {
		expectedNode = expectedDOM.ChildNodes[0]
	}
	for actualNode != nil || expectedNode != nil {
		// We'll stop if we don't have both actualNode and expectedNode
		if !callback(actualNode, expectedNode) {
			return true
		}
		actualNode = inOrderIgnoreEmptyTextNodes(actualNode)
		expectedNode = inOrderIgnoreEmptyTextNodes(expectedNode)
	}
	return false
}

// Collapse subsequent whitespace like HTML:
func htmlTransform(str string) string {
	return regexp.MustCompile(`\s+`).ReplaceAllString(str, " ")
}
