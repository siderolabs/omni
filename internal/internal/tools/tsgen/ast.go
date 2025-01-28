// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build tools

package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"

	"golang.org/x/tools/go/packages"
)

type file struct {
	f   *ast.File
	pkg *packages.Package
}

// ParseRecursive parses the source code recursively and returns the AST slice.
func ParseRecursive(dir, tags string) ([]file, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		// TODO: Need to think about constants in test files. Maybe write type_string_test.go
		// in a separate pass? For later.
		Tests:      false,
		BuildFlags: []string{fmt.Sprintf("-tags=%s", tags)},
		Dir:        dir,
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}

	if len(pkgs) == 0 {
		return nil, fmt.Errorf("error: no packages found")
	}

	if len(pkgs[0].Errors) > 0 {
		return nil, fmt.Errorf("error loading packages: %s", pkgs[0].Errors)
	}

	var result []file

	for _, pkg := range pkgs {
		for _, f := range pkg.Syntax {
			result = append(result, file{f, pkg})
		}
	}

	return result, nil
}

// FindTSGenConstants returns the list of constants that have tsgen directive.
func FindTSGenConstants(f file) ([]ConstantWithDirective, error) {
	constants := FindConstants(f)

	var result []ConstantWithDirective
	for _, constant := range constants {
		for _, comment := range constant.Doc {
			if directive, ok := extractTsGenDirective(comment); ok {
				if directive == "" {
					return nil, fmt.Errorf("empty directive in %s", constant.Name)
				}
				result = append(result, ConstantWithDirective{
					Constant:  constant,
					Directive: directive,
				})
			}
		}
	}

	return result, nil
}

// tsGenMatcher is a regexp matcher for tsgen directive.
var tsGenMatcher = regexp.MustCompile(`tsgen:(\w*)`)

// extractTsGenDirective extracts the tsgen directive from the comment.
func extractTsGenDirective(comment string) (string, bool) {
	matches := tsGenMatcher.FindStringSubmatch(comment)
	switch {
	case len(matches) == 0:
		return "", false
	case len(matches) == 2:
		return matches[1], true
	default:
		return "", true
	}
}

// Constant represents a constant with a doc, name and a value.
type Constant struct {
	Doc   []string
	Name  string
	Value string
}

// ConstantWithDirective represents a Constant with a directive.
type ConstantWithDirective struct {
	Constant
	Directive string
}

// FindConstants returns a list of constants in the given file.
func FindConstants(file file) []Constant {
	var result []Constant
	ast.Inspect(file.f, func(n ast.Node) bool {
		if decl, ok := n.(*ast.GenDecl); ok {
			if decl.Tok == token.CONST {
				if decl.Lparen == token.NoPos {
					name := decl.Specs[0].(*ast.ValueSpec).Names[0]
					obj := file.pkg.TypesInfo.Defs[name].(*types.Const)
					value := obj.Val().String()

					result = append(result, Constant{
						Doc:   commentToStrings(decl.Doc),
						Name:  name.Name,
						Value: value,
					})

					return false
				}

				for _, spec := range decl.Specs {
					// multiple constants declared inside parentheses
					if valueSpec, ok := spec.(*ast.ValueSpec); ok {
						if len(valueSpec.Values) == 0 {
							// constant declared without a value, skip it
							continue
						}

						name := valueSpec.Names[0]
						obj := file.pkg.TypesInfo.Defs[name].(*types.Const)
						value := obj.Val().String()

						result = append(result, Constant{
							Doc:   commentToStrings(valueSpec.Doc),
							Name:  name.Name,
							Value: value,
						})
					}
				}
			}

			return false
		}
		return true
	})

	return result
}

// commentToStrings converts a list of comments to a list of strings.
func commentToStrings(doc *ast.CommentGroup) []string {
	if doc == nil {
		return nil
	}

	var result []string
	for _, c := range doc.List {
		result = append(result, c.Text)
	}

	return result
}
