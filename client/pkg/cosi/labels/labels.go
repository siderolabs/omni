// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package labels implements label selector parser.
package labels

import (
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
