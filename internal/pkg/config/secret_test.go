// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package config_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/internal/pkg/config"
)

// credentialName matches the field names that carry a credential. "key" on its own is left out,
// because the config is full of key files and key sources that are paths rather than values.
var credentialName = regexp.MustCompile(`(?i)password|passphrase|secret|token|apikey`)

// publishedValues are the fields that match credentialName and are still written out, each with the
// reason. Anything else that matches has to be a config.Secret.
var publishedValues = map[string]string{
	"account.posthog.apiKey":                   "the PostHog project key, served to the browser",
	"account.userPilot.appToken":               "the UserPilot application token, served to the browser",
	"registries.factories.primary.tokenFile":   "a path, not the token",
	"registries.factories.secondary.tokenFile": "a path, not the token",
	"services.siderolink.joinTokensMode":       "the name of a mode, not a token",
}

// TestConfigIsSafeToMarshal fills every string field in the config with a value unique to its path,
// writes the config out the way the logger does, and checks that no credential comes with it.
func TestConfigIsSafeToMarshal(t *testing.T) {
	var params config.Params

	paths := fill(reflect.ValueOf(&params).Elem(), "")
	require.NotEmpty(t, paths)

	out, err := json.Marshal(params)
	require.NoError(t, err)

	written := string(out)

	for path, value := range paths {
		// quoted, so that one path being a prefix of another cannot hide a leak
		value = strconv.Quote(value)
		reason, published := publishedValues[path]

		switch {
		case !credentialName.MatchString(lastSegment(path)):
			assert.Contains(t, written, value, "%s is missing from the config that was written out", path)
		case published:
			assert.Contains(t, written, value, "%s is listed as published (%s) but was not written out", path, reason)
		default:
			assert.NotContains(t, written, value, "%s is written out in clear, it needs to be a config.Secret", path)
		}
	}
}

func lastSegment(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		return path[i+1:]
	}

	return path
}

// fill sets every string field reachable from v to a value unique to its path, and returns them by
// path. Strings inside slices and maps are left out, the config holds none that carry a credential.
func fill(v reflect.Value, path string) map[string]string {
	filled := map[string]string{}

	switch v.Kind() { //nolint:exhaustive // the config holds pointers, structs and strings, the rest carries nothing to check
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}

		return fill(v.Elem(), path)
	case reflect.Struct:
		for i := range v.NumField() {
			field := v.Type().Field(i)
			if !field.IsExported() {
				continue
			}

			name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
			if name == "" || name == "-" {
				name = field.Name
			}

			maps.Copy(filled, fill(v.Field(i), join(path, name)))
		}
	case reflect.String:
		value := fmt.Sprintf("value-of-%s", path)

		v.SetString(value)

		filled[path] = value
	default:
	}

	return filled
}

func join(path, name string) string {
	if path == "" {
		return name
	}

	return path + "." + name
}
