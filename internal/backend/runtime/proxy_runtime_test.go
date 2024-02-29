// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package runtime_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/omni/client/api/omni/resources"
	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	pkgruntime "github.com/siderolabs/omni/client/pkg/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/cosi"
	omni2 "github.com/siderolabs/omni/internal/backend/runtime/omni"
)

type runtimeMock struct {
	runtime.Runtime
	watch func(ctx context.Context, responses chan<- runtime.WatchResponse, option ...runtime.QueryOption) error
	list  func(ctx context.Context, opts ...runtime.QueryOption) (runtime.ListResult, error)
}

func (r *runtimeMock) Watch(ctx context.Context, responses chan<- runtime.WatchResponse, opts ...runtime.QueryOption) error {
	if r.watch == nil {
		return r.Runtime.Watch(ctx, responses, opts...)
	}

	return r.watch(ctx, responses, opts...)
}

func (r *runtimeMock) List(ctx context.Context, opts ...runtime.QueryOption) (runtime.ListResult, error) {
	if r.list == nil {
		return r.Runtime.List(ctx, opts...)
	}

	return r.list(ctx, opts...)
}

func TestProxyRuntime_Watch(t *testing.T) {
	t.Parallel()

	type args struct {
		msgs         []runtime.WatchResponse
		expectedMsgs []runtime.WatchResponse
		skip         int
		limit        int
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "limit 0, skip 0, with duplicates",
			args: args{
				limit:        0,
				skip:         0,
				msgs:         duplicate(produce(0, 3, makeResponse)),
				expectedMsgs: safeSort(duplicate(produce(0, 3, makeResponse)), "", false),
			},
		},
		{
			name: "limit 3, skip 0",
			args: args{
				limit:        3,
				skip:         0,
				msgs:         produce(0, 10, makeResponse),
				expectedMsgs: produce(0, 3, makeResponse),
			},
		},
		{
			name: "limit 0, skip 3",
			args: args{
				limit:        0,
				skip:         3,
				msgs:         produce(0, 10, makeResponse),
				expectedMsgs: produce(3, 7, makeResponse),
			},
		},
		{
			name: "limit 3, skip 3",
			args: args{
				limit:        3,
				skip:         3,
				msgs:         produce(0, 10, makeResponse),
				expectedMsgs: produce(3, 3, makeResponse),
			},
		},
		{
			name: "limit 6, skip 3, with duplicates",
			args: args{
				limit:        6,
				skip:         3,
				msgs:         duplicate(produce(0, 10, makeResponse)),
				expectedMsgs: safeSort(duplicate(produce(3, 6, makeResponse)), "", false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testWatch(
				t,
				tt.args.msgs,
				tt.args.expectedMsgs,
				false,
				runtime.WithLimit(tt.args.limit),
				runtime.WithOffset(tt.args.skip),
			)
		})
	}
}

func makeResponse(i int) runtime.WatchResponse {
	return pointer.To(runtime.NewBasicResponse(
		fmt.Sprintf("id-%d", i),
		fmt.Sprintf("msg-%d", i),
		&resources.WatchResponse{Event: &resources.Event{}},
	))
}

func duplicate[T any](v []T) []T { return append(v, v...) }

func produce[T any](start, count int, fn func(i int) T) []T {
	msgs := make([]T, 0, count)

	for i := 0; i < count; i++ {
		msgs = append(msgs, fn(i+start))
	}

	return msgs
}

func testWatch(t *testing.T, msgs, expectedMsgs []runtime.WatchResponse, compareTotal bool, opts ...runtime.QueryOption) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	mock := runtimeMock{
		watch: func(ctx context.Context, responses chan<- runtime.WatchResponse, _ ...runtime.QueryOption) error {
			for _, msg := range msgs {
				responses <- msg
			}

			<-ctx.Done()

			return nil
		},
	}

	msgs = appendBootstrapped(msgs)
	expectedMsgs = appendBootstrapped(expectedMsgs)

	proxy := &runtime.ProxyRuntime{Runtime: &mock}
	ch := make(chan runtime.WatchResponse)
	errCh := make(chan error)

	go func() {
		err := proxy.Watch(ctx, ch, opts...)
		if err != nil {
			t.Log("error from proxy.Watch:", err)
		}

		errCh <- err
	}()

	chResult, err := takeCount(ctx, ch, len(expectedMsgs))
	assert.NoError(t, err)

	cancel()

	if !compareSlices(chResult, expectedMsgs, func(a, b runtime.WatchResponse) bool {
		result := true

		if compareTotal {
			result = a.Unwrap().GetTotal() == b.Unwrap().GetTotal()
		}

		return result &&
			a.ID() == b.ID() &&
			a.Namespace() == b.Namespace() &&
			runtime.EventType(a) == runtime.EventType(b)
	}) {
		t.Helper()
		t.Log("got:", chResult)
		t.Log("expected:", expectedMsgs)
		t.FailNow()
	}

	require.NoError(t, <-errCh)
}

func appendBootstrapped(msgs []runtime.WatchResponse) []runtime.WatchResponse {
	if !slices.ContainsFunc(msgs, func(response runtime.WatchResponse) bool {
		return runtime.EventType(response) == resources.EventType_BOOTSTRAPPED
	}) {
		msgs = append(msgs, pointer.To(runtime.NewBasicResponse(
			"",
			"",
			&resources.WatchResponse{Event: &resources.Event{EventType: resources.EventType_BOOTSTRAPPED}},
		)))
	}

	return msgs
}

func takeCount[T any](ctx context.Context, ch <-chan T, count int) ([]T, error) {
	msgs := make([]T, 0, count)

	for i := 0; i < count; i++ {
		select {
		case got, ok := <-ch:
			if !ok {
				return nil, errors.New("channel closed early")
			}

			msgs = append(msgs, got)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return msgs, nil
}

func TestProxyRuntime_WatchContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mock := runtimeMock{
		watch: func(ctx context.Context, ch chan<- runtime.WatchResponse, _ ...runtime.QueryOption) error {
			select {
			case ch <- makeResponse(0):
			case <-ctx.Done():
			}

			<-ctx.Done()

			return ctx.Err()
		},
	}

	proxy := &runtime.ProxyRuntime{Runtime: &mock}
	ch := make(chan runtime.WatchResponse)

	errCh := make(chan error, 1)

	go func() {
		errCh <- proxy.Watch(ctx, ch)
	}()

	cancel()

	require.Equal(t, context.Canceled, <-errCh)
}

func compareSlices[T comparable](a, b []T, cmp func(T, T) bool) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !cmp(a[i], b[i]) {
			return false
		}
	}

	return true
}

func safeSort(msgs []runtime.WatchResponse, field string, desc bool) []runtime.WatchResponse {
	err := runtime.SortResponses(msgs, runtime.MakeWatchResponseComparator(field, desc))
	if err != nil {
		panic(err)
	}

	return msgs
}

func TestProxyRuntime_WatchWithWithSort(t *testing.T) {
	t.Parallel()

	produced := produce(0, 10, makeResponse)
	expected := reverse(produced)

	testWatch(t, produced, expected, false, runtime.WithSort("", true))
}

func reverse[T any](slc []T) []T {
	for i := len(slc)/2 - 1; i >= 0; i-- {
		opp := len(slc) - 1 - i
		slc[i], slc[opp] = slc[opp], slc[i]
	}

	return slc
}

func TestProxyRuntime_WatchWithSearchFor(t *testing.T) {
	t.Parallel()

	produced := produce(1, 10, makeResponse)
	expected := []runtime.WatchResponse{produced[0], produced[9], produced[4]}

	testWatch(t, produced, expected, false, runtime.WithSearchFor([]string{"this-should-not-match", "id-1", "id-5"}))
}

func TestProxyRuntime_List(t *testing.T) {
	t.Parallel()

	machines := toListResult([]*omni.MachineStatus{
		newMachine(1, &specs.MachineStatusSpec{Cluster: "cluster5"}),
		newMachine(2, &specs.MachineStatusSpec{Cluster: "cluster3"}),
		newMachine(3, &specs.MachineStatusSpec{Cluster: "cluster2"}),
		newMachine(4, &specs.MachineStatusSpec{Cluster: "cluster4"}),
		newMachine(5, &specs.MachineStatusSpec{Cluster: "cluster1"}),
		newMachine(6, &specs.MachineStatusSpec{Cluster: "cluster1"}),
	})

	expected := runtime.ListResult{
		Items: []pkgruntime.ListItem{
			machines.Items[4],
			machines.Items[5],
			machines.Items[2],
			machines.Items[1],
		},
		Total: 5,
	}

	testList(
		t,
		machines,
		expected,
		nil,
		runtime.WithSort("cluster", false),
		runtime.WithSearchFor([]string{"cluster1", "cluster2", "cluster3", "cluster4"}),
		runtime.WithLimit(4),
	)
}

func toListResult(machines []*omni.MachineStatus) runtime.ListResult {
	items := make([]pkgruntime.ListItem, 0, len(machines))

	for _, machine := range machines {
		items = append(items, omni2.NewItem(ensure.Value(runtime.NewResource(machine))))
	}

	return runtime.ListResult{
		Items: items,
		Total: len(items),
	}
}

func newMachine(i int, val *specs.MachineStatusSpec) *omni.MachineStatus {
	machine := omni.NewMachineStatus("default", fmt.Sprintf("id%d", i))

	machine.Metadata().SetVersion(ensure.Value(resource.ParseVersion("1")))

	machine.TypedSpec().Value = val

	return machine
}

func testList(t *testing.T, original, expected runtime.ListResult, expectedErr error, opts ...runtime.QueryOption) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mock := runtimeMock{
		list: func(context.Context, ...runtime.QueryOption) (runtime.ListResult, error) {
			return original, nil
		},
	}

	proxy := &runtime.ProxyRuntime{Runtime: &mock}

	res, err := proxy.List(ctx, opts...)
	if err != nil || expectedErr != nil {
		require.NotNil(t, expectedErr)
		require.EqualError(t, err, expectedErr.Error())

		return
	}

	require.Equal(t, expected.Total, res.Total)

	if !compareSlices(res.Items, expected.Items, func(a, b pkgruntime.ListItem) bool {
		require.Equal(t, a.ID(), b.ID())
		require.Equal(t, a.Namespace(), b.Namespace())
		require.Equal(t, a.Unwrap(), b.Unwrap())

		return true
	}) {
		t.Helper()
		t.Log("got:", res.Items)
		t.Log("expected:", expected.Items)
		t.FailNow()
	}
}

func TestProxyRuntime_ListError(t *testing.T) {
	t.Parallel()

	machines := toListResult([]*omni.MachineStatus{newMachine(1, &specs.MachineStatusSpec{Cluster: "cluster1"})})

	testList(
		t,
		machines,
		runtime.ListResult{},
		errors.New("failed to sort: field \"such-field-do-not-exist\" for element \"id1\" not found"),
		runtime.WithSort("such-field-do-not-exist", false),
	)
}

func TestProxyRuntime_WatchBootstrappedFirst(t *testing.T) {
	t.Parallel()

	msgs := []runtime.WatchResponse{
		cosi.NewResponse("", "", &resources.WatchResponse{
			Event: &resources.Event{
				EventType: resources.EventType_BOOTSTRAPPED,
			},
		}, nil),
		watchResponse(1, "cluster3", "cluster", 0),
		watchResponse(2, "cluster2", "cluster", 0),
		watchResponse(3, "cluster1", "cluster", 0),
		watchResponse(4, "cluster3", "cluster", 0),
		watchResponse(5, "cluster3", "cluster", 0),
		watchResponse(6, "cluster3", "cluster", 0),
		watchResponseDestroy(3, "cluster1", "cluster", 0),
	}

	expected := []runtime.WatchResponse{
		cosi.NewResponse("", "", &resources.WatchResponse{
			Event: &resources.Event{
				EventType: resources.EventType_BOOTSTRAPPED,
			},
		}, nil),
		watchResponse(3, "cluster1", "cluster", 2),
		watchResponse(4, "cluster3", "cluster", 3),
		watchResponseDestroy(3, "cluster1", "cluster", 4),
	}

	testWatch(
		t,
		msgs,
		expected,
		true,
		runtime.WithSort("cluster", false),
		runtime.WithOffset(1),
		runtime.WithLimit(2),
		runtime.WithSearchFor([]string{"cluster1", "cluster3"}),
	)
}

//nolint:unparam
func watchResponse(id int, cluster, sortByField string, count int) runtime.WatchResponse {
	return cosi.NewResponse(
		fmt.Sprintf("id%d", id),
		"default",
		&resources.WatchResponse{
			Event: &resources.Event{
				EventType: resources.EventType_CREATED,
			},
			Total:         int32(count),
			SortFieldData: sortByField,
		},
		newMachine(id, &specs.MachineStatusSpec{Cluster: cluster}),
	)
}

func watchResponseDestroy(id int, cluster, field string, count int) runtime.WatchResponse {
	return cosi.NewResponse(
		fmt.Sprintf("id%d", id),
		"default",
		&resources.WatchResponse{
			Event: &resources.Event{
				EventType: resources.EventType_DESTROYED,
			},
			Total:         int32(count),
			SortFieldData: field,
		},
		newMachine(id, &specs.MachineStatusSpec{Cluster: cluster}),
	)
}
