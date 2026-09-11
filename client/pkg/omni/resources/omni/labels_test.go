// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package omni_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

func TestValidateLabel(t *testing.T) {
	for _, tt := range []struct {
		name      string
		key       string
		value     string
		errString string
	}{
		{
			name:  "key and value",
			key:   "env",
			value: "prod",
		},
		{
			name:  "empty value",
			key:   "marked",
			value: "",
		},
		{
			// the state layer only reserves the prefix for its own use, it does not reject it:
			// the callers which must not accept it check for it themselves
			name:  "system prefixed key",
			key:   omni.SystemLabelPrefix + "cluster",
			value: "c1",
		},
		{
			name:      "empty key",
			key:       "",
			value:     "v",
			errString: "label key must not be empty",
		},
		{
			name:      "key too long",
			key:       strings.Repeat("k", omni.MaxLabelKeyLength+1),
			value:     "v",
			errString: "label key is too long",
		},
		{
			name:      "value too long",
			key:       "env",
			value:     strings.Repeat("v", omni.MaxLabelValueLength+1),
			errString: "label value for key \"env\" is too long",
		},
		{
			name:      "control character in key",
			key:       "en\x00v",
			value:     "prod",
			errString: "must not contain control characters",
		},
		{
			name:      "control character in value",
			key:       "env",
			value:     "pr\x00od",
			errString: "must not contain control characters",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := omni.ValidateLabel(tt.key, tt.value)

			if tt.errString == "" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errString)
		})
	}
}

// TestValidateAnnotation checks that the annotations share the label rule and only differ in how
// the errors name the entry.
func TestValidateAnnotation(t *testing.T) {
	require.NoError(t, omni.ValidateAnnotation("env", "prod"))

	err := omni.ValidateAnnotation("", "v")

	require.Error(t, err)
	require.Contains(t, err.Error(), "annotation key must not be empty")
}
