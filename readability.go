/*
 * Copyright (c) 2010 Arc90 Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/*
 * This code is heavily based on Arc90's readability.js (1.7.1) script
 * available at: http://code.google.com/p/arc90labs-readability
 */

package readability

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

const (
	flagStripUnlikelys     = 0x1
	flagWeightClasses      = 0x2
	flagCleanConditionally = 0x4

	// Max number of nodes supported by this parser. Default: 0 (no limit)
	defaultMaxElemsToParse = 0
	// The number of top candidates to consider when analysing how
	// tight the competition is among candidates.
	defaultNTopCandidates = 5

	// The default number of chars an article must have in order to return a result
	defaultCharThreshold = 500
)

var (
	// Element tags to score by default.
	defaultTagsToScore = []string{"SECTION", "H2", "H3", "H4", "H5", "H6", "P", "TD", "PRE"}

	unlinkelyRoles = []string{"menu", "menubar", "complementary", "navigation", "alert", "alertdialog", "dialog"}

	divToPElemns = []string{"BLOCKQUOTE", "DL", "DIV", "IMG", "OL", "P", "PRE", "TABLE", "UL"}

	alterToDiveExceptions = []string{"DIV", "ARTICLE", "SECTION", "P"}

	presentationalAttribute = []string{"align", "background", "bgcolor", "border", "cellpadding", "cellspacing", "frame", "hspace", "rules", "style", "valign", "vspace"}

	deprecatedSizeAttributeElems = []string{"TABLE", "TH", "TD", "HR", "PRE"}

	// The commented out elements qualify as phrasing content but tend to be
	// removed by readability when put into paragraphs, so we ignore them here.
	phrasingElems = []string{
		// "CANVAS", "IFRAME", "SVG", "VIDEO",
		"ABBR", "AUDIO", "B", "BDO", "BR", "BUTTON", "CITE", "CODE", "DATA",
		"DATALIST", "DFN", "EM", "EMBED", "I", "IMG", "INPUT", "KBD", "LABEL",
		"MARK", "MATH", "METER", "NOSCRIPT", "OBJECT", "OUTPUT", "PROGRESS", "Q",
		"RUBY", "SAMP", "SCRIPT", "SELECT", "SMALL", "SPAN", "STRONG", "SUB",
		"SUP", "TEXTAREA", "TIME", "VAR", "WBR"}

	// These are the classes that readability sets itself.
	classesToPreserve = []string{"page"}
)

type Readability struct {
	options         *Options
	flags           int
	doc             *Node
	articleTitle    string
	articleByline   string
	articleDir      string
	articleSiteName string
	articleLang     string
	attempts        []*attempt
}

type attempt struct {
	articleContent *Node
	textLength     int
}

// New is the public constructor of Readability and it supports the following options:
//   - options.debug
//   - options.maxElemsToParse
//   - options.nbTopCandidates
//   - options.charThreshold
//   - this.classesToPreseve
//   - options.keepClasses
//   - options.serializer
func New(htmlSource, uri string, opts ...Option) (*Readability, error) {

	if htmlSource == "" {
		return nil, fmt.Errorf("first argument to Readability constructor should be a HTML document")
	}

	r := &Readability{
		options: defaultOpts(),
	}

	// Configurable options
	for _, opt := range opts {
		opt(r.options)
	}

	r.doc = newDOMParser().parse(htmlSource, uri)
	if r.doc == nil || r.doc.Body == nil {
		return nil, fmt.Errorf("cannot parse doc")
	}

	// Start with all flags set
	r.flags = flagStripUnlikelys | flagWeightClasses | flagCleanConditionally

	return r, nil
}

type Result struct {
	// article title
	Title string
	// HTML string of processed article HTMLContent
	HTMLContent string
	// text content of the article, with all the HTML tags removed
	TextContent string
	// length of an article, in characters (runes)
	Length int
	// article description, or short excerpt from the content
	Excerpt string
	// author metadata
	Byline string
	// content direction
	Dir string
	// name of the site
	SiteName string
	// content language
	Lang string
	// published time
	PublishedTime string
}

// Run any post-process modifications to article content as necessary.
func (r *Readability) postProcessContent(articleContent *Node) {
	// Readability cannot open relative uris so we convert them to absolute uris.
	r.fixRelativeUris(articleContent)

	r.simplifyNestedElements(articleContent)

	if !r.options.keepClasses {
		// Remove classes.
		r.cleanClasses(articleContent)
	}
}

// Iterates over a NodeList, calls `filterFn` for each node and removes node
// if function returned `true`.
// If function is not passed, removes all the nodes in node list.
func (r *Readability) removeNodes(nodeList []*Node, filterFn func(n *Node) bool) {
	for i := len(nodeList) - 1; i >= 0; i-- {
		node := nodeList[i]
		parentNode := node.ParentNode
		if parentNode != nil {
			if filterFn == nil || filterFn(node) {
				if _, err := parentNode.RemoveChild(node); err != nil {
					slog.Error("cannot remove child", slog.String("err", err.Error()))
				}
			}
		}
	}
}

// Iterates over a NodeList, and calls setNodeTag for each node.
func (r *Readability) replaceNodeTags(nodeList []*Node, newtagName string) {
	for _, node := range nodeList {
		r.setNodeTag(node, newtagName)
	}
}

// Iterate over a NodeList, return true if any of the provided iterate
// function calls returns true, false otherwise.
func (r *Readability) someNode(nodeList []*Node, fn func(n *Node) bool) bool {
	for _, node := range nodeList {
		if fn(node) {
			return true
		}
	}
	return false
}

// Iterate over a NodeList, return true if all of the provided iterate
// function calls return true, false otherwise.
func (r *Readability) everyNode(nodeList []*Node, fn func(n *Node) bool) bool {
	for _, node := range nodeList {
		if !fn(node) {
			return false
		}
	}
	return true
}

// Concat all nodelists passed as arguments.
func (r *Readability) concatNodeLists(nodeLists ...[]*Node) []*Node {
	ret := make([]*Node, 0)
	for _, list := range nodeLists {
		ret = append(ret, list...)
	}
	return ret
}

func (r *Readability) getAllNodesWithTag(n *Node, tagNames ...string) []*Node {
	nodes := make([]*Node, 0)
	for _, tag := range tagNames {
		nodes = append(nodes, n.getElementsByTagName(tag)...)
	}
	return nodes
}

// Removes the class="" attribute from every element in the given
// subtree, except those that match CLASSES_TO_PRESERVE and
// the classesToPreserve array from the options object.
func (r *Readability) cleanClasses(n *Node) {
	className := n.GetAttribute("class")
	if className != "" {
		className = strings.Join(filter(r.preserve, multipleWhitespaces.Split(className, -1)...), " ")
	}

	if className != "" {
		n.SetAttribute("class", className)
	} else {
		n.RemoveAttribute("class")
	}

	for n := n.FirstElementChild(); n != nil; n = n.NextElementSibling {
		r.cleanClasses(n)
	}
}

func (r *Readability) preserve(s string) bool {
	return slices.Contains(r.options.classesToPreserve, s)
}

func filter(filterFn func(string) bool, strs ...string) []string {
	var filtered []string
	for _, str := range strs {
		if filterFn(str) {
			filtered = append(filtered, str)
		}
	}
	return filtered
}

// Converts each <a> and <img> uri in the given element to an absolute URI,
// ignoring #ref URIs.
func (r *Readability) fixRelativeUris(articleContent *Node) {
	baseURI := r.doc.getBaseURI()
	documentURI := r.doc.DocumentURI

	var toAbsoluteURI = func(uri string) string {
		uri = strings.TrimSpace(uri)
		// Leave hash links alone if the base URI matches the document URI:
		if baseURI == documentURI && []rune(uri)[0] == '#' {
			return uri
		}
		base, err := url.Parse(baseURI)
		if err != nil {
			// Something went wrong, just return the original:
			return uri
		}
		ref, err := url.Parse(uri)
		if err != nil {
			// Something went wrong, just return the original:
			return uri
		}
		u := base.ResolveReference(ref)
		var abs string
		if u.Scheme != "" {
			abs += u.Scheme
			if strings.HasPrefix(u.Scheme, "http") {
				abs += "://"
			} else {
				abs += ":"
			}
		}
		abs += strings.ToLower(u.Host)

		var b, a string
		if strings.Contains(uri, "?") {
			before, _, _ := strings.Cut(uri, "?")
			b = before
		} else if strings.Contains(uri, "#") {
			before, after, _ := strings.Cut(uri, "#")
			b = before
			a = after
		} else {
			b = uri
		}

		if u.Path != "" {
			p := u.Path
			if strings.Contains(uri, "%") {
				if strings.HasPrefix(uri, "//") {
					p = doubleForwardSlashes.ReplaceAllString(b, "")
				} else {
					p = strings.ReplaceAll(b, abs, "")
				}
			}
			abs += strings.ReplaceAll(p, "/C|/", "/C:/")
		} else if u.Opaque != "" {
			abs += u.Opaque
		} else {
			abs += "/"
		}
		if u.RawQuery != "" {
			abs += "?" + u.RawQuery
		}
		if u.Fragment != "" {
			if strings.Contains(a, "%") {
				abs += "#" + a
			} else {
				abs += "#" + u.Fragment
			}
		}
		if strings.HasSuffix(uri, "#") && !strings.HasSuffix(abs, "#") {
			abs += "#"
		}
		if strings.HasSuffix(uri, "?") && !strings.HasSuffix(abs, "?") {
			abs += "?"
		}
		return abs
	}

	var links = r.getAllNodesWithTag(articleContent, "a")
	for _, link := range links {
		var href = link.GetAttribute("href")
		if href != "" {
			// Remove links with javascript: URIs, since
			// they won't work after scripts have been removed from the page.
			if strings.HasPrefix(href, "javascript:") {
				// if the link only contains simple text content, it can be converted to a text node
				if len(link.ChildNodes) == 1 && link.ChildNodes[0].NodeType == textNode {
					var text = r.doc.createTextNode(link.GetTextContent())
					link.ParentNode.ReplaceChild(text, link)
				} else {
					// if the link has multiple children, they should all be preserved
					var container = r.doc.createElementNode("span")
					for link.FirstChild() != nil {
						container.AppendChild(link.FirstChild())
					}
					link.ParentNode.ReplaceChild(container, link)
				}
			} else {
				if strings.Contains(href, ",%20") {
					var hrefs []string
					for _, link := range strings.Split(href, ",%20") {
						hrefs = append(hrefs, toAbsoluteURI(link))
					}
					link.SetAttribute("href", strings.Join(hrefs, ",%20"))
				} else {
					link.SetAttribute("href", toAbsoluteURI(href))
				}
			}
		}
	}

	var medias = r.getAllNodesWithTag(articleContent,
		"img", "picture", "figure", "video", "audio", "source",
	)

	for _, media := range medias {
		var src = media.GetAttribute("src")
		if src != "" {
			media.SetAttribute("src", toAbsoluteURI(src))
		}
		var poster = media.GetAttribute("poster")
		if poster != "" {
			media.SetAttribute("poster", toAbsoluteURI(poster))
		}
		var srcset = media.GetAttribute("srcset")
		if srcset != "" {
			submatches := srcsetUrl.FindAllStringSubmatch(srcset, -1)
			var newSrcset []string
			for _, submatch := range submatches {
				newSrcset = append(newSrcset, toAbsoluteURI(submatch[1])+submatch[2]+submatch[3])
			}
			if !strings.Contains(srcset, ", ") {
				media.SetAttribute("srcset", strings.Join(newSrcset, ""))
			} else {
				media.SetAttribute("srcset", strings.Join(newSrcset, " "))
			}
		}
	}
}

func (r *Readability) simplifyNestedElements(articleContent *Node) {
	var node = articleContent
	for node != nil {
		if node.ParentNode != nil && slices.Contains([]string{"DIV", "SECTION"}, node.TagName) && !strings.HasPrefix(node.GetId(), "readability") {
			if r.isElementWithoutContent(node) {
				node = r.removeAndGetNext(node)
				continue
			} else if r.hasSingleTagInsideElement(node, "DIV") || r.hasSingleTagInsideElement(node, "SECTION") {
				var child = node.Children[0]
				for i := 0; i < node.GetAttributeLen(); i++ {
					child.SetAttribute(node.GetAttributeByIndex(i).getName(), node.GetAttributeByIndex(i).getValue())
				}
				node.ParentNode.ReplaceChild(child, node)
				node = child
				continue
			}
		}
		node = r.getNextNode(node, false)
	}
}

// Get the article title as an H1.
func (r *Readability) getArticleTitle() string {
	var doc = r.doc
	var curTitle = strings.TrimSpace(doc.title)
	var origTitle = curTitle

	// If they had an element with id "title" in their HTML
	if curTitle == "" {
		titles := doc.getElementsByTagName("title")
		if len(titles) != 0 {
			curTitle = r.getInnerText(doc.getElementsByTagName("title")[0], true)
			origTitle = curTitle
		}
	}

	var titleHadHierarchicalSeparators bool
	var wordCount func(string) int = func(s string) int {
		return len(multipleWhitespaces.Split(s, -1))
	}

	// If there's a separator in the title, first remove the final part
	if titleFinalPart.MatchString(curTitle) {
		titleHadHierarchicalSeparators = titleSeparators.MatchString(curTitle)
		submatches := otherTitleSeparators.FindAllStringSubmatch(origTitle, -1)
		if len(submatches) != 0 && len(submatches[0]) > 0 {
			curTitle = submatches[0][1]
		}
		// If the resulting title is too short (3 words or fewer), remove
		// the first part instead:
		if wordCount(curTitle) < 3 {
			curTitle = titleFirstPart.ReplaceAllStringFunc(origTitle, func(s string) string {
				return s
			})
		}
	} else if strings.Contains(curTitle, ": ") {
		// Check if we have an heading containing this exact string, so we
		// could assume it's the full title.
		var headings = r.concatNodeLists(
			doc.getElementsByTagName("h1"),
			doc.getElementsByTagName("h2"),
		)
		var trimmedTitle = strings.TrimSpace(curTitle)
		var match = r.someNode(headings, func(heading *Node) bool {
			return strings.TrimSpace(heading.GetTextContent()) == trimmedTitle
		})

		// If we don't, let's extract the title out of the original title string.
		if !match {
			curTitle = origTitle[strings.LastIndex(origTitle, ":")+1:]
		}

		// If the title is now too short, try the first colon instead:
		if wordCount(curTitle) < 3 {
			curTitle = origTitle[strings.Index(origTitle, ":")+1:]
			// But if we have too many words before the colon there's something weird
			// with the titles and the H tags so let's just use the original title instead
		} else if wordCount(origTitle[:strings.Index(origTitle, ":")]) > 5 {
			curTitle = origTitle
		}
	} else if len([]rune(curTitle)) > 150 || len([]rune(curTitle)) < 15 {
		var hOnes = doc.getElementsByTagName("h1")
		if len(hOnes) == 1 {
			curTitle = r.getInnerText(hOnes[0], true)
		}
	}

	curTitle = normalize.ReplaceAllString(strings.TrimSpace(curTitle), " ")
	// If we now have 4 words or fewer as our title, and either no
	// 'hierarchical' separators (\, /, > or ») were found in the original
	// title or we decreased the number of words by more than 1 word, use
	// the original title.
	var curTitleWordCount = wordCount(curTitle)
	if curTitleWordCount <= 4 &&
		(!titleHadHierarchicalSeparators || curTitleWordCount != wordCount(separators.ReplaceAllString(origTitle, ""))) {
		curTitle = origTitle
	}
	return curTitle
}

// Prepare the HTML document for readability to scrape it.
// This includes things like stripping javascript, CSS, and handling terrible markup.
func (r *Readability) prepDocument() {
	var doc = r.doc
	// Remove all style tags in head
	r.removeNodes(r.getAllNodesWithTag(doc, "style"), nil)

	if doc.Body != nil {
		r.replaceBrs(doc.Body)
	}

	r.replaceNodeTags(r.getAllNodesWithTag(doc, "font"), "SPAN")
}

// Finds the next node, starting from the given node, and ignoring
// whitespace in between. If the given node is an element, the same node is
// returned.
func (r *Readability) nextNode(n *Node) *Node {
	var next = n
	for next != nil &&
		next.NodeType != elementNode &&
		whitespace.MatchString(next.GetTextContent()) {
		next = next.NextSibling
	}
	return next
}

// Replaces 2 or more successive <br> elements with a single <p>.
// Whitespace between <br> elements are ignored. For example:
//
//	<div>foo<br>bar<br> <br><br>abc</div>
//
// will become:
//
//	<div>foo<br>bar<p>abc</p></div>
func (r *Readability) replaceBrs(n *Node) {

	for _, br := range r.getAllNodesWithTag(n, "br") {
		var next = br.NextSibling

		// Whether 2 or more <br> elements have been found and replaced with a
		// <p> block.
		var replaced = false

		// If we find a <br> chain, remove the <br>s until we hit another node
		// or non-whitespace. This leaves behind the first <br> in the chain
		// (which will be replaced with a <p> later).
		for next = r.nextNode(next); next != nil && next.TagName == "BR"; {
			replaced = true
			var brSibling = next.NextSibling
			if _, err := next.ParentNode.RemoveChild(next); err != nil {
				slog.Error("cannot remove child", slog.String("err", err.Error()))
			}
			next = brSibling
		}

		// If we removed a <br> chain, replace the remaining <br> with a <p>. Add
		// all sibling nodes as children of the <p> until we hit another <br>
		// chain.
		if replaced {
			var p = r.doc.createElementNode("p")
			br.ParentNode.ReplaceChild(p, br)

			next = p.NextSibling
			for next != nil {
				// If we've hit another <br><br>, we're done adding children to this <p>.
				if next.TagName == "BR" {
					var nextElem = r.nextNode(next.NextSibling)
					if nextElem != nil && nextElem.TagName == "BR" {
						break
					}
				}

				if !r.isPhrasingContent(next) {
					break
				}

				// Otherwise, make this node a child of the new <p>.
				var sibling = next.NextSibling
				p.AppendChild(next)
				next = sibling
			}

			for p.LastChild() != nil && r.isWhitespace(p.LastChild()) {
				if _, err := p.RemoveChild(p.LastChild()); err != nil {
					slog.Error("cannot remove child", slog.String("err", err.Error()))
				}
			}

			if p.ParentNode.TagName == "P" {
				r.setNodeTag(p.ParentNode, "DIV")
			}
		}
	}
}

func (r *Readability) setNodeTag(n *Node, tag string) *Node {
	slog.Debug("setNodeTag", "node", n, "tag", tag)
	n.LocalName = strings.ToLower(tag)
	n.TagName = strings.ToUpper(tag)
	return n
}

// Prepare the article node for display. Clean out any inline styles,
// iframes, forms, strip extraneous <p> tags, etc.
func (r *Readability) prepArticle(articleContent *Node) {
	r.cleanStyles(articleContent)

	// Check for data tables before we continue, to avoid removing items in
	// those tables, which will often be isolated even though they're
	// visually linked to other content-ful elements (text, images, etc.).
	r.markDataTables(articleContent)

	r.fixLazyImages(articleContent)

	// Clean out junk from the article content
	r.cleanConditionally(articleContent, "form")
	r.cleanConditionally(articleContent, "fieldset")
	r.clean(articleContent, "object")
	r.clean(articleContent, "embed")
	r.clean(articleContent, "footer")
	r.clean(articleContent, "link")
	r.clean(articleContent, "aside")

	// Clean out elements with little content that have "share" in their id/class combinations from final top candidates,
	// which means we don't remove the top candidates even they have "share".
	var shareElementThreshold = defaultCharThreshold
	for _, topCandidate := range articleContent.Children {
		r.cleanMatchedNodes(topCandidate, func(n *Node, matchString string) bool {
			return shareElements.MatchString(matchString) &&
				len([]rune(n.GetTextContent())) < shareElementThreshold
		})
	}

	r.clean(articleContent, "iframe")
	r.clean(articleContent, "input")
	r.clean(articleContent, "textarea")
	r.clean(articleContent, "select")
	r.clean(articleContent, "button")
	r.cleanHeaders(articleContent)

	// Do these last as the previous stuff may have removed junk
	// that will affect these
	r.cleanConditionally(articleContent, "table")
	r.cleanConditionally(articleContent, "ul")
	r.cleanConditionally(articleContent, "div")

	// replace H1 with H2 as H1 should be only title that is displayed separately
	r.replaceNodeTags(r.getAllNodesWithTag(articleContent, "h1"), "h2")

	// Remove extra paragraphs
	r.removeNodes(r.getAllNodesWithTag(articleContent, "p"), func(paragraph *Node) bool {
		var imgCount = len(paragraph.getElementsByTagName("img"))
		var embedCount = len(paragraph.getElementsByTagName("embed"))
		var objectCount = len(paragraph.getElementsByTagName("object"))
		// At this point, nasty iframes have been removed, only remain embedded video ones.
		var iframeCount = len(paragraph.getElementsByTagName("iframe"))
		var totalCount = imgCount + embedCount + objectCount + iframeCount
		return totalCount == 0 && r.getInnerText(paragraph, false) == ""
	})

	for _, br := range r.getAllNodesWithTag(articleContent, "br") {
		var next = r.nextNode(br.NextSibling)
		if next != nil && next.TagName == "P" {
			if _, err := br.ParentNode.RemoveChild(br); err != nil {
				slog.Error("cannot remove child", slog.String("err", err.Error()))
			}
		}
	}

	// Remove single-cell tables
	for _, table := range r.getAllNodesWithTag(articleContent, "table") {
		var tbody *Node = table
		if r.hasSingleTagInsideElement(table, "TBODY") {
			tbody = table.FirstElementChild()
		}
		if r.hasSingleTagInsideElement(tbody, "TR") {
			var row = tbody.FirstElementChild()
			if r.hasSingleTagInsideElement(row, "TD") {
				var cell = row.FirstElementChild()
				var tag = "DIV"
				if r.everyNode(cell.ChildNodes, r.isPhrasingContent) {
					tag = "P"
				}
				cell = r.setNodeTag(cell, tag)
				table.ParentNode.ReplaceChild(cell, table)
			}
		}
	}
}

// Initialize a node with the readability object. Also checks the
// className/id for special names to add to its score.
func (r *Readability) initializeNode(n *Node) {

	n.ReadabilityNode = &readabilityNode{
		ContentScore: 0,
	}

	switch n.TagName {
	case "DIV":
		n.ReadabilityNode.ContentScore += 5

	case "PRE", "TD", "BLOCKQUOTE":
		n.ReadabilityNode.ContentScore += 3

	case "ADDRESS", "OL", "UL", "DL", "DD", "DT", "LI", "FORM":
		n.ReadabilityNode.ContentScore -= 3

	case "H1", "H2", "H3", "H4", "H5", "H6", "TH":
		n.ReadabilityNode.ContentScore -= 5
	}

	n.ReadabilityNode.ContentScore += r.getClassWeight(n)
}

func (r *Readability) removeAndGetNext(n *Node) *Node {
	var nextNode = r.getNextNode(n, true)
	if _, err := n.ParentNode.RemoveChild(n); err != nil {
		slog.Error("cannot remove child", slog.String("err", err.Error()))
	}
	return nextNode
}

// Traverse the DOM from node to node, starting at the node passed in.
// Pass true for the second parameter to indicate this node itself
// (and its kids) are going away, and we want the next node over.
// Calling this in a loop will traverse the DOM depth-first.
func (r *Readability) getNextNode(n *Node, ignoreSelfAndKids bool) *Node {
	// First check for kids if those aren't being ignored
	if !ignoreSelfAndKids && n.FirstElementChild() != nil {
		return n.FirstElementChild()
	}
	// Then for siblings...
	if n.NextElementSibling != nil {
		return n.NextElementSibling
	}
	// And finally, move up the parent chain *and* find a sibling
	// (because this is depth-first traversal, we will have already
	// seen the parent nodes themselves).
	n = n.ParentNode
	for n != nil && n.NextElementSibling == nil {
		n = n.ParentNode
	}
	if n != nil {
		return n.NextElementSibling
	}
	return n
}

// Compares second text to first one
// 1 = same text, 0 = completely different text.
// Works the way that it splits both texts into words and then finds words that are unique in second text
// the result is given by the lower length of unique parts.
func (r *Readability) textSimilarity(textA, textB string) float64 {
	var tokensA = tokenize.Split(strings.ToLower(textA), -1)
	var tokensB = tokenize.Split(strings.ToLower(textB), -1)
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0
	}
	var uniqTokensB []string
	for _, t := range tokensB {
		if !slices.Contains(tokensA, t) && t != "" {
			uniqTokensB = append(uniqTokensB, t)
		}
	}
	var distanceB = float64(len(strings.Join(uniqTokensB, " "))) / float64(len(strings.Join(tokensB, " ")))
	return 1 - distanceB
}

func (r *Readability) checkByline(n *Node, matchString string) bool {
	if r.articleByline != "" {
		return false
	}

	var rel = n.GetAttribute("rel")
	var itemprop = n.GetAttribute("itemprop")

	if (rel == "author" || strings.Contains(itemprop, "author") || byline.MatchString(matchString)) && r.isValidByline(n.GetTextContent()) {
		r.articleByline = strings.TrimSpace(n.GetTextContent())
		return true
	}

	return false
}

func (r *Readability) getNodeAncestors(n *Node, maxDepth int) []*Node {
	var i, ancestors = 0, []*Node{}
	for n.ParentNode != nil {
		ancestors = append(ancestors, n.ParentNode)
		if i++; i == maxDepth {
			break
		}
		n = n.ParentNode
	}
	return ancestors
}

// Using a variety of metrics (content score, classname, element types), find the content that is
// most likely to be the stuff a user wants to read. Then return it wrapped up in a div.
func (r *Readability) grabArticle(page *Node) *Node {

	slog.Debug("**** grabArticle ****")
	var doc = r.doc

	var isPaging bool
	if page != nil {
		isPaging = true
	}
	if page == nil {
		page = r.doc.Body
	}

	// We can't grab an article if we don't have a page!
	if page == nil {
		slog.Debug("No body found in document. Abort.")
		return nil
	}

	var pageCacheHtml = page.GetInnerHTML()

	for {
		slog.Debug("Starting grabArticle loop")
		var stripUnlikelyCandidates = r.flagIsActive(flagStripUnlikelys)

		// First, node prepping. Trash nodes that look cruddy (like ones with the
		// class name "comment", etc), and turn divs into P tags where they have been
		// used inappropriately (as in, where they contain no other block level elements.)
		var elementsToScore []*Node
		var n = r.doc.DocumentElement

		var shouldRemoveTitleHeader bool = true

		for n != nil {

			slog.Debug("elementsToScore", "nodeText", n.GetTextContent())

			if n.TagName == "HTML" {
				r.articleLang = n.GetAttribute("lang")
			}

			var matchString = n.GetClassName() + " " + n.GetId()

			if !isProbablyVisible(n) {
				slog.Debug("Removing hidden node - " + matchString)
				n = r.removeAndGetNext(n)
				continue
			}

			// User is not able to see elements applied with both "aria-modal = true" and "role = dialog"
			if n.GetAttribute("aria-modal") == "true" && n.GetAttribute("role") == "dialog" {
				n = r.removeAndGetNext(n)
				continue
			}

			// Check to see if this node is a byline, and remove it if it is.
			if r.checkByline(n, matchString) {
				n = r.removeAndGetNext(n)
				continue
			}

			if shouldRemoveTitleHeader && r.headerDuplicatesTitle(n) {
				slog.Debug("Removing header:", "textContent", strings.TrimSpace(n.GetTextContent()), "articleTitle", strings.TrimSpace(r.articleTitle))
				shouldRemoveTitleHeader = false
				n = r.removeAndGetNext(n)
				continue
			}

			// Remove unlikely candidates
			if stripUnlikelyCandidates {
				if unlikelyCandidates.MatchString(matchString) &&
					!okMaybeItsACandidate.MatchString(matchString) &&
					!r.hasAncestorTag(n, "table", 3, nil) &&
					!r.hasAncestorTag(n, "code", 3, nil) &&
					n.TagName != "BODY" &&
					n.TagName != "A" {
					slog.Debug("Removing unlikely candidate", "matchString", matchString)
					n = r.removeAndGetNext(n)
					continue
				}
			}

			if slices.Contains(unlinkelyRoles, n.GetAttribute("role")) {
				slog.Debug("Removing content", "role", n.GetAttribute("role"), "matchString", matchString)
				n = r.removeAndGetNext(n)
				continue
			}

			// Remove DIV, SECTION, and HEADER nodes without any content(e.g. text, image, video, or iframe).
			if (n.TagName == "DIV" || n.TagName == "SECTION" || n.TagName == "HEADER" ||
				n.TagName == "H1" || n.TagName == "H2" || n.TagName == "H3" ||
				n.TagName == "H4" || n.TagName == "H5" || n.TagName == "H6") &&
				r.isElementWithoutContent(n) {
				n = r.removeAndGetNext(n)
				continue
			}

			if slices.Contains(defaultTagsToScore, n.TagName) {
				elementsToScore = append(elementsToScore, n)
			}

			// Turn all divs that don't have children block level elements into p's
			if n.TagName == "DIV" {
				// Put phrasing content into paragraphs.
				var p *Node
				var childNode = n.FirstChild()
				for childNode != nil {
					var nextSibling = childNode.NextSibling
					if r.isPhrasingContent(childNode) {
						if p != nil {
							p.AppendChild(childNode)
						} else if !r.isWhitespace(childNode) {
							p = doc.createElementNode("p")
							n.ReplaceChild(p, childNode)
							p.AppendChild(childNode)
						}
					} else if p != nil {
						for p.LastChild() != nil && r.isWhitespace(p.LastChild()) {
							if _, err := p.RemoveChild(p.LastChild()); err != nil {
								slog.Error("cannot remove child", slog.String("err", err.Error()))
							}
						}
						p = nil
					}
					childNode = nextSibling
				}

				// Sites like http://mobile.slate.com encloses each paragraph with a DIV
				// element. DIVs with only a P element inside and no text content can be
				// safely converted into plain P elements to avoid confusing the scoring
				// algorithm with DIVs with are, in practice, paragraphs.
				if r.hasSingleTagInsideElement(n, "P") && r.getLinkDensity(n) < 0.25 {
					var newNode = n.Children[0]
					n.ParentNode.ReplaceChild(newNode, n)
					n = newNode
					elementsToScore = append(elementsToScore, n)
				} else if !r.hasChildBlockElement(n) {
					n = r.setNodeTag(n, "P")
					elementsToScore = append(elementsToScore, n)
				}
			}
			n = r.getNextNode(n, false)
		}

		// Loop through all paragraphs, and assign a score to them based on how content-y they look.
		// Then add their score to their parent node.
		// A score is determined by things like number of commas, class names, etc. Maybe eventually link density.

		var candidates []*Node
		for _, elementToScore := range elementsToScore {
			if elementToScore.ParentNode == nil {
				continue
			}

			// If this paragraph is less than 25 characters, don't even count it.
			var innerText = r.getInnerText(elementToScore, true)
			if len([]rune(innerText)) < 25 {
				continue
			}

			// Exclude nodes with no ancestor.
			var ancestors = r.getNodeAncestors(elementToScore, 5)
			if len(ancestors) == 0 {
				continue
			}

			var contentScore float64 = 0

			// Add a point for the paragraph itself as a base.
			contentScore += 1

			// Add points for any commas within this paragraph.
			contentScore += float64(len(commas.Split(innerText, -1)))

			// For every 100 characters in this paragraph, add another point. Up to 3 points.
			contentScore += math.Min(math.Floor(float64(len([]rune(innerText)))/100), 3)

			for level, ancestor := range ancestors {
				if ancestor.TagName == "" || ancestor.ParentNode == nil || ancestor.ParentNode.TagName == "" {
					continue
				}

				if ancestor.ReadabilityNode == nil {
					r.initializeNode(ancestor)
					candidates = append(candidates, ancestor)
				}

				// Node score divider:
				// - parent:             1 (no division)
				// - grandparent:        2
				// - great grandparent+: ancestor level * 3
				var scoreDivider int
				if level == 0 {
					scoreDivider = 1
				} else if level == 1 {
					scoreDivider = 2
				} else {
					scoreDivider = level * 3
				}
				ancestor.ReadabilityNode.ContentScore += contentScore / float64(scoreDivider)
				slog.Debug("assigned score", "ancestor", ancestor.GetTextContent(), "score", ancestor.ReadabilityNode.ContentScore)
			}
		}

		// After we've calculated scores, loop through all of the possible
		// candidate nodes we found and find the one with the highest score.
		var topCandidates []*Node
		for c := 0; c < len(candidates); c++ {
			var candidate = candidates[c]

			// Scale the final candidates score based on link density. Good content
			// should have a relatively small link density (5% or less) and be mostly
			// unaffected by this operation.
			var candidateScore = candidate.ReadabilityNode.ContentScore * (1 - r.getLinkDensity(candidate))
			candidate.ReadabilityNode.ContentScore = candidateScore

			slog.Debug("grabArticle", "candidate", candidate.GetTextContent(), "scaled-score", candidateScore)

			for t := 0; t < r.options.nbTopCandidates; t++ {
				var aTopCandidate *Node
				if len(topCandidates) > t {
					aTopCandidate = topCandidates[t]
				}

				if aTopCandidate == nil || candidateScore > aTopCandidate.ReadabilityNode.ContentScore {
					topCandidates = insert(candidate, t, topCandidates)
					if len(topCandidates) > r.options.nbTopCandidates {
						topCandidates[len(topCandidates)-1] = nil
						topCandidates = topCandidates[:len(topCandidates)-1]
					}
					break
				}
			}
		}

		var topCandidate *Node
		if len(topCandidates) > 0 {
			topCandidate = topCandidates[0]
		}
		var neededToCreateTopCandidate bool
		var parentOfTopCandidate *Node

		// If we still have no top candidate, just use the body as a last resort.
		// We also have to copy the body node so it is something we can modify.
		if topCandidate == nil || topCandidate.TagName == "BODY" {
			// Move all of the page's children into topCandidate
			topCandidate = doc.createElementNode("DIV")
			neededToCreateTopCandidate = true
			// Move everything (not just elements, also text nodes etc.) into the container
			// so we even include text directly in the body:
			for page.FirstChild() != nil {
				slog.Debug("Moving out:", "child", page.FirstChild().nodeName)
				topCandidate.AppendChild(page.FirstChild())
			}

			page.AppendChild(topCandidate)

			r.initializeNode(topCandidate)
		} else {
			// Find a better top candidate node if it contains (at least three) nodes which belong to `topCandidates` array
			// and whose scores are quite closed with current `topCandidate` node.
			var alternativeCandidateAncestors [][]*Node
			for i := 1; i < len(topCandidates); i++ {
				if topCandidates[i].ReadabilityNode.ContentScore/topCandidate.ReadabilityNode.ContentScore >= 0.75 {
					alternativeCandidateAncestors = append(alternativeCandidateAncestors, r.getNodeAncestors(topCandidates[i], 0))
				}
			}
			var MINIMUM_TOPCANDIDATES = 3
			if len(alternativeCandidateAncestors) >= MINIMUM_TOPCANDIDATES {
				parentOfTopCandidate = topCandidate.ParentNode
				for parentOfTopCandidate.TagName != "BODY" {
					var listsContainingThisAncestor = 0
					for ancestorIndex := 0; ancestorIndex < len(alternativeCandidateAncestors) && listsContainingThisAncestor < MINIMUM_TOPCANDIDATES; ancestorIndex++ {
						includes := slices.ContainsFunc(alternativeCandidateAncestors[ancestorIndex], func(n *Node) bool {
							return n == parentOfTopCandidate
						})
						if includes {
							listsContainingThisAncestor += 1
						}
					}
					if listsContainingThisAncestor >= MINIMUM_TOPCANDIDATES {
						topCandidate = parentOfTopCandidate
						break
					}
					parentOfTopCandidate = parentOfTopCandidate.ParentNode
				}
			}
			if topCandidate.ReadabilityNode == nil {
				r.initializeNode(topCandidate)
			}

			// Because of our bonus system, parents of candidates might have scores
			// themselves. They get half of the node. There won't be nodes with higher
			// scores than our topCandidate, but if we see the score going *up* in the first
			// few steps up the tree, that's a decent sign that there might be more content
			// lurking in other places that we want to unify in. The sibling stuff
			// below does some of that - but only if we've looked high enough up the DOM
			// tree.
			parentOfTopCandidate = topCandidate.ParentNode
			var lastScore = topCandidate.ReadabilityNode.ContentScore
			// The scores shouldn't get too low.
			var scoreThreshold = lastScore / 3
			for parentOfTopCandidate.TagName != "BODY" {
				if parentOfTopCandidate.ReadabilityNode == nil {
					parentOfTopCandidate = parentOfTopCandidate.ParentNode
					continue
				}

				var parentScore = parentOfTopCandidate.ReadabilityNode.ContentScore
				if parentScore < scoreThreshold {
					break
				}
				if parentScore > lastScore {
					// Alright! We found a better parent to use.
					topCandidate = parentOfTopCandidate
					break
				}
				lastScore = parentOfTopCandidate.ReadabilityNode.ContentScore
				parentOfTopCandidate = parentOfTopCandidate.ParentNode
			}

			// If the top candidate is the only child, use parent instead. This will help sibling
			// joining logic when adjacent content is actually located in parent's sibling node.
			parentOfTopCandidate = topCandidate.ParentNode
			for parentOfTopCandidate.TagName != "BODY" && len(parentOfTopCandidate.Children) == 1 {
				topCandidate = parentOfTopCandidate
				parentOfTopCandidate = topCandidate.ParentNode
			}
			if topCandidate.ReadabilityNode == nil {
				r.initializeNode(topCandidate)
			}
		}

		// Now that we have the top candidate, look through its siblings for content
		// that might also be related. Things like preambles, content split by ads
		// that we removed, etc.
		var articleContent = doc.createElementNode("DIV")
		if isPaging {
			articleContent.SetId("readability-content")
		}
		var siblingScoreThreshold = math.Max(10, topCandidate.ReadabilityNode.ContentScore*0.2)
		// Keep potential top candidate's parent node to try to get text direction of it later.
		parentOfTopCandidate = topCandidate.ParentNode
		var siblings = parentOfTopCandidate.Children
		var sl = len(siblings)
		for s := 0; s < sl; s++ {
			var sibling = siblings[s]
			var append = false

			slog.Debug("Looking at sibling node:", "sibling", sibling.GetTextContent(), "score", sibling.ReadabilityNode)

			if sibling == topCandidate {
				append = true
			} else {
				var contentBonus = 0.0
				// Give a bonus if sibling nodes and top candidates have the example same classname
				if sibling.GetClassName() == topCandidate.GetClassName() && topCandidate.GetClassName() != "" {
					contentBonus += topCandidate.ReadabilityNode.ContentScore * 0.2
				}

				if sibling.ReadabilityNode != nil &&
					(sibling.ReadabilityNode.ContentScore+contentBonus) >= siblingScoreThreshold {
					append = true
				} else if sibling.GetNodeName() == "P" {
					var linkDensity = r.getLinkDensity(sibling)
					var nodeContent = r.getInnerText(sibling, true)
					var nodeLength = len([]rune(nodeContent))

					if nodeLength > 80 && linkDensity < 0.25 {
						append = true
					} else if nodeLength < 80 && linkDensity == 0 && dotSpaceOrDollar.FindAllString(nodeContent, -1) != nil {
						append = true
					}
				}
			}

			if append {
				slog.Debug("appending", "node", sibling.GetTextContent())
				if !slices.Contains(alterToDiveExceptions, sibling.GetNodeName()) {
					// We have a node that isn't a common block level element, like a form or td tag.
					// Turn it into a div so it doesn't get filtered out later by accident.
					slog.Debug("altering", "node", sibling.GetTextContent())

					sibling = r.setNodeTag(sibling, "DIV")
				}

				articleContent.AppendChild(sibling)
				// Fetch children again to make it compatible
				// with DOM parsers without live collection support.
				siblings = parentOfTopCandidate.Children
				// siblings is a reference to the children array, and
				// sibling is removed from the array when we call appendChild().
				// As a result, we must revisit this index since the nodes
				// have been shifted.
				s -= 1
				sl -= 1
			}
		}

		slog.Debug("Article content pre-prep", "innerHTML", articleContent.GetInnerHTML())
		// So we have all of the content that we need. Now we clean it up for presentation.
		r.prepArticle(articleContent)
		slog.Debug("Article content post-prep", "innerHTML", articleContent.GetInnerHTML())

		if neededToCreateTopCandidate {
			// We already created a fake div thing, and there wouldn't have been any siblings left
			// for the previous loop, so there's no point trying to create a new div, and then
			// move all the children over. Just assign IDs and class names here. No need to append
			// because that already happened anyway.
			topCandidate.SetId("readability-page-1")
			topCandidate.SetClassName("page")
		} else {
			var div = doc.createElementNode("DIV")
			div.SetId("readability-page-1")
			div.SetClassName("page")
			for articleContent.FirstChild() != nil {
				div.AppendChild(articleContent.FirstChild())
			}
			articleContent.AppendChild(div)
		}

		slog.Debug("Article content after paging", "innerHTML", articleContent.GetInnerHTML())

		var parseSuccessful = true

		// Now that we've gone through the full algorithm, check to see if
		// we got any meaningful content. If we didn't, we may need to re-run
		// grabArticle with different flags set. This gives us a higher likelihood of
		// finding the content, and the sieve approach gives us a higher likelihood of
		// finding the -right- content.
		var textLength = len(r.getInnerText(articleContent, true))
		if textLength < r.options.charThreshold {
			parseSuccessful = false
			page.SetInnerHTML(pageCacheHtml)

			if r.flagIsActive(flagStripUnlikelys) {
				r.removeFlag(flagStripUnlikelys)
				r.attempts = append(r.attempts, &attempt{articleContent: articleContent, textLength: textLength})
			} else if r.flagIsActive(flagWeightClasses) {
				r.removeFlag(flagWeightClasses)
				r.attempts = append(r.attempts, &attempt{articleContent: articleContent, textLength: textLength})
			} else if r.flagIsActive(flagCleanConditionally) {
				r.removeFlag(flagCleanConditionally)
				r.attempts = append(r.attempts, &attempt{articleContent: articleContent, textLength: textLength})
			} else {
				r.attempts = append(r.attempts, &attempt{articleContent: articleContent, textLength: textLength})
				// No luck after removing flags, just return the longest text we found during the different loops
				slices.SortFunc(r.attempts, func(a, b *attempt) int {
					return b.textLength - a.textLength
				})

				if r.attempts[0].textLength == 0 {
					return nil
				}
				articleContent = r.attempts[0].articleContent
				parseSuccessful = true
			}
		}

		if parseSuccessful {
			// Find out text direction from ancestors of final top candidate.
			var ancestors = []*Node{parentOfTopCandidate, topCandidate}
			ancestors = append(ancestors, r.getNodeAncestors(parentOfTopCandidate, 0)...)
			r.someNode(ancestors, func(ancestor *Node) bool {
				if ancestor.TagName == "" {
					return false
				}
				var articleDir = ancestor.GetAttribute("dir")
				if articleDir != "" {
					r.articleDir = articleDir
					return true
				}
				return false
			})
			return articleContent
		}
	}
}

// Check whether the input string could be a byline.
// This verifies that the input is a string, and that the length
// is less than 100 chars.
func (r *Readability) isValidByline(possibleByline string) bool {
	bylineLen := len([]rune(strings.TrimSpace(possibleByline)))
	return bylineLen > 0 && bylineLen < 100
}

// Converts some of the common HTML entities in string to their corresponding characters.
func (r *Readability) unescapeHtmlEntities(str string) string {
	if str == "" {
		return str
	}
	decoded, err := decodeHTML(str)
	if err != nil {
		slog.Error(err.Error())
	}
	return decoded
}

type metadata struct {
	title         string
	byline        string
	excerpt       string
	siteName      string
	datePublished string
	publishedTime string
}

// Try to extract metadata from JSON-LD object.
// For now, only Schema.org objects of type Article or its subtypes are supported.
func (r *Readability) getJSONLD(doc *Node) *metadata {

	var scripts = r.getAllNodesWithTag(doc, "script")

	var meta *metadata

	for _, jsonLdElement := range scripts {
		if meta == nil && jsonLdElement.GetAttribute("type") == "application/ld+json" {
			// Strip CDATA markers if present
			var content = cdata.ReplaceAllString(jsonLdElement.GetTextContent(), "")
			var parsed map[string]interface{}
			if err := json.Unmarshal([]byte(content), &parsed); err != nil {
				slog.Error("cannot unmarshal JSON-LD element content", "err", err)
				continue
			}

			ctx, ctxFound := parsed["@context"]
			if !ctxFound || !schemaUrl.MatchString(ctx.(string)) {
				continue
			}

			_, typeFound := parsed["@type"]
			graph, graphFound := parsed["@graph"]
			if !typeFound || graphFound {
				graphArr, isArray := graph.([]interface{})
				if isArray {
					for _, el := range graphArr {
						elMap, ok := el.(map[string]interface{})
						if ok {
							elType, found := elMap["@type"]
							if found {
								typeStr, ok := elType.(string)
								if ok && jsonLdArticleTypes.MatchString(typeStr) {
									parsed = elMap
									break
								}
							}
						}
					}
				}
			}

			if parsed == nil || parsed["@type"] == nil || !jsonLdArticleTypes.MatchString(parsed["@type"].(string)) {
				continue
			}

			meta = &metadata{}
			name, nameFound := parsed["name"]
			headline, headlineFound := parsed["headline"]

			if nameFound && headlineFound && reflect.ValueOf(name).Kind() == reflect.String && reflect.ValueOf(headline).Kind() == reflect.String && name.(string) != headline.(string) {
				// we have both name and headline element in the JSON-LD. They should both be the same but some websites like aktualne.cz
				// put their own name into "name" and the article title to "headline" which confuses Readability. So we try to check if either
				// "name" or "headline" closely matches the html title, and if so, use that one. If not, then we use "name" by default.

				var title = r.getArticleTitle()
				var nameMatches = r.textSimilarity(name.(string), title) > 0.75
				var headlineMatchs = r.textSimilarity(headline.(string), title) > 0.75

				if headlineMatchs && !nameMatches {
					meta.title = headline.(string)
				} else {
					meta.title = name.(string)
				}
			} else if _, ok := name.(string); ok {
				meta.title = strings.TrimSpace(name.(string))
			} else if _, ok := headline.(string); ok {
				meta.title = strings.TrimSpace(headline.(string))
			}

			author, authorFound := parsed["author"]
			if authorFound {
				if _, isObj := author.(map[string]interface{}); isObj {
					auth := author.(map[string]interface{})
					authorName, authorNameFound := auth["name"]
					if authorNameFound {
						if _, isStr := authorName.(string); isStr {
							meta.byline = strings.TrimSpace(auth["name"].(string))
						}
					}
				} else if _, isArray := author.([]interface{}); isArray {
					authors := author.([]interface{})
					if len(authors) != 0 {
						firstAuthor, isObj := authors[0].(map[string]interface{})
						if isObj {
							authorName, authorNameFound := firstAuthor["name"]
							if authorNameFound {
								if _, isStr := authorName.(string); isStr {
									var authorNames []string
									for _, a := range authors {
										n := a.(map[string]interface{})["name"].(string)
										authorNames = append(authorNames, strings.TrimSpace(n))
									}
									meta.byline = strings.Join(authorNames, ", ")
								}
							}
						}
					}
				}
			}

			descr, ok := parsed["description"]
			if ok {
				if _, ok := descr.(string); ok {
					meta.excerpt = strings.TrimSpace(descr.(string))
				}
			}
			publisher, ok := parsed["publisher"]
			if ok {
				if _, isObj := publisher.(map[string]interface{}); isObj {
					publisherName, publisherNameFound := publisher.(map[string]interface{})["name"]
					if publisherNameFound {
						if _, isStr := publisherName.(string); isStr {
							meta.siteName = strings.TrimSpace(publisherName.(string))
						}
					}
				}
			}
			datePublished, ok := parsed["datePublished"]
			if ok {
				if _, ok := datePublished.(string); ok {
					meta.datePublished = strings.TrimSpace(datePublished.(string))
				}
			}
			continue
		}
	}
	return meta
}

// Attempts to get excerpt and byline metadata for the article.
// Accepts as param 'jsonld' an object containing any metadata that
// could be extracted from a JSON-LD object.
// Returns an object with optional "excerpt" and "byline" properties.
func (r *Readability) getArticleMetadata(jsonld *metadata) *metadata {

	var meta, values = &metadata{}, make(map[string]string, 0)
	var metaElements = r.doc.getElementsByTagName("meta")

	for _, element := range metaElements {
		var elementName = element.GetAttribute("name")
		var elementProperty = element.GetAttribute("property")
		var content = element.GetAttribute("content")
		if content == "" {
			continue
		}

		var matches []string
		var name string

		if elementProperty != "" {
			matches = propertyPattern.FindAllString(elementProperty, -1)
			if len(matches) != 0 {
				// Convert to lowercase, and remove any whitespace
				// so we can match below.
				name = singleWhitespace.ReplaceAllString(strings.ToLower(matches[0]), "")
				// multiple authors
				values[name] = strings.TrimSpace(content)
			}
		}

		if len(matches) == 0 && elementName != "" && namePattern.MatchString(elementName) {
			name = elementName
			if content != "" {
				// Convert to lowercase, remove any whitespace, and convert dots
				// to colons so we can match below.
				name = singleWhitespace.ReplaceAllString(strings.ToLower(name), "")
				name = singleDot.ReplaceAllString(name, ":")
				values[name] = strings.TrimSpace(content)

			}
		}
	}

	if jsonld == nil {
		jsonld = &metadata{}
	}

	// get title
	meta.title = anyOf(jsonld.title,
		values["dc:title"],
		values["dcterm:title"],
		values["og:title"],
		values["weibo:article:title"],
		values["weibo:webpage:title"],
		values["title"],
		values["twitter:title"])

	if meta.title == "" {
		meta.title = r.getArticleTitle()
	}

	// get author
	meta.byline = anyOf(jsonld.byline,
		values["dc:creator"],
		values["dcterm:creator"],
		values["author"])

	// get description
	meta.excerpt = anyOf(jsonld.excerpt,
		values["dc:description"],
		values["dcterm:description"],
		values["og:description"],
		values["weibo:article:description"],
		values["weibo:webpage:description"],
		values["description"],
		values["twitter:description"])

	// get site name
	meta.siteName = anyOf(jsonld.siteName,
		values["og:site_name"])

	// get article published time
	meta.publishedTime = anyOf(jsonld.datePublished,
		values["article:published_time"])

	// in many sites the meta value is escaped with HTML entities,
	// so here we need to unescape it
	meta.title = r.unescapeHtmlEntities(meta.title)
	meta.byline = r.unescapeHtmlEntities(meta.byline)
	meta.excerpt = r.unescapeHtmlEntities(meta.excerpt)
	meta.siteName = r.unescapeHtmlEntities(meta.siteName)
	meta.publishedTime = r.unescapeHtmlEntities(meta.publishedTime)

	return meta
}

// Check if node is image, or if node contains exactly only one image
// whether as a direct child or as its descendants.
func (r *Readability) isSingleImage(n *Node) bool {
	if n.TagName == "IMG" {
		return true
	}

	if len(n.Children) != 1 || strings.TrimSpace(n.GetTextContent()) != "" {
		return false
	}
	return r.isSingleImage(n.Children[0])
}

// Find all <noscript> that are located after <img> nodes, and which contain only one
// <img> element. Replace the first image with the image from inside the <noscript> tag,
// and remove the <noscript> tag. This improves the quality of the images we use on
// some sites (e.g. Medium).
func (r *Readability) unwrapNoscriptImages(doc *Node) {
	// Find img without source or attributes that might contains image, and remove it.
	// This is done to prevent a placeholder img is replaced by img from noscript in next step.
	for _, img := range doc.getElementsByTagName("img") {
		containsImg := slices.ContainsFunc(img.Attributes, func(attr *attribute) bool {
			anyImgAttr := slices.Contains([]string{"src", "srcset", "data-src", "data-srcset"}, attr.name)
			if anyImgAttr {
				return true
			}
			if imgExtensions.MatchString(attr.value) {
				return true
			}
			return false
		})

		if !containsImg {
			if _, err := img.ParentNode.RemoveChild(img); err != nil {
				slog.Error("cannot remove child", slog.String("err", err.Error()))
			}
		}
	}

	// Next find noscript and try to extract its image
	for _, noscript := range doc.getElementsByTagName("noscript") {
		// Parse content of noscript and make sure it only contains image
		var div = doc.createElementNode("div")
		div.SetInnerHTML(noscript.GetInnerHTML())
		if !r.isSingleImage(div) {
			continue
		}

		// If noscript has previous sibling and it only contains image,
		// replace it with noscript content. However we also keep old
		// attributes that might contains image.
		var prevElement = noscript.PreviousElementSibling
		if prevElement != nil && r.isSingleImage(prevElement) {
			var prevImg = prevElement
			if prevImg.TagName != "IMG" {
				prevImg = prevElement.getElementsByTagName("img")[0]
			}

			var newImg = div.getElementsByTagName("img")[0]
			for i := 0; i < len(prevImg.Attributes); i++ {
				var attr = prevImg.Attributes[i]
				if attr.value == "" {
					continue
				}

				if attr.name == "src" || attr.name == "srcset" || imgExtensions.MatchString(attr.value) {
					if newImg.GetAttribute(attr.name) == attr.value {
						continue
					}

					var attrName = attr.name
					if newImg.HasAttribute(attrName) {
						attrName = "data-old-" + attrName
					}
					newImg.SetAttribute(attrName, attr.value)
				}
			}

			noscript.ParentNode.ReplaceChild(div.FirstElementChild(), prevElement)
		}
	}
}

// Removes script tags from the document.
func (r *Readability) removeScripts(doc *Node) {
	r.removeNodes(r.getAllNodesWithTag(doc, "script", "noscript"), nil)
}

// Check if this node has only whitespace and a single element with given tag
// Returns false if the DIV node contains non-empty text nodes
// or if it contains no element with given tag or more than 1 element.
func (r *Readability) hasSingleTagInsideElement(element *Node, tag string) bool {
	// There should be exactly 1 element child with given tag
	if len(element.Children) != 1 || element.Children[0].TagName != tag {
		return false
	}

	// And there should be no text nodes with real content
	return !r.someNode(element.ChildNodes, func(n *Node) bool {
		return n.NodeType == textNode &&
			hasContent.MatchString(n.GetTextContent())
	})
}

func (r *Readability) isElementWithoutContent(n *Node) bool {
	return n.NodeType == elementNode &&
		len([]rune(strings.TrimSpace(n.GetTextContent()))) == 0 &&
		(len(n.Children) == 0 || len(n.Children) == len(n.getElementsByTagName("br"))+len(n.getElementsByTagName("hr")))
}

// Determine whether element has any children block level elements.
func (r *Readability) hasChildBlockElement(element *Node) bool {
	return r.someNode(element.ChildNodes, func(n *Node) bool {
		return slices.Contains(divToPElemns, n.TagName) ||
			r.hasChildBlockElement(n)
	})
}

// Determine if a node qualifies as phrasing content.
// see: https://developer.mozilla.org/en-US/docs/Web/Guide/HTML/Content_categories#Phrasing_content
func (r *Readability) isPhrasingContent(n *Node) bool {
	return n.NodeType == textNode || slices.Contains(phrasingElems, n.TagName) ||
		((n.TagName == "A" || n.TagName == "DEL" || n.TagName == "INS") &&
			r.everyNode(n.ChildNodes, r.isPhrasingContent))
}

func (r *Readability) isWhitespace(n *Node) bool {
	return (n.NodeType == textNode && len(strings.TrimSpace(n.GetTextContent())) == 0) ||
		(n.NodeType == elementNode && n.TagName == "BR")
}

// Get the inner text of a node - cross browser compatibly.
// This also strips out any excess whitespace to be found ('normalizeSpaces', defaults to true).
func (r *Readability) getInnerText(e *Node, normalizeSpaces bool) string {
	var textContent = strings.TrimSpace(e.GetTextContent())
	if normalizeSpaces {
		return normalize.ReplaceAllString(textContent, " ")
	}
	return textContent
}

// Get the number of times a string s appears in the node e.
func (r *Readability) getCharCount(e *Node, s string) int {
	return len(strings.Split(r.getInnerText(e, true), s)) - 1
}

// Remove the style attribute on every e and under.
// TODO: Test if getElementsByTagName(*) is faster.
func (r *Readability) cleanStyles(e *Node) {
	if e == nil || strings.ToLower(e.TagName) == "svg" {
		return
	}

	// Remove `style` and deprecated presentational attributes
	for i := 0; i < len(presentationalAttribute); i++ {
		e.RemoveAttribute(presentationalAttribute[i])
	}

	if slices.Contains(deprecatedSizeAttributeElems, e.TagName) {
		e.RemoveAttribute("width")
		e.RemoveAttribute("height")
	}

	var cur = e.FirstElementChild()
	for cur != nil {
		r.cleanStyles(cur)
		cur = cur.NextElementSibling
	}
}

// Get the density of links as a percentage of the content
// This is the amount of text that is inside a link divided by the total text in the node.
func (r *Readability) getLinkDensity(element *Node) float64 {
	var textLength = len([]rune(r.getInnerText(element, true)))
	if textLength == 0 {
		return 0
	}

	var linkLength = 0.0

	// XXX implement _reduceNodeList?
	for _, linkNode := range element.getElementsByTagName("a") {
		var href = linkNode.GetAttribute("href")
		var coefficient = 1.0
		if href != "" && hashUrl.MatchString(href) {
			coefficient = 0.3
		}
		linkLength += float64(len([]rune(r.getInnerText(linkNode, true)))) * coefficient
	}

	return linkLength / float64(textLength)
}

// Get an elements class/id weight. Uses regular expressions to tell if this
// element looks good or bad.
func (r *Readability) getClassWeight(e *Node) float64 {
	if !r.flagIsActive(flagWeightClasses) {
		return 0
	}

	var weight = 0

	// Look for a special classname
	if e.GetClassName() != "" {
		if negative.MatchString(e.GetClassName()) {
			weight -= 25
		}
		if positive.MatchString(e.GetClassName()) {
			weight += 25
		}
	}

	// Look for a special ID
	if e.GetId() != "" {
		if negative.MatchString(e.GetId()) {
			weight -= 25
		}
		if positive.MatchString(e.GetId()) {
			weight += 25
		}
	}

	return float64(weight)
}

// Clean a node of all elements of type "tag".
// (Unless it's a youtube/vimeo video. People love movies.)
func (r *Readability) clean(e *Node, tag string) {

	var isEmbed = slices.Contains([]string{"object", "embed", "iframe"}, tag)

	r.removeNodes(r.getAllNodesWithTag(e, tag), func(element *Node) bool {
		// Allow youtube and vimeo videos through as people usually want to see those.
		if isEmbed {
			// First, check the elements attributes to see if any of them contain youtube or vimeo
			for i := 0; i < len(element.Attributes); i++ {
				if r.options.allowedVideoRegex.MatchString(element.Attributes[i].getValue()) {
					return false
				}
			}

			// For embed with <object> tag, check inner HTML as well.
			if element.TagName == "object" && r.options.allowedVideoRegex.MatchString(element.GetInnerHTML()) {
				return false
			}
		}
		return true
	})
}

// Check if a given node has one of its ancestor tag name matching the
// provided one.
func (r *Readability) hasAncestorTag(n *Node, tagName string, maxDepth int, filterFn func(*Node) bool) bool {
	tagName = strings.ToUpper(tagName)
	var depth = 0
	for n.ParentNode != nil {
		if maxDepth > 0 && depth > maxDepth {
			return false
		}
		if n.ParentNode.TagName == tagName && (filterFn == nil || filterFn(n.ParentNode)) {
			return true
		}
		n = n.ParentNode
		depth++
	}
	return false
}

// Return an object indicating how many rows and columns this table has.
func (r *Readability) getRowAndColumnCount(table *Node) (int, int) {
	var rows = 0
	var columns = 0
	var trs = table.getElementsByTagName("tr")
	for i := 0; i < len(trs); i++ {
		var rowspan = trs[i].GetAttribute("rowspan")
		var rs int
		if rowspan != "" {
			num, err := strconv.Atoi(rowspan)
			if err != nil {
				slog.Error(err.Error())
			}
			rs = num
		}
		if rs != 0 {
			rows += rs
		} else {
			rows += 1
		}

		// Now look for column-related info
		var columnsInThisRow = 0
		var cells = trs[i].getElementsByTagName("td")
		for j := 0; j < len(cells); j++ {
			var colspan = cells[j].GetAttribute("colspan")
			var cs int
			if colspan != "" {
				num, err := strconv.Atoi(colspan)
				if err != nil {
					slog.Error(err.Error())
				}
				cs = num
			}
			if cs != 0 {
				columnsInThisRow += cs
			} else {
				columnsInThisRow += 1
			}
		}
		columns = int(math.Max(float64(columns), float64(columnsInThisRow)))
	}
	return rows, columns
}

// Look for 'data' (as opposed to 'layout') tables, for which we use
// similar checks as
// https://searchfox.org/mozilla-central/rev/f82d5c549f046cb64ce5602bfd894b7ae807c8f8/accessible/generic/TableAccessible.cpp#19
func (r *Readability) markDataTables(root *Node) {

	var tables = root.getElementsByTagName("table")
	for i := 0; i < len(tables); i++ {
		var table = tables[i]
		var role = table.GetAttribute("role")
		if role == "presentation" {
			table.ReadabilityDataTable = &readabilityDataTable{value: false}
			continue
		}

		var datatable = table.GetAttribute("datatable")
		if datatable == "0" {
			table.ReadabilityDataTable = &readabilityDataTable{value: false}
			continue
		}

		var summary = table.GetAttribute("summary")
		if summary != "" {
			table.ReadabilityDataTable = &readabilityDataTable{value: true}
			continue
		}

		if captions := table.getElementsByTagName("caption"); len(captions) > 0 && captions[0] != nil && len(captions[0].ChildNodes) > 0 {
			table.ReadabilityDataTable = &readabilityDataTable{value: true}
		}

		// If the table has a descendant with any of these tags, consider a data table:
		var dataTableDescendants = []string{"col", "colgroup", "tfoot", "thead", "th"}
		var descendantExists = func(tag string) bool {
			elements := table.getElementsByTagName(tag)
			return len(elements) != 0 && elements[0] != nil
		}

		if slices.ContainsFunc(dataTableDescendants, descendantExists) {
			slog.Debug("Data table because found data-y descendant")
			table.ReadabilityDataTable = &readabilityDataTable{value: true}
			continue
		}

		// Nested tables indicate a layout table:
		if tables := table.getElementsByTagName("table"); len(tables) > 0 && tables[0] != nil {
			table.ReadabilityDataTable = &readabilityDataTable{value: false}
		}

		var rows, columns = r.getRowAndColumnCount(table)
		if rows >= 10 || columns > 4 {
			table.ReadabilityDataTable = &readabilityDataTable{value: true}
			continue
		}
		// Now just go by size entirely:
		table.ReadabilityDataTable = &readabilityDataTable{
			value: (rows*columns > 10),
		}
	}
}

// convert images and figures that have properties like data-src into images that can be loaded without JS
func (r *Readability) fixLazyImages(root *Node) {

	for _, elem := range r.getAllNodesWithTag(root, "img", "picture", "figure") {
		// In some sites (e.g. Kotaku), they put 1px square image as base64 data uri in the src attribute.
		// So, here we check if the data uri is too short, just might as well remove it.

		if elem.GetSrc() != "" && b64DataUrl.MatchString(elem.GetSrc()) {
			// Make sure it's not SVG, because SVG can have a meaningful image in under 133 bytes.
			var parts = b64DataUrl.FindAllStringSubmatch(elem.GetSrc(), -1)
			if parts[0][1] == "image/svg+xml" {
				continue
			}

			// Make sure this element has other attributes which contains image.
			// If it doesn't, then this src is important and shouldn't be removed.
			var srcCouldBeRemoved = false
			for i := 0; i < len(elem.Attributes); i++ {
				var attr = elem.Attributes[i]
				if attr.name == "src" {
					continue
				}

				if imgExtensions.MatchString(attr.value) {
					srcCouldBeRemoved = true
					break
				}
			}

			// Here we assume if image is less than 100 bytes (or 133B after encoded to base64)
			// it will be too small, therefore it might be placeholder image.
			if srcCouldBeRemoved {
				var b64starts = base64Starts.FindStringIndex(elem.GetSrc())[0] + 7
				var b64length = len([]rune(elem.GetSrc())) - b64starts
				if b64length < 133 {
					elem.RemoveAttribute("src")
				}
			}
		}

		// also check for "null" to work around https://github.com/jsdom/jsdom/issues/2580
		if (elem.GetSrc() != "" || (elem.GetSrcset() != "" && elem.GetSrcset() != "null")) && !strings.Contains(strings.ToLower(elem.GetClassName()), "lazy") {
			continue
		}

		for j := 0; j < len(elem.Attributes); j++ {
			attr := elem.Attributes[j]
			if attr.name == "src" || attr.name == "srcset" || attr.name == "alt" {
				continue
			}
			var copyTo string
			if imgExtensionsWithSpacesAndNum.MatchString(attr.value) {
				copyTo = "srcset"
			} else if imgExtensionsAmongText.MatchString(attr.value) {
				copyTo = "src"
			}

			if copyTo != "" {
				//if this is an img or picture, set the attribute directly
				if elem.TagName == "IMG" || elem.TagName == "PICTURE" {
					elem.SetAttribute(copyTo, attr.value)
				} else if elem.TagName == "FIGURE" && len(r.getAllNodesWithTag(elem, "img", "picture")) == 0 {
					//if the item is a <figure> that does not contain an image or picture, create one and place it inside the figure
					//see the nytimes-3 testcase for an example
					var img = r.doc.createElementNode("img")
					img.SetAttribute(copyTo, attr.value)
					elem.AppendChild(img)
				}
			}
		}
	}
}

func (r *Readability) getTextDensity(e *Node, tags ...string) float64 {
	var textLength = len(r.getInnerText(e, true))
	if textLength == 0 {
		return 0
	}

	var childrenLength = 0
	var children = r.getAllNodesWithTag(e, tags...)
	for _, child := range children {
		childrenLength += len(r.getInnerText(child, true))
	}
	return float64(childrenLength) / float64(textLength)
}

// Clean an element of all tags of type "tag" if they look fishy.
// "Fishy" is an algorithm based on content length, classnames, link density, number of images & embeds, etc.
func (r *Readability) cleanConditionally(e *Node, tag string) {
	if !r.flagIsActive(flagCleanConditionally) {
		return
	}

	// Gather counts for other typical elements embedded within.
	// Traverse backwards so we can remove nodes at the same time
	// without effecting the traversal.
	//
	// TODO: Consider taking into account original contentScore here.

	r.removeNodes(r.getAllNodesWithTag(e, tag), func(n *Node) bool {
		// First check if this node IS data table, in which case don't remove it.
		var isDataTable = func(t *Node) bool {
			return t.ReadabilityDataTable != nil && t.ReadabilityDataTable.value
		}

		var isList = (tag == "ul" || tag == "ol")
		if !isList {
			var listLength = 0
			var listNodes = r.getAllNodesWithTag(n, "ul", "ol")
			for _, list := range listNodes {
				listLength += len(r.getInnerText(list, true))
			}
			isList = float64(listLength)/float64(len(r.getInnerText(n, true))) > 0.9
		}

		if tag == "table" && isDataTable(n) {
			return false
		}

		// Next check if we're inside a data table, in which case don't remove it as well.
		if r.hasAncestorTag(n, "table", -1, isDataTable) {
			return false
		}

		if r.hasAncestorTag(n, "code", 3, nil) {
			return false
		}

		var weight = r.getClassWeight(n)

		slog.Debug("Cleaning Conditionally", "node", n)

		var contentScore = 0.0

		if weight+contentScore < 0 {
			return true
		}

		if r.getCharCount(n, ",") < 10 {
			// If there are not very many commas, and the number of
			// non-paragraph elements is more than paragraphs or other
			// ominous signs, remove the element.
			var p = len(n.getElementsByTagName("p"))
			var img = len(n.getElementsByTagName("img"))
			var li = len(n.getElementsByTagName("li")) - 100
			var input = len(n.getElementsByTagName("input"))
			var headingDensity = r.getTextDensity(n, "h1", "h2", "h3", "h4", "h5", "h6")

			var embedCount = 0
			var embeds = r.getAllNodesWithTag(n, "object", "embed", "iframe")

			for i := 0; i < len(embeds); i++ {
				// If this embed has attribute that matches video regex, don't delete it.
				for j := 0; j < len(embeds[i].Attributes); j++ {
					if r.options.allowedVideoRegex != nil && r.options.allowedVideoRegex.MatchString(embeds[i].Attributes[j].value) {
						return false
					}
				}

				// For embed with <object> tag, check inner HTML as well.
				if embeds[i].TagName == "object" && r.options.allowedVideoRegex != nil && r.options.allowedVideoRegex.MatchString(embeds[i].GetInnerHTML()) {
					return false
				}

				embedCount++
			}

			var linkDensity = r.getLinkDensity(n)
			var contentLength = len([]rune(r.getInnerText(n, true)))

			var haveToRemove = (img > 1 && float64(p)/float64(img) < 0.5 && !r.hasAncestorTag(n, "figure", 3, nil)) ||
				(!isList && li > p) ||
				(input > int(math.Floor(float64(p)/3.0))) ||
				(!isList && headingDensity < 0.9 && contentLength < 25 && (img == 0 || img > 2) && !r.hasAncestorTag(n, "figure", 3, nil)) ||
				(!isList && weight < 25 && linkDensity > 0.2) ||
				(weight >= 25 && linkDensity > 0.5) ||
				((embedCount == 1 && contentLength < 75) || embedCount > 1)

			// Allow simple lists of images to remain in pages
			if isList && haveToRemove {
				for x := 0; x < len(n.Children); x++ {
					var child = n.Children[x]
					// Don't filter in lists with li's that contain more than one child
					if len(child.Children) > 1 {
						return haveToRemove
					}
				}
				var liCount = len(n.getElementsByTagName("li"))
				// Only allow the list to remain if every li contains an image
				if img == liCount {
					return false
				}
			}
			return haveToRemove
		}
		return false
	})
}

// Clean out elements that match the specified conditions
func (r *Readability) cleanMatchedNodes(e *Node, filter func(*Node, string) bool) {
	var endOfSearchMarkerNode = r.getNextNode(e, true)
	var next = r.getNextNode(e, false)
	for next != nil && next != endOfSearchMarkerNode {
		if filter(next, next.GetClassName()+" "+next.GetId()) {
			next = r.removeAndGetNext(next)
		} else {
			next = r.getNextNode(next, false)
		}
	}
}

// Clean out spurious headers from an Element.
func (r *Readability) cleanHeaders(n *Node) {
	var headingNodes = r.getAllNodesWithTag(n, "h1", "h2")
	r.removeNodes(headingNodes, func(nn *Node) bool {
		var shouldRemove = r.getClassWeight(nn) < 0
		if shouldRemove {
			slog.Debug("Removing header with low class weight", "node", nn)
		}
		return shouldRemove
	})
}

// Check if this node is an H1 or H2 element whose content is mostly
// the same as the article title.
func (r *Readability) headerDuplicatesTitle(n *Node) bool {
	if n.TagName != "H1" && n.TagName != "H2" {
		return false
	}
	var heading = r.getInnerText(n, false)
	slog.Debug("Evaluating similarity of header", "heading", heading, "articleTitle", r.articleTitle)
	return r.textSimilarity(r.articleTitle, heading) > 0.75
}

func (r *Readability) flagIsActive(flag int) bool {
	return r.flags&flag > 0
}

func (r *Readability) removeFlag(flag int) {
	r.flags = r.flags & ^flag
}

func isProbablyVisible(n *Node) bool {
	// Have to null-check node.style and node.className.indexOf to deal with SVG and MathML nodes.
	return (n.style == nil || n.style.getStyle("display") != "none") &&
		(n.style == nil || n.style.getStyle("visibility") != "hidden") &&
		!n.HasAttribute("hidden") &&
		(!n.HasAttribute("aria-hidden") || n.GetAttribute("aria-hidden") != "true" || (n.GetClassName() != "" && strings.Contains(n.GetClassName(), "fallback-image")))
}

// Runs readability.
// Workflow:
//  1. Prep the document by removing script tags, css, etc.
//  2. Build readability's DOM tree.
//  3. Grab the article content from the current dom tree.
//  4. Replace the current DOM tree with the new one.
//  5. Read peacefully.
func (r *Readability) Parse() (*Result, error) {
	// Avoid parsing too large documents, as per configuration option
	if r.options.maxElemsToParse > 0 {
		var numTags = len(r.doc.getElementsByTagName("*"))
		if numTags > r.options.maxElemsToParse {
			return nil, fmt.Errorf("aborting parsing document: elements_found=%d", numTags)
		}
	}

	// Unwrap image from noscript
	r.unwrapNoscriptImages(r.doc)

	// Extract JSON-LD metadata before removing scripts
	var jsonLd *metadata
	if !r.options.disableJSONLD {
		jsonLd = r.getJSONLD(r.doc)
	}

	// Remove script tags from the document.
	r.removeScripts(r.doc)

	r.prepDocument()

	var metadata = r.getArticleMetadata(jsonLd)
	r.articleTitle = metadata.title

	var articleContent = r.grabArticle(nil)
	if articleContent == nil {
		return nil, fmt.Errorf("cannot grab article")
	}

	slog.Debug("grabbed", "articleContent.innerHTML", articleContent.GetInnerHTML())

	r.postProcessContent(articleContent)

	// If we haven't found an excerpt in the article's metadata, use the article's
	// first paragraph as the excerpt. This is used for displaying a preview of
	// the article's content.
	if metadata.excerpt == "" {
		var paragraphs = articleContent.getElementsByTagName("p")
		if len(paragraphs) > 0 {
			metadata.excerpt = strings.TrimSpace(paragraphs[0].GetTextContent())
		}
	}

	htmlContent := r.options.serializer(articleContent)

	var textContent string
	if r.options.html2text != nil {
		textContent = r.options.html2text(htmlContent)
	} else {
		textContent = articleContent.GetTextContent()
	}

	return &Result{
		Title:         r.articleTitle,
		Byline:        anyOf(metadata.byline, r.articleByline),
		Dir:           r.articleDir,
		Lang:          r.articleLang,
		HTMLContent:   htmlContent,
		TextContent:   textContent,
		Length:        len([]rune(textContent)),
		Excerpt:       metadata.excerpt,
		SiteName:      anyOf(metadata.siteName, r.articleSiteName),
		PublishedTime: metadata.publishedTime,
	}, nil
}
