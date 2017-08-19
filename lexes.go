package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	
	"lexes/parser"

	"github.com/alexflint/go-arg"
)

type args struct {
	Input          string `arg:"help:File containing a search strategy."`
	Output         string `arg:"help:File to output the transformed query to."`
	Field         string `arg:"help:Fields to search on (defaults to plain_text)."`
	Operators	   bool `arg:"help:Insert operators between keywords (true)."`
	Retrieve	   []string `arg:"help:Fields to be returned in Elasticsearch result."`
	Highlight	   bool `arg:"help:Return highlight in es results (false)."`
}

func (args) Version() string {
	return "lexes 0.0.1"
}

func (args) Description() string {
	return "LexisNexis format boolean to elasticsearch query converter. By default, will read from stdin and output to stdout."
}

func main() {
	var args args
	var query string
	var err error
	inputFile := os.Stdin
	outputFile := os.Stdout
	
	args.Field = "plain_text"
	args.Retrieve = []string{"case_name", "date_filed"}
	args.Operators = true
	args.Highlight = false

	// Parse the args into the struct
	arg.MustParse(&args)

	// Grab the input file (if defaults to stdin).
	if args.Input != "" {
		query, err = Load(args.Input)
		if err != nil {
			log.Panicln(err)
		}
	} else {
		data, err := ioutil.ReadAll(inputFile)
		if err != nil {
			log.Panicln(err)
		}
		query = string(data)
	}

	js, err := parser.ParseJson(query, args.Field, args.Retrieve, args.Operators, args.Highlight)
	if err != nil {
		log.Panic(err)
	}

	// Write output of parser to file or output to screen
	if args.Output != "" {
		var err error
		outputFile, err = os.OpenFile(args.Output, os.O_WRONLY, 0)
		if os.IsNotExist(err) {
			outputFile, err = os.Create(args.Output)
		}

		if err != nil {
			log.Panicln(err)
		}
	} 

	outputFile.Write(js)
	fmt.Println("\n")
}

func Load(filename string) (string, error) {
	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(f), nil
}
