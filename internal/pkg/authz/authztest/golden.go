// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package authztest

import (
	"bytes"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata")

// CompareGoldenFiles fails when the files differ from the golden files in the directory. With the -update flag it rewrites them.
//
// The content is compared and not the bytes, so the formatting of a golden file does not matter.
func CompareGoldenFiles(t *testing.T, dir string, files map[string][]byte) {
	if *update {
		require.NoError(t, os.RemoveAll(dir))
		require.NoError(t, os.MkdirAll(dir, 0o755))

		for name, content := range files {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), content, 0o644))
		}

		return
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err, "golden files are missing, create them with -update")

	// a file which no subject writes anymore fails the test too
	for _, entry := range entries {
		require.Contains(t, files, entry.Name(), "golden file %s has no subject, rewrite the golden files with -update", entry.Name())
	}

	for _, name := range slices.Sorted(maps.Keys(files)) {
		golden, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err, "golden file %s is missing, create it with -update", name)

		var expected, actual any

		require.NoError(t, yaml.Unmarshal(golden, &expected), "golden file %s", name)
		require.NoError(t, yaml.Unmarshal(files[name], &actual))

		require.Equal(t, expected, actual, "authorization changed in %s, review the diff and rewrite the golden files with -update", name)
	}
}

// Outcome returns what an access check decided, as it is written in the golden files.
//
// It fails the test on any other error, as the check did not decide then.
func Outcome(t *testing.T, err error) string {
	switch status.Code(err) { //nolint:exhaustive
	case codes.OK:
		return "allowed"
	case codes.Unauthenticated:
		return "unauthenticated"
	case codes.PermissionDenied:
		return "denied"
	default:
		require.NoError(t, err, "the access check failed without a decision")

		return ""
	}
}

// Outcomes are the lines of a golden file, in the order they are added.
type Outcomes struct {
	lines []*yaml.Node
}

// Add adds the outcome of a call.
func (o *Outcomes) Add(call, outcome string) {
	o.lines = append(o.lines, &yaml.Node{Kind: yaml.ScalarNode, Value: call}, &yaml.Node{Kind: yaml.ScalarNode, Value: outcome})
}

// AddGroup adds the outcomes of the calls which belong together, under the name of the group.
func (o *Outcomes) AddGroup(name string, group *Outcomes) {
	o.lines = append(o.lines, &yaml.Node{Kind: yaml.ScalarNode, Value: name}, &yaml.Node{Kind: yaml.MappingNode, Content: group.lines})
}

// CompareGolden records the outcomes of each subject and compares them with the golden files in testdata/authorization,
// one file for each subject.
//
// The header is the comment at the top of each file. It takes the name and the description of the subject.
func CompareGolden(t *testing.T, header string, record func(subject Subject, outcomes *Outcomes)) {
	files := make(map[string][]byte, len(Subjects))

	for _, subject := range Subjects {
		var outcomes Outcomes

		record(subject, &outcomes)

		node := &yaml.Node{Kind: yaml.MappingNode, Content: outcomes.lines, HeadComment: fmt.Sprintf(header, subject.Name, subject.Description)}

		var buf bytes.Buffer

		encoder := yaml.NewEncoder(&buf)
		encoder.SetIndent(2)

		require.NoError(t, encoder.Encode(node))
		require.NoError(t, encoder.Close())

		files[subject.Name+".yaml"] = buf.Bytes()
	}

	CompareGoldenFiles(t, filepath.Join("testdata", "authorization"), files)
}
