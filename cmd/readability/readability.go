package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/giulianopz/go-readability"
)

var output string

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
	flag.Parse()

	url := flag.Arg(0)

	if url == "" {
		exit("missing url")
	}

	resp, err := http.Get(url)
	handle(err)

	bs, err := io.ReadAll(resp.Body)
	handle(err)

	parser, err := readability.New(string(bs), url, readability.LogLevel(-1))
	handle(err)

	res, err := parser.Parse()
	handle(err)

	if output == "html" {
		fmt.Print(res.Content)
	} else {
		fmt.Print(res.TextContent)
	}
}
