package example

import (
	"fmt"

	"github.com/giulianopz/go-readability"
)

var htmlSource = `<!DOCTYPE html><html>
<head>
<meta charset="utf-8">
<title>
Redis will remain BSD licensed - &lt;antirez&gt;
</title>
<meta content="index" name="robots">
<meta name="viewport" content="width=device-width, initial-scale=1">
<link href="/css/style.css?v=14" rel="stylesheet" type="text/css">
<link href="/images/favicon.png" rel="shortcut icon">
<link href="/rss" rel="alternate" type="application/rss+xml">
<link href="http://fonts.googleapis.com/css?family=Inconsolata" rel="stylesheet" type="text/css">
<script src="/js/jquery.1.6.4.min.js"></script><script src="/js/app.js?v=10"></script>
</head>
<body>
<div id="container">
<header><h1><a href="/">&lt;antirez&gt;</a></h1><nav></nav> <nav id="account"></nav></header><div id="content">
<section id="newslist"><article data-news-id="120"><h2><a href="/news/120">Redis will remain BSD licensed</a></h2></article></section><topcomment><article class="comment" style="margin-left:0px" data-comment-id="120-" id="120-"><span class="info"><span class="username"><a href="/user/antirez">antirez</a></span> 2095 days ago. 170643 views.  </span><pre>Today a page about the new Common Clause license in the Redis Labs web site was interpreted as if Redis itself switched license. This is not the case, Redis is, and will remain, BSD licensed. However in the era of [edit] uncontrollable spreading of information, my attempts to provide the correct information failed, and I’m still seeing everywhere “Redis is no longer open source”. The reality is that Redis remains BSD, and actually Redis Labs did the right thing supporting my effort to keep the Redis core open as usually.

What is happening instead is that certain Redis modules, developed inside Redis Labs, are now released under the Common Clause (using Apache license as a base license). This means that basically certain enterprise add-ons, instead of being completely closed source as they could be, will be available with a more permissive license.

I think that Redis Labs Common Clause page did not provide a clear and complete information, but software companies often make communication errors, it happens. To me however, it looks more important that while running a system software business in the “cloud era” (LOL) is very challenging using an open source license, yet Redis Labs totally understood and supported the idea that the Redis core is an open source project, in the *most permissive license ever*, that is, BSD, and during the years provided a lot of funding to the project.

The reason why certain modules developed internally at Redis Labs are switching license, is because they are added value that Redis Labs wants to be able to provide only to end users that are willing to compile and install the system themselves, or to the Redis Labs customers using their services. But it’s not ok to give away that value to everybody willing to resell it. An example of such module is RediSearch: it was AGPL and is now going to be Apache + Common Clause.

About myself, I’ll keep writing BSD code for Redis. For Redis modules I’ll develop, such as Disque, I’ll pick AGPL instead, for similar reasons: we live in a “Cloud-poly”, so it’s a good idea to go forward with licenses that will force other SaaS companies to redistribute back their improvements. However this does not apply to Redis itself. Redis at this point is a 10 years collective effort, the base for many other things that we can do together, and this base must be as available as possible, that is, BSD licensed.

We at Redis Labs are sorry for the confusion generated by the Common Clause page, and my colleagues are working to fix the page with better wording.</pre></article></topcomment><a href="http://invece.org/wohpe_EN_six_chapters.epub">🚀 Dear reader, the first six chapters of my AI sci-fi novel, WOHPE, are now available as a free eBook. Click here to get it.</a><div id="disqus_thread_outdiv">
<div id="disqus_thread"></div>
</div>
<script type="text/javascript">
    /* * * CONFIGURATION VARIABLES: EDIT BEFORE PASTING INTO YOUR WEBPAGE * * */
    var disqus_shortname = 'antirezweblog'; // required: replace example with your forum shortname

    // The following are highly recommended additional parameters. Remove the slashes in front to use.
    var disqus_identifier = 'antirez_weblog_new_120';
    var disqus_url = 'http://antirez.com/news/120';

    /* * * DON'T EDIT BELOW THIS LINE * * */
    (function() {
        var dsq = document.createElement('script'); dsq.type = 'text/javascript'; dsq.async = true;
        dsq.src = 'http://' + disqus_shortname + '.disqus.com/embed.js';
        (document.getElementsByTagName('head')[0] || document.getElementsByTagName('body')[0]).appendChild(dsq);
    })();
</script>
<noscript>Please enable JavaScript to view the <a href="http://disqus.com/?ref_noscript">comments powered by Disqus.</a></noscript>
<a href="http://disqus.com" class="dsq-brlink">blog comments powered by <span class="logo-disqus">Disqus</span></a>

</div>
<footer><a href="/rss">rss feed</a> | <a href="http://twitter.com/antirezdotcom">twitter</a> | <a href="https://groups.google.com/forum/?fromgroups#!forum/redis-db">google group</a> | <a href="http://oldblog.antirez.com">old site</a></footer><script type="text/javascript">
    /* * * CONFIGURATION VARIABLES: EDIT BEFORE PASTING INTO YOUR WEBPAGE * * */
    var disqus_shortname = 'antirezweblog';

    /* * * DON'T EDIT BELOW THIS LINE * * */
    (function () {
        var s = document.createElement('script'); s.async = true;
        s.type = 'text/javascript';
        s.src = 'http://' + disqus_shortname + '.disqus.com/count.js';
        (document.getElementsByTagName('HEAD')[0] || document.getElementsByTagName('BODY')[0]).appendChild(s);
    }());
</script>:

</div>

<script>
  (function(i,s,o,g,r,a,m){i['GoogleAnalyticsObject']=r;i[r]=i[r]||function(){
  (i[r].q=i[r].q||[]).push(arguments)},i[r].l=1*new Date();a=s.createElement(o),
  m=s.getElementsByTagName(o)[0];a.async=1;a.src=g;m.parentNode.insertBefore(a,m)
  })(window,document,'script','//www.google-analytics.com/analytics.js','ga');

  ga('create', 'UA-46379280-1', 'antirez.com');
  ga('send', 'pageview');

</script>
</body>

</html>`

func handle(err error) {
	if err != nil {
		panic(err)
	}
}

func ExampleReadability_Parse() {

	reader, err := readability.New(
		htmlSource,
		"http://antirez.com/news/120",
		readability.ClassesToPreserve("caption"),
	)
	handle(err)

	result, err := reader.Parse()
	handle(err)

	fmt.Println(result.TextContent)
	// Output
	//
	//antirez 2095 days ago. 170643 views.  Today a page about the new Common Clause license in the Redis Labs web site was interpreted as if Redis itself switched license. This is not the case, Redis is, and will remain, BSD licensed. However in the era of [edit] uncontrollable spreading of information, my attempts to provide the correct information failed, and I’m still seeing everywhere “Redis is no longer open source”. The reality is that Redis remains BSD, and actually Redis Labs did the right thing supporting my effort to keep the Redis core open as usually.
	//
	//What is happening instead is that certain Redis modules, developed inside Redis Labs, are now released under the Common Clause (using Apache license as a base license). This means that basically certain enterprise add-ons, instead of being completely closed source as they could be, will be available with a more permissive license.
	//
	//I think that Redis Labs Common Clause page did not provide a clear and complete information, but software companies often make communication errors, it happens. To me however, it looks more important that while running a system software business in the “cloud era” (LOL) is very challenging using an open source license, yet Redis Labs totally understood and supported the idea that the Redis core is an open source project, in the *most permissive license ever*, that is, BSD, and during the years provided a lot of funding to the project.
	//
	//The reason why certain modules developed internally at Redis Labs are switching license, is because they are added value that Redis Labs wants to be able to provide only to end users that are willing to compile and install the system themselves, or to the Redis Labs customers using their services. But it’s not ok to give away that value to everybody willing to resell it. An example of such module is RediSearch: it was AGPL and is now going to be Apache + Common Clause.
	//
	//About myself, I’ll keep writing BSD code for Redis. For Redis modules I’ll develop, such as Disque, I’ll pick AGPL instead, for similar reasons: we live in a “Cloud-poly”, so it’s a good idea to go forward with licenses that will force other SaaS companies to redistribute back their improvements. However this does not apply to Redis itself. Redis at this point is a 10 years collective effort, the base for many other things that we can do together, and this base must be as available as possible, that is, BSD licensed.
	//
	//We at Redis Labs are sorry for the confusion generated by the Common Clause page, and my colleagues are working to fix the page with better wording.🚀 Dear reader, the first six chapters of my AI sci-fi novel, WOHPE, are now available as a free eBook. Click here to get it.
	//
	//
	//blog comments powered by Disqus
}

func ExampleIsProbablyReaderable() {

	isReaderable := readability.IsProbablyReaderable(
		htmlSource,
		readability.MinContentLength(140),
		readability.MinScore(20),
	)
	fmt.Printf("Contains any text? %t\n", isReaderable)
	// Output:
	// Contains any text? true
}
