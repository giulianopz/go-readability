package readability

import "regexp"

// All of the regular expressions in use within readability.
// Defined up here so we don't instantiate them repeatedly in loops.
var (
	unlikelyCandidates   = regexp.MustCompile(`(?i)-ad-|ai2html|banner|breadcrumbs|combx|comment|community|cover-wrap|disqus|extra|footer|gdpr|header|legends|menu|related|remark|replies|rss|shoutbox|sidebar|skyscraper|social|sponsor|supplemental|ad-break|agegate|pagination|pager|popup|yom-remote`)
	okMaybeItsACandidate = regexp.MustCompile(`(?i)and|article|body|column|content|main|shadow`)
	positive             = regexp.MustCompile(`(?i)article|body|content|entry|hentry|h-entry|main|page|pagination|post|text|blog|story`)
	negative             = regexp.MustCompile(`(?i)-ad-|hidden|^hid$| hid$| hid |^hid |banner|combx|comment|com-|contact|foot|footer|footnote|gdpr|masthead|media|meta|outbrain|promo|related|scroll|share|shoutbox|sidebar|skyscraper|sponsor|shopping|tags|tool|widget`)
	//extraneous           = regexp.MustCompile(`(?i)print|archive|comment|discuss|e[\-]?mail|share|reply|all|login|sign|single|utility`)
	byline = regexp.MustCompile(`(?i)byline|author|dateline|writtenby|p-author`)
	//replaceFonts         = regexp.MustCompile(`(?i)<(\/?)font[^>]*>`)
	normalize     = regexp.MustCompile(`\s{2,}`)
	videos        = regexp.MustCompile(`(?i)\/\/(www\.)?((dailymotion|youtube|youtube-nocookie|player\.vimeo|v\.qq)\.com|(archive|upload\.wikimedia)\.org|player\.twitch\.tv)`)
	shareElements = regexp.MustCompile(`(?i)(\b|_)(share|sharedaddy)(\b|_)`)
	//nextLink             = regexp.MustCompile(`(?i)(next|weiter|continue|>([^\|]|$)|»([^\|]|$))`)
	//prevLink             = regexp.MustCompile(`(prev|earl|old|new|<|«)`)
	tokenize   = regexp.MustCompile(`\W+`)
	whitespace = regexp.MustCompile(`^\s*$`)
	hasContent = regexp.MustCompile(`\S$`)
	hashUrl    = regexp.MustCompile(`^#.+`)
	srcsetUrl  = regexp.MustCompile(`(\S+)(\s+[\d.]+[xw])?(\s*(?:,|$))`)
	b64DataUrl = regexp.MustCompile(`(?i)^data:\s*([^\s;,]+)\s*;\s*base64\s*,`)
	// commas as used in Latin, Sindhi, Chinese and various other scripts.
	// see: https://en.wikipedia.org/wiki/Comma#Comma_variants
	commas = regexp.MustCompile(`\x{002C}|\x{060C}|\x{FE50}|\x{FE10}|\x{FE11}|\x{2E41}|\x{2E34}|\x{2E32}|\x{FF0C}`)
	// See: https://schema.org/Article
	jsonLdArticleTypes   = regexp.MustCompile(`^Article|AdvertiserContentArticle|NewsArticle|AnalysisNewsArticle|AskPublicNewsArticle|BackgroundNewsArticle|OpinionNewsArticle|ReportageNewsArticle|ReviewNewsArticle|Report|SatiricalArticle|ScholarlyArticle|MedicalScholarlyArticle|SocialMediaPosting|BlogPosting|LiveBlogPosting|DiscussionForumPosting|TechArticle|APIReference$`)
	titleFinalPart       = regexp.MustCompile(` [\|\-\\\/>»] `)
	titleSeparators      = regexp.MustCompile(` [\\\/>»] `)
	otherTitleSeparators = regexp.MustCompile(`(?i)(.*)[\|\-\\\/>»] .*`)
	titleFirstPart       = regexp.MustCompile(`(?i)[^\|\-\\\/>»]*[\|\-\\\/>»](.*)`)
	multipleWhitespaces  = regexp.MustCompile(`\s+`)
	singleWhitespace     = regexp.MustCompile(`\s`)
	singleDot            = regexp.MustCompile(`\.`)
	// https://www.dcs.bbk.ac.uk/~ptw/teaching/DBM/XML/slide17.html
	entityReferencesRgx  = regexp.MustCompile(`&(quot|amp|apos|lt|gt);`)
	htmlCharCodesRgx     = regexp.MustCompile(`(?i)&#(?:x([0-9a-fA-F]{1,4})|([0-9]{1,5}));`)
	doubleForwardSlashes = regexp.MustCompile(`//[^/]+`)
	separators           = regexp.MustCompile(`[\|\-\\\/>»]+`)
	dotSpaceOrDollar     = regexp.MustCompile(`\.( |$)`)
	cdata                = regexp.MustCompile(`^\s*<!\[CDATA\[|\]\]>\s*$`)
	schemaUrl            = regexp.MustCompile(`^https?\:\/\/schema\.org\/?$`)
	// property is a space-separated list of values
	propertyPattern = regexp.MustCompile(`(?i)\s*(article|dc|dcterm|og|twitter)\s*:\s*(author|creator|description|published_time|title|site_name)\s*`)
	// name is a single value
	namePattern                   = regexp.MustCompile(`(?i)^\s*(?:(dc|dcterm|og|twitter|weibo:(article|webpage))\s*[\.:]\s*)?(author|creator|description|title|site_name)\s*$`)
	imgExtensions                 = regexp.MustCompile(`\.(jpg|jpeg|png|webp)`)
	base64Starts                  = regexp.MustCompile(`base64\s*`)
	imgExtensionsWithSpacesAndNum = regexp.MustCompile(`\.(jpg|jpeg|png|webp)\s+\d`)
	imgExtensionsAmongText        = regexp.MustCompile(`^\s*\S+\.(jpg|jpeg|png|webp)\S*\s*$`)
)
