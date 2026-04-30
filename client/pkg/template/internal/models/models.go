// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package models provides cluster template models (for each sub-document of multi-doc YAML).
package models

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/pair"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// Meta is embedded into all template objects.
type Meta struct {
	Kind string `yaml:"kind"`
}

// FileContext describes how to resolve and read files referenced by a template.
type FileContext struct {
	// Root restricts file access to a single directory tree. When nil, no restriction is applied.
	Root *os.Root
	// Dir is the directory used to resolve relative file paths from the template.
	// When empty, callers should treat it as "." (for example, for non-file-backed templates).
	Dir string
}

// TranslateContext is a context for translation.
type TranslateContext struct {
	FileContext
	LockedMachines            map[MachineID]struct{}
	MachineDescriptors        map[MachineID]Descriptors
	MachineSetLevelKernelArgs map[MachineID]KernelArgs
	ClusterLevelKernelArgs    KernelArgs
	ClusterName               string
}

// SystemExtensions is embedded in Cluster, MachineSet and Machine objects.
type SystemExtensions struct {
	SystemExtensions []string `yaml:"systemExtensions,omitempty"`
}

func (s *SystemExtensions) translate(ctx TranslateContext, nameSuffix string, labels ...pair.Pair[string, string]) []resource.Resource {
	if len(s.SystemExtensions) == 0 {
		return nil
	}

	configuration := omni.NewExtensionsConfiguration(fmt.Sprintf("schematic-%s", nameSuffix))

	configuration.Metadata().Labels().Set(omni.LabelCluster, ctx.ClusterName)

	configuration.Metadata().Labels().Do(func(temp kvutils.TempKV) {
		for _, l := range labels {
			temp.Set(l.F1, l.F2)
		}
	})

	configuration.TypedSpec().Value.Extensions = s.SystemExtensions

	return []resource.Resource{
		configuration,
	}
}

// Descriptors are the user descriptors (i.e. Labels, Annotations) to apply to the resource.
type Descriptors struct {
	// Labels are the user labels to apply to the resource.
	Labels map[string]string `yaml:"labels,omitempty"`

	// Annotations are the user annotations to apply to the resource.
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// Validate validates the descriptors.
func (d *Descriptors) Validate() error {
	var multiErr error

	for labelKey := range d.Labels {
		if strings.HasPrefix(labelKey, omni.SystemLabelPrefix) {
			multiErr = multierror.Append(multiErr, fmt.Errorf("label %q is invalid: prefix %q is reserved for internal use", labelKey, omni.SystemLabelPrefix))
		}
	}

	for annotationKey := range d.Annotations {
		if strings.HasPrefix(annotationKey, omni.SystemLabelPrefix) {
			multiErr = multierror.Append(multiErr, fmt.Errorf("annotation %q is invalid: prefix %q is reserved for internal use", annotationKey, omni.SystemLabelPrefix))
		}
	}

	return multiErr
}

// Apply applies the descriptors to the given resource.
func (d *Descriptors) Apply(res resource.Resource) {
	for k, v := range d.Labels {
		res.Metadata().Labels().Set(k, v)
	}

	for k, v := range d.Annotations {
		res.Metadata().Annotations().Set(k, v)
	}
}

// ValidateOptions contains options for model validation.
type ValidateOptions struct {
	FileContext
}

// resolveForDir resolves path against fc.Dir when path is relative.
func (fc FileContext) resolveForDir(path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(fc.Dir, path)
}

// resolveForRoot translates a path into a path relative to fc.Root,
// resolving relative paths against fc.Dir.
func (fc FileContext) resolveForRoot(path string) (string, error) {
	absPath, err := filepath.Abs(fc.resolveForDir(path))
	if err != nil {
		return "", err
	}

	rootAbs, err := filepath.Abs(fc.Root.Name())
	if err != nil {
		return "", err
	}

	return filepath.Rel(rootAbs, absPath)
}

// ReadFile reads a file, using fc.Root to restrict access when non-nil.
// Relative paths are resolved against fc.Dir.
func (fc FileContext) ReadFile(path string) ([]byte, error) {
	if fc.Root == nil {
		return os.ReadFile(fc.resolveForDir(path))
	}

	rel, err := fc.resolveForRoot(path)
	if err != nil {
		return nil, err
	}

	return fc.Root.ReadFile(rel)
}

// StatFile stats a file, using fc.Root to restrict access when non-nil.
// Relative paths are resolved against fc.Dir.
func (fc FileContext) StatFile(path string) (os.FileInfo, error) {
	if fc.Root == nil {
		return os.Stat(fc.resolveForDir(path))
	}

	rel, err := fc.resolveForRoot(path)
	if err != nil {
		return nil, err
	}

	return fc.Root.Stat(rel)
}

// Model is a base interface for cluster templates.
type Model interface {
	Validate(ValidateOptions) error
	Translate(TranslateContext) ([]resource.Resource, error)
}

var registeredModels = map[string]func() Model{}

type model[T any] interface {
	*T
	Model
}

func register[T any, P model[T]](kind string) {
	if _, ok := registeredModels[kind]; ok {
		panic(fmt.Sprintf("model %s already registered", kind))
	}

	registeredModels[kind] = func() Model {
		return P(new(T))
	}
}

// New creates a model by kind.
func New(kind string) (Model, error) {
	f, ok := registeredModels[kind]
	if !ok {
		return nil, fmt.Errorf("unknown model kind %q", kind)
	}

	return f(), nil
}
