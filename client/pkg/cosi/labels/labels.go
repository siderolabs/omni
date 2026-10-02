// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package labels implements label selector parsing, and reading values back out of the parsed terms.
package labels

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ParseQuery creates resource.LabelQuery from the string formatted selector.
func ParseQuery(selector string) (*resource.LabelQuery, error) {
	return (&parser{
		l: &lexer{
			s: selector,
		},
	}).parse()
}

// ParseSelectors creates resource.LabelQuery from the string formatted selectors.
func ParseSelectors(selectors []string) (resource.LabelQueries, error) {
	res := make([]resource.LabelQuery, 0, len(selectors))

	for _, selector := range selectors {
		query, err := ParseQuery(selector)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		res = append(res, *query)
	}

	return res, nil
}

// ExactValue returns the value the terms require the label to have, and an empty string if they do not require one.
//
// Terms for other labels, and inverted terms, which exclude a value rather than requiring one, are ignored.
// Two terms requiring different values are an error, since the terms then require no value at all.
func ExactValue(terms []resource.LabelTerm, key string) (string, error) {
	var value string

	for _, term := range terms {
		// an inverted term narrows the result down, it never requires a value
		if term.Key != key || term.Op != resource.LabelOpEqual || term.Invert {
			continue
		}

		switch {
		case len(term.Value) == 0 || term.Value[0] == "":
			return "", fmt.Errorf("empty value for %q is not supported", key)
		case value != "" && value != term.Value[0]:
			return "", fmt.Errorf("multiple values for %q are not supported", key)
		}

		value = term.Value[0]
	}

	return value, nil
}
