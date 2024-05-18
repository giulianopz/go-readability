# go-readability

A Go port of Mozilla [Readability.js](https://github.com/mozilla/readability), an algorithm based on heuristics (e.g. link density, text similarity, number of images, etc.) that [just somehow work well](https://stackoverflow.com/a/4240037) which powers the [Firefox Reader View](https://support.mozilla.org/kb/firefox-reader-view-clutter-free-web-pages) offering a distraction-free reading experience for articles, blog posts, and other text-heavy web pages by removing ads, GDPR-compliant cookie banners and other unsolicited junk.

This port uses only the minimal DOM parser bundled with the original lib, without resorting to the Go stdlib (`net/html`). The rest of the source code is aligned with the latest commit ([97db40b](https://github.com/mozilla/readability/commit/97db40ba035a2de5e42d1ac7437893cf0da31d76)) on the main branch.


## A Bit of History

Readability.js maintained by Mozilla is based on a JavaScript [bookmarklet](https://en.wikipedia.org/wiki/Bookmarklet) developed by Arc90, a consulting firm which was experimenting with Web techonlogy at that time and which used to share some of their stuff as open source software. The company site has long disappeared but it can still be found with the [Wayback Machine](https://web.archive.org/web/20091225055930/http://lab.arc90.com//2009//03//02//readability//).

The source code was released in 2009 under the Apache 2.0 software license on [Google Code](https://code.google.com/archive/p/arc90labs-readability/) before being abandoned in 2010 to be repackaged as a web service called [Readability.com](https://en.wikipedia.org/wiki/Readability_(service)), then discontinued in 2016. The main source code contributor was Chris Dary ([@umbrae](http://www.umbrae.net/)). 

Most modern browsers still use one of the available forks of the Arc90 original implementation when displaying web pages in reading mode.

For a historical and detailed analysis of this topic, please read this excellent series of articles by [Daniel Aleksandersen](https://www.ctrl.blog/entry/browser-reading-mode-parsers.html).


## Basic usage

Add a dependency for the package:
```bash
go get -u github.com/giulianopz/go-readability
```

Get text content from a web page article:
```go
package main

import (
	"fmt"

	"github.com/giulianopz/go-readability"
)

func main() {

	var htmlSource = `<!DOCTYPE html>
<html>

<head>
	<meta charset="utf-8" />
	<title>
		Redis will remain BSD licensed - &lt;antirez&gt;
	</title>
	<link href="/rss" rel="alternate" type="application/rss+xml" />
</head>

<body>
	<div id="container">
		<header>
			<h1><a href="/">&lt;antirez&gt;</a></h1>
		</header>
		<div id="content">
			<section id="newslist">
				<article data-news-id="120">
					<h2><a href="/news/120">Redis will remain BSD licensed</a></h2>
				</article>
			</section>
			<article class="comment" style="margin-left:0px" data-comment-id="120-" id="120-"><span class="info"><span
						class="username"><a href="/user/antirez">antirez</a></span> 2095 days ago.
					170643 views. </span>
				<pre>Today a page about the new Common Clause license in the Redis Labs web site was interpreted as if Redis itself switched license. This is not the case, Redis is, and will remain, BSD licensed. However in the era of [edit] uncontrollable spreading of information, my attempts to provide the correct information failed, and I’m still seeing everywhere “Redis is no longer open source”. The reality is that Redis remains BSD, and actually Redis Labs did the right thing supporting my effort to keep the Redis core open as usually.

				[...]

We at Redis Labs are sorry for the confusion generated by the Common Clause page, and my colleagues are working to fix the page with better wording.</pre>
			</article>
		</div>
	</div>
</body>

</html>`

	isReaderable := readability.IsProbablyReaderable(htmlSource)
	fmt.Printf("Contains any text?: %t\n", isReaderable)

	reader, err := readability.New(
		htmlSource,
		"http://antirez.com/news/120",
		readability.ClassesToPreserve("caption"),
	)
	if err != nil {
		panic(err)
	}

	result, err := reader.Parse()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Title: %s\n", result.Title)
	fmt.Printf("Author: %s\n", result.Byline)
	fmt.Printf("Length: %d\n", result.Length)
	fmt.Printf("Excerpt: %s\n", result.Excerpt)
	fmt.Printf("SiteName: %s\n", result.SiteName)
	fmt.Printf("Lang: %s\n", result.Lang)
	fmt.Printf("PublishedTime: %s\n", result.PublishedTime)
	fmt.Printf("Content: %s\n", result.Content)
	fmt.Printf("TextContent: %s\n", result.TextContent)
}
```
