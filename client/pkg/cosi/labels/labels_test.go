// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package labels_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/cosi/labels"
)

func TestParseQuery(t *testing.T) {
	for _, tt := range []struct {
		expected      *resource.LabelQuery
		expectedError string
		query         string
	}{
		{
			query: "!a",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:    "a",
						Op:     resource.LabelOpExists,
						Invert: true,
					},
				},
			},
		},
		{
			query: "a",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key: "a",
						Op:  resource.LabelOpExists,
					},
				},
			},
		},
		{
			query: "a != b",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:    "a",
						Op:     resource.LabelOpEqual,
						Value:  []string{"b"},
						Invert: true,
					},
				},
			},
		},
		{
			query: "a == b",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:   "a",
						Op:    resource.LabelOpEqual,
						Value: []string{"b"},
					},
				},
			},
		},
		{
			query: "a = b",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:   "a",
						Op:    resource.LabelOpEqual,
						Value: []string{"b"},
					},
				},
			},
		},
		{
			query: "a < b",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:   "a",
						Op:    resource.LabelOpLTNumeric,
						Value: []string{"b"},
					},
				},
			},
		},
		{
			query: "a <= b",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:   "a",
						Op:    resource.LabelOpLTENumeric,
						Value: []string{"b"},
					},
				},
			},
		},
		{
			query: "a > b",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:    "a",
						Op:     resource.LabelOpLTENumeric,
						Value:  []string{"b"},
						Invert: true,
					},
				},
			},
		},
		{
			query: "a >= 㰀㰀", //nolint:gosmopolitan
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:    "a",
						Op:     resource.LabelOpLTNumeric,
						Value:  []string{"㰀㰀"}, //nolint:gosmopolitan
						Invert: true,
					},
				},
			},
		},
		{
			query: "a >= b",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:    "a",
						Op:     resource.LabelOpLTNumeric,
						Value:  []string{"b"},
						Invert: true,
					},
				},
			},
		},
		{
			query: "a notin (a, b, c, d)",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:    "a",
						Op:     resource.LabelOpIn,
						Value:  []string{"a", "b", "c", "d"},
						Invert: true,
					},
				},
			},
		},
		{
			query: "a in (a, b, c, d)",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:   "a",
						Op:    resource.LabelOpIn,
						Value: []string{"a", "b", "c", "d"},
					},
				},
			},
		},
		{
			query: "a in a, b, c, d",
		},
		{
			query: "a in a",
		},
		{
			query: "!a= 1",
		},
		{
			query: "a, b > 0",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key: "a",
						Op:  resource.LabelOpExists,
					},
					{
						Key:    "b",
						Op:     resource.LabelOpLTENumeric,
						Value:  []string{"0"},
						Invert: true,
					},
				},
			},
		},
		{
			query: "b = a b, a in (a, b, c, d e)",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:   "b",
						Op:    resource.LabelOpEqual,
						Value: []string{"a b"},
					},
					{
						Key:   "a",
						Op:    resource.LabelOpIn,
						Value: []string{"a", "b", "c", "d e"},
					},
				},
			},
		},
		{
			query: "b = c d, a = a b, z, c = f e",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:   "b",
						Op:    resource.LabelOpEqual,
						Value: []string{"c d"},
					},
					{
						Key:   "a",
						Op:    resource.LabelOpEqual,
						Value: []string{"a b"},
					},
					{
						Key: "z",
						Op:  resource.LabelOpExists,
					},
					{
						Key:   "c",
						Op:    resource.LabelOpEqual,
						Value: []string{"f e"},
					},
				},
			},
		},
		{
			query: `a="b, c"`,
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:   "a",
						Op:    resource.LabelOpEqual,
						Value: []string{"b, c"},
					},
				},
			},
		},
		{
			query: `a="b, \"c\""`,
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:   "a",
						Op:    resource.LabelOpEqual,
						Value: []string{`b, "c"`},
					},
				},
			},
		},
		{
			query: `a="b, \\"`,
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:   "a",
						Op:    resource.LabelOpEqual,
						Value: []string{`b, \`},
					},
				},
			},
		},
		{
			query:         `a="b, c`,
			expectedError: "unterminated quoted string",
		},
		{
			query:         `a="b,\`,
			expectedError: "unterminated escape sequence",
		},
	} {
		t.Run(tt.query, func(t *testing.T) {
			require := require.New(t)

			res, err := labels.ParseQuery(tt.query)

			if tt.expected == nil {
				require.Error(err)

				if tt.expectedError != "" {
					require.ErrorContains(err, tt.expectedError)
				}

				return
			}

			require.NoError(err)
			require.Equal(tt.expected, res)
		})
	}
}

// interpretedTerm lists the parts of a label term ExactValue reads.
//
// The conversion stops compiling when resource.LabelTerm gains a field, so a new way to write a term
// cannot pass through unread. Work out what the new field changes about which resources a term selects,
// and make ExactValue still return the value the terms require, or an error when they require more than
// one. Only then add the field here.
type interpretedTerm struct {
	Key    string
	Value  []string
	Op     resource.LabelOp
	Invert bool
}

var _ = interpretedTerm(resource.LabelTerm{})

func TestExactValue(t *testing.T) {
	t.Parallel()

	const key = "example.com/cluster"

	term := func(value string, opts ...resource.TermOption) resource.LabelTerm {
		var query resource.LabelQuery

		resource.LabelEqual(key, value, opts...)(&query)

		return query.Terms[0]
	}

	for name, tt := range map[string]struct {
		expected string
		terms    []resource.LabelTerm
		fails    bool
	}{
		"no terms": {},
		"one value": {
			terms:    []resource.LabelTerm{term("cluster1")},
			expected: "cluster1",
		},
		"the same value twice": {
			terms:    []resource.LabelTerm{term("cluster1"), term("cluster1")},
			expected: "cluster1",
		},
		"a term for another label": {
			terms:    []resource.LabelTerm{{Key: "another", Op: resource.LabelOpEqual, Value: []string{"value"}}, term("cluster1")},
			expected: "cluster1",
		},
		"a term with another operator": {
			terms: []resource.LabelTerm{{Key: key, Op: resource.LabelOpExists}},
		},
		"an inverted value": {
			terms: []resource.LabelTerm{term("cluster1", resource.NotMatches)},
		},
		"a value with another one excluded": {
			terms:    []resource.LabelTerm{term("cluster1"), term("cluster2", resource.NotMatches)},
			expected: "cluster1",
		},
		"two different values": {
			terms: []resource.LabelTerm{term("cluster1"), term("cluster2")},
			fails: true,
		},
		"an empty value": {
			terms: []resource.LabelTerm{term("")},
			fails: true,
		},
		"a term without a value": {
			terms: []resource.LabelTerm{{Key: key, Op: resource.LabelOpEqual}},
			fails: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			value, err := labels.ExactValue(tt.terms, key)

			if tt.fails {
				require.Error(t, err)
				require.Empty(t, value)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, value)
		})
	}
}
