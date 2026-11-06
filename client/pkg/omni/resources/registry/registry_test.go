// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package registry_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/k8s"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/oidc"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/omni/resources/registry"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/siderolink"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/system"
	_ "github.com/siderolabs/omni/client/pkg/omni/resources/virtual"
)

// TestAllResourceTypesRegistered fails when a resource type is defined in the resources packages without being
// registered, as such a type gets no resource definition and cannot be decoded by clients.
func TestAllResourceTypesRegistered(t *testing.T) {
	t.Parallel()

	registered := make(map[string]struct{}, len(registry.Resources))

	for _, r := range registry.Resources {
		registered[r.ResourceDefinition().Type] = struct{}{}
	}

	defined := definedResourceTypes(t, "..")
	require.NotEmpty(t, defined)

	for typ, pos := range defined {
		assert.Contains(t, registered, typ, "resource type %q defined at %s is not registered", typ, pos)
	}
}

// definedResourceTypes returns the resource types declared as resource.Type("...") constants or variables under root.
func definedResourceTypes(t *testing.T, root string) map[string]string {
	t.Helper()

	fset := token.NewFileSet()
	types := map[string]string{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}

		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.ValueSpec)
			if !ok {
				return true
			}

			for _, value := range spec.Values {
				if typ, ok := resourceTypeLiteral(value); ok {
					types[typ] = fset.Position(value.Pos()).String()
				}
			}

			return true
		})

		return nil
	})
	require.NoError(t, err)

	return types
}

// resourceTypeLiteral returns the string of a resource.Type("...") call expression.
func resourceTypeLiteral(expr ast.Expr) (string, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return "", false
	}

	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Type" {
		return "", false
	}

	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "resource" {
		return "", false
	}

	lit, ok := call.Args[0].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}

	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}

	return value, true
}
