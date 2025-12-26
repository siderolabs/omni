// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package migration

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/kvutils"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
)

type res[T any, S protobuf.Spec[T]] interface {
	generic.ResourceWithRD
	TypedSpec() *protobuf.ResourceSpec[T, S]
}

// changeOwner creates a new resource, copies all labels, annotation, finalizers, phase, version
// and spec to it, then runs state.Update with the new res.
// It's not possible to update the owner on the resource in the Modify methods.
func changeOwner[T any, S protobuf.Spec[T], R res[T, S]](ctx context.Context, st state.State, r R, owner string) error {
	res, err := protobuf.CreateResource(r.ResourceDefinition().Type)
	if err != nil {
		return err
	}

	updated, ok := res.(R)
	if !ok {
		return fmt.Errorf("failed to convert %s to %s", res.Metadata().Type(), r.Metadata().Type())
	}

	*updated.Metadata() = resource.NewMetadata(r.Metadata().Namespace(), r.Metadata().Type(), r.Metadata().ID(), r.Metadata().Version())

	for _, fin := range *r.Metadata().Finalizers() {
		updated.Metadata().Finalizers().Add(fin)
	}

	updated.TypedSpec().Value = r.TypedSpec().Value

	updated.Metadata().SetPhase(r.Metadata().Phase())

	updated.Metadata().Labels().Do(func(temp kvutils.TempKV) {
		for key, value := range r.Metadata().Labels().Raw() {
			temp.Set(key, value)
		}
	})

	updated.Metadata().Annotations().Do(func(temp kvutils.TempKV) {
		for key, value := range r.Metadata().Annotations().Raw() {
			temp.Set(key, value)
		}
	})

	if err = updated.Metadata().SetOwner(owner); err != nil {
		return err
	}

	if err = st.Update(ctx, updated, state.WithUpdateOwner(r.Metadata().Owner()), state.WithExpectedPhaseAny()); err != nil {
		return err
	}

	return nil
}
