package readability

import (
	"log/slog"
	"regexp"

	"golang.org/x/net/html"
)

type Options struct {
	logLevel          slog.Level
	maxElemsToParse   int
	nbTopCandidates   int
	charThreshold     int
	classesToPreserve []string
	keepClasses       bool
	serializer        func(*node) string
	disableJSONLD     bool
	allowedVideoRegex *regexp.Regexp
	minContentLength  int
	minScore          float64
	visibilityChecker func(*html.Node) bool
}

type Option func(*Options)

func defaultOpts() *Options {
	return &Options{
		logLevel:          slog.LevelError,
		maxElemsToParse:   defaultMaxElemsToParse,
		nbTopCandidates:   defaultNTopCandidates,
		charThreshold:     defaultCharThreshold,
		classesToPreserve: classesToPreserve,
		allowedVideoRegex: videos,
		serializer: func(n *node) string {
			return n.getInnerHTML()
		},
		minScore:          20,
		minContentLength:  140,
		visibilityChecker: isNodeVisible,
	}
}

func LogLevel(l slog.Level) Option {
	return func(o *Options) {
		o.logLevel = l
	}
}

func MaxElemsToParse(n int) Option {
	return func(o *Options) {
		o.maxElemsToParse = n
	}
}

func NTopCandidates(n int) Option {
	return func(o *Options) {
		o.nbTopCandidates = n
	}
}

func CharThreshold(n int) Option {
	return func(o *Options) {
		o.charThreshold = n
	}
}

func ClassesToPreserve(classes ...string) Option {
	return func(o *Options) {
		o.classesToPreserve = append(o.classesToPreserve, classes...)
	}
}

func KeepClasses(b bool) Option {
	return func(o *Options) {
		o.keepClasses = b
	}
}

func Serializer(f func(*node) string) Option {
	return func(o *Options) {
		o.serializer = f
	}
}

func DisableJSONLD(b bool) Option {
	return func(o *Options) {
		o.disableJSONLD = b
	}
}

func AllowedVideoRegex(rgx *regexp.Regexp) Option {
	return func(o *Options) {
		o.allowedVideoRegex = rgx
	}
}

func MinContentLength(len int) Option {
	return func(o *Options) {
		o.minContentLength = len
	}
}

func MinScore(score float64) Option {
	return func(o *Options) {
		o.minScore = score
	}
}

func VisibilityChecker(f func(*html.Node) bool) Option {
	return func(o *Options) {
		o.visibilityChecker = f
	}
}
