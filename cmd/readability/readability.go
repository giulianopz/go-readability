package main

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/giulianopz/go-readability"
)

var (
	output  string
	verbose bool
)

func handle(err error) {
	if err != nil {
		exit(err.Error())
	}
}

func exit(msg string) {
	fmt.Println(msg)
	os.Exit(1)
}

func main() {

	flag.StringVar(&output, "output", "text", "the result output format: 'text' or 'html'")
	flag.StringVar(&output, "o", "text", "the result output format: 'text' or 'html'")
	flag.BoolVar(&verbose, "verbose", false, "enable logs")
	flag.BoolVar(&verbose, "v", false, "enable logs")
	flag.Parse()

	if !verbose {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	}

	url := flag.Arg(0)
	if url == "" {
		exit("missing url")
	}

	resp, err := http.Get(url)
	handle(err)

	bs, err := io.ReadAll(resp.Body)
	handle(err)

	parser, err := readability.New(string(bs), url)
	handle(err)

	res, err := parser.Parse()
	handle(err)

	if output == "html" {
		fmt.Print(res.HTMLContent)
	} else {
		fmt.Print(res.TextContent)
	}
}
