// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/pair"
	"github.com/siderolabs/gen/xslices"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// KubernetesHealthcheck is an advanced healthcheck definition: a Kubernetes Job that Omni runs to check the
// cluster while it gates an upgrade, plus how often Omni re-checks.
type KubernetesHealthcheck struct { //nolint:govet
	// Descriptors are the user descriptors to apply to the resource.
	Descriptors Descriptors `yaml:",inline"`

	// IDOverride overrides the generated resource ID. When set, the prefix is ignored.
	IDOverride string `yaml:"idOverride,omitempty"`

	// Name of the healthcheck, used to derive the resource ID.
	Name string `yaml:"name,omitempty"`

	// Job is the inline Kubernetes Job manifest Omni runs to perform the healthcheck. Mutually exclusive with File.
	Job *InlineContent `yaml:"job,omitempty"`

	// File is the path to a file whose contents are used as the Job manifest. Mutually exclusive with Job.
	File string `yaml:"file,omitempty"`

	// Interval is how often Omni re-checks while it is holding an upgrade. If not set, a default interval of
	// 30s is used.
	Interval time.Duration `yaml:"interval,omitempty"`
}

// KubernetesHealthcheckList is a list of advanced healthchecks.
type KubernetesHealthcheckList []KubernetesHealthcheck

// Validate the healthcheck.
func (h *KubernetesHealthcheck) Validate(opts ValidateOptions) error {
	var multiErr error

	if h.Name == "" && h.IDOverride == "" {
		multiErr = multierror.Append(multiErr, errors.New("either name or idOverride is required for a healthcheck"))
	}

	switch {
	case h.Job != nil && h.File != "":
		multiErr = multierror.Append(multiErr, fmt.Errorf("healthcheck %q: job and file are mutually exclusive", h.identifier()))
	case h.Job == nil && h.File == "":
		multiErr = multierror.Append(multiErr, fmt.Errorf("healthcheck %q: job or file is required", h.identifier()))
	case h.File != "":
		if _, err := opts.StatFile(h.File); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("healthcheck %q: failed to access %q: %w", h.identifier(), h.File, err))
		}
	case h.Job != nil:
		if _, err := h.Job.Bytes(); err != nil {
			multiErr = multierror.Append(multiErr, fmt.Errorf("healthcheck %q: invalid job manifest: %w", h.identifier(), err))
		}
	}

	if err := h.Descriptors.Validate(); err != nil {
		multiErr = multierror.Append(multiErr, err)
	}

	return multiErr
}

// Translate the healthcheck into an Omni resource.
//
// prefix is used to namespace the generated resource ID when IDOverride is not set.
func (h *KubernetesHealthcheck) Translate(ctx TranslateContext, prefix string, labels ...pair.Pair[string, string]) (*omni.KubernetesHealthcheck, error) {
	id := h.IDOverride
	if id == "" {
		id = fmt.Sprintf("%s-%s", prefix, h.Name)
	}

	var (
		job []byte
		err error
	)

	switch {
	case h.File != "":
		if job, err = ctx.ReadFile(h.File); err != nil {
			return nil, fmt.Errorf("healthcheck %q: failed to read %q: %w", h.identifier(), h.File, err)
		}
	case h.Job != nil:
		if job, err = h.Job.Bytes(); err != nil {
			return nil, fmt.Errorf("healthcheck %q: failed to marshal job manifest: %w", h.identifier(), err)
		}
	}

	res := omni.NewKubernetesHealthcheck(id)

	res.Metadata().Labels().Do(func(temp kvutils.TempKV) {
		for _, label := range labels {
			temp.Set(label.F1, label.F2)
		}
	})

	h.Descriptors.Apply(res)

	res.TypedSpec().Value.Job = string(job)

	if h.Interval > 0 {
		res.TypedSpec().Value.Interval = durationpb.New(h.Interval)
	}

	return res, nil
}

// identifier returns the user-facing identifier (name or idOverride) of the healthcheck for diagnostics.
func (h *KubernetesHealthcheck) identifier() string {
	if h.IDOverride != "" {
		return h.IDOverride
	}

	return h.Name
}

// Validate the list of healthchecks.
func (l KubernetesHealthcheckList) Validate(opts ValidateOptions) error {
	ids := make(map[string]struct{}, len(l))

	return errors.Join(xslices.Map(l, func(h KubernetesHealthcheck) error {
		if err := h.Validate(opts); err != nil {
			return err
		}

		key := h.IDOverride
		if key == "" {
			key = h.Name
		}

		if key == "" {
			return nil
		}

		if _, exists := ids[key]; exists {
			return fmt.Errorf("duplicate healthcheck %q", key)
		}

		ids[key] = struct{}{}

		return nil
	})...)
}

// Translate the list of healthchecks into resources.
func (l KubernetesHealthcheckList) Translate(ctx TranslateContext, prefix string, labels ...pair.Pair[string, string]) ([]resource.Resource, error) {
	resources := make([]resource.Resource, 0, len(l))

	for _, h := range l {
		res, err := h.Translate(ctx, prefix, labels...)
		if err != nil {
			return nil, err
		}

		resources = append(resources, res)
	}

	return resources, nil
}
