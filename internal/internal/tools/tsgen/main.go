// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build tools

package main

import (
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func main() {
	out, dirsToParse, tags, err := getParams()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = run(dirsToParse, tags, out)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(dirsToParse []string, tags string, out string) error {
	var result []ConstantWithDirective

	for _, dirToParse := range dirsToParse {
		fmt.Println("Parsing directory:", dirToParse)

		files, err := ParseRecursive(dirToParse, tags)
		if err != nil {
			return fmt.Errorf("failed to parse directory: %w", err)
		}

		fmt.Printf("Parsed %d files\n", len(files))

		for _, file := range files {
			filtered, err := FindTSGenConstants(file)
			if err != nil {
				return err
			}
			result = append(result, filtered...)
		}

		fmt.Printf("Found %d constants with \"tsgen:\" directive\nSaving to %s\n", len(result), out)
	}

	return SaveConstantsToFile(out, result)
}

func createFoldersForFile(out string) error {
	dir := filepath.Dir(out)
	return os.MkdirAll(dir, os.ModePerm)
}

// getParams parses the command line parameters and returns the output file, the directory to parse and an error
func getParams() (out string, dirsToParse []string, tags string, _ error) {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s -out <output file> <input dir>\n", os.Args[0])
	}
	flag.StringVar(&out, "out", "", "output file")
	flag.StringVar(&tags, "tags", "", "build tags")
	flag.Parse()

	if out == "" {
		return "", nil, "", fmt.Errorf("-out is required")
	}

	dirs := flag.Arg(0)
	if dirs == "" {
		return "", nil, "", fmt.Errorf("input dirs is required")
	}

	dirsToParse = strings.Split(dirs, ",")

	out, err := filepath.Abs(out)
	if err != nil {
		return "", nil, "", err
	}

	return out, dirsToParse, tags, err
}

// fillTemplate fills the provided Template with the given data.
func fillTemplate(t *template.Template, data any) (string, error) {
	var buf strings.Builder
	err := t.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// MakeTemplate creates a new named template from the given string. It panics on error.
func MakeTemplate(name, s string) *template.Template {
	return template.Must(template.New(name).Parse(s))
}

// Tpl is template for generate TS file which consumes slices of ConstantWithDirective.
//
//go:embed ts.template
var Tpl string

// SaveConstantsToFile saves the given constants to the given file.
func SaveConstantsToFile(file string, data any) error {
	t := MakeTemplate("ts_gen", Tpl)
	s, err := fillTemplate(t, data)
	if err != nil {
		return err
	}

	return WriteFile(file, s)
}

// WriteFile writes the given string to the given file.
func WriteFile(file string, s string) error {
	if err := createFoldersForFile(file); err != nil {
		return err
	}

	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(s)
	if err != nil {
		return err
	}

	return nil
}
