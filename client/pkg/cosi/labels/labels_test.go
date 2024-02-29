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
		expected *resource.LabelQuery
		query    string
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
			query: "a >= 㰀㰀",
			expected: &resource.LabelQuery{
				Terms: []resource.LabelTerm{
					{
						Key:    "a",
						Op:     resource.LabelOpLTNumeric,
						Value:  []string{"㰀㰀"},
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
	} {
		t.Run(tt.query, func(t *testing.T) {
			require := require.New(t)

			res, err := labels.ParseQuery(tt.query)

			if tt.expected == nil {
				require.Error(err)

				return
			}

			require.NoError(err)
			require.Equal(tt.expected, res)
		})
	}
}
