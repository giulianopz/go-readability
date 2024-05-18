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
	"log"
	"math"
	"slices"
	"strings"

	"golang.org/x/net/html"
)

func isNodeVisible(node *html.Node) bool {
	// Have to null-check node.style and node.className.indexOf to deal with SVG and MathML nodes.
	style := attr(node, "style")
	return (style == "" || !slices.Contains(strings.Split(";", style), "display=none")) &&
		(style == "" || !slices.Contains(strings.Split(";", style), "visibility=hidden")) &&
		attr(node, "hidden") == "" &&
		(attr(node, "aria-hidden") == "" || attr(node, "aria-hidden") != "true" || (attr(node, "class") != "" && strings.Contains(attr(node, "class"), "fallback-image")))
}

// Decides whether or not the document is reader-able without parsing the whole thing.
// Options:
//   - options.minContentLength (default 140), the minimum node content length used to decide if the document is readerable
//   - options.minScore (default 20), the minumum cumulated 'score' used to determine if the document is readerable
//   - options.visibilityChecker (default isNodeVisible), the function used to determine if a node is visible
func IsProbablyReaderable(htmlSource string, opts ...Option) bool {

	doc, err := html.Parse(strings.NewReader(htmlSource))
	if err != nil {
		log.Fatal(err)
	}

	var options = defaultOpts()
	for _, opt := range opts {
		opt(options)
	}

	var nodes = querySelectorAll(doc, "p, pre, article")
	// Get <div> nodes which have <br> node(s) and append them into the `nodes` variable.
	// Some articles' DOM structures might look like
	// <div>
	//   Sentences<br>
	//   <br>
	//   Sentences<br>
	// </div>
	var brNodes = querySelectorAll(doc, "div > br")
	if len(brNodes) != 0 {
		var set []*html.Node
		for _, n := range brNodes {
			set = append(set, n.Parent)
		}
		nodes = append(nodes, set...)
	}

	var score = 0.0
	// This is a little cheeky, we use the accumulator 'score' to decide what to return from
	// this callback:
	return slices.ContainsFunc(nodes, func(n *html.Node) bool {
		if !options.visibilityChecker(n) {
			return false
		}

		var matchString = attr(n, "class") + " " + attr(n, "id")
		if unlikelyCandidates.MatchString(matchString) &&
			!okMaybeItsACandidate.MatchString(matchString) {
			return false
		}

		if matches(n, "li p") {
			return false
		}

		var textContentLength = len(strings.TrimSpace(textContent(n)))
		if textContentLength < options.minContentLength {
			return false
		}

		score += math.Sqrt(float64(textContentLength - options.minContentLength))

		return score > options.minScore
	})
}
