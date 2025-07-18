// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build sidero.debug

package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/omni/client/api/omni/resources"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (s *ResourceServer) Controllers(ctx context.Context, _ *emptypb.Empty) (*resources.ControllersResponse, error) {
	graph, err := s.runtime.GetDependencyGraph()
	if err != nil {
		return nil, err
	}

	resp := &resources.ControllersResponse{}

	visited := map[string]struct{}{}

	for _, edge := range graph.Edges {
		if _, ok := visited[edge.ControllerName]; ok {
			continue
		}

		resp.Controllers = append(resp.Controllers, edge.ControllerName)

		visited[edge.ControllerName] = struct{}{}
	}

	slices.Sort(resp.Controllers)

	return resp, nil
}

func (s *ResourceServer) DependencyGraph(ctx context.Context, req *resources.DependencyGraphRequest) (*resources.DependencyGraphResponse, error) {
	graph, err := s.runtime.GetDependencyGraph()
	if err != nil {
		return nil, err
	}

	filterControllers := xslices.ToSet(req.Controllers)

	res := &resources.DependencyGraphResponse{}

	nodesMap := map[string]string{}

	genNodeID := func(label string) {
		id := fmt.Sprintf("n_%s", label)

		nodesMap[label] = id
	}

	addNode := func(edge controller.DependencyEdge, t resources.DependencyGraphResponse_Node_Type) error {
		var label string

		switch t {
		case resources.DependencyGraphResponse_Node_RESOURCE:
			label = edge.ResourceType
		case resources.DependencyGraphResponse_Node_CONTROLLER:
			label = edge.ControllerName
		case resources.DependencyGraphResponse_Node_UNKNOWN:
			return errors.New("unknown node type")
		}

		id, ok := nodesMap[label]

		if !ok {
			return nil
		}

		var (
			labels []string
			fields []string
		)

		if t == resources.DependencyGraphResponse_Node_RESOURCE {
			ctx := actor.MarkContextAsInternalActor(ctx)

			var resources []resource.Resource

			if edge.ResourceID != "" {
				res, err := s.state.Get(ctx, resource.NewMetadata(edge.ResourceNamespace, edge.ResourceType, edge.ResourceID, resource.VersionUndefined))
				if err != nil && !state.IsNotFoundError(err) {
					return err
				}

				if err == nil {
					resources = append(resources, res)
				}
			} else {
				response, err := s.state.List(ctx, resource.NewMetadata(edge.ResourceNamespace, edge.ResourceType, "", resource.VersionUndefined))
				if err != nil {
					return err
				}

				resources = response.Items
			}

			labelsMap := map[string]struct{}{}

			for _, res := range resources {
				for _, key := range res.Metadata().Labels().Keys() {
					labelsMap[key] = struct{}{}
				}
			}

			labels = slices.Sorted(maps.Keys(labelsMap))

			res, err := protobuf.CreateResource(edge.ResourceType)
			if err != nil {
				return err
			}

			if reflect.ValueOf(res.Spec()).Kind() == reflect.Ptr {
				err = json.Unmarshal([]byte("{}"), res.Spec())
				if err != nil {
					return err
				}

				type protobufWrapper interface {
					GetValue() proto.Message
				}

				if message, ok := res.Spec().(protobufWrapper); ok {
					v := reflect.TypeOf(message.GetValue()).Elem()

					for i := range v.NumField() {
						field := v.Field(i)

						f := field.Tag.Get("json")
						if f == "" {
							continue
						}

						fields = append(fields, strings.Split(f, ",")[0])
					}
				}
			}
		}

		res.Nodes = append(res.Nodes, &resources.DependencyGraphResponse_Node{
			Id:     id,
			Label:  label,
			Type:   t,
			Labels: labels,
			Fields: fields,
		})

		return nil
	}

	addEdge := func(edge controller.DependencyEdge) error {
		var (
			source string
			target string
			style  string
		)

		switch edge.EdgeType {
		case controller.EdgeOutputExclusive:
			source = nodesMap[edge.ControllerName]
			target = nodesMap[edge.ResourceType]
			style = "bold"
		case controller.EdgeOutputShared:
			source = nodesMap[edge.ControllerName]
			target = nodesMap[edge.ResourceType]
			style = "solid"
		case controller.EdgeInputStrong, controller.EdgeInputQPrimary:
			source = nodesMap[edge.ResourceType]
			target = nodesMap[edge.ControllerName]
			style = "solid"
		case controller.EdgeInputWeak, controller.EdgeInputQMapped:
			source = nodesMap[edge.ResourceType]
			target = nodesMap[edge.ControllerName]
			style = "dotted"
		case controller.EdgeInputDestroyReady:
			// don't show the DestroyReady inputs to reduce the visual clutter
		}

		if source == "" || target == "" {
			return nil
		}

		res.Edges = append(res.Edges,
			&resources.DependencyGraphResponse_Edge{
				Id:     fmt.Sprintf("e_%s->%s", source, target),
				Source: source,
				Target: target,
				Style:  style,
			},
		)

		if err = addNode(edge, resources.DependencyGraphResponse_Node_CONTROLLER); err != nil {
			return err
		}

		if err = addNode(edge, resources.DependencyGraphResponse_Node_RESOURCE); err != nil {
			return err
		}

		return nil
	}

	for _, edge := range graph.Edges {
		if len(filterControllers) != 0 {
			if _, ok := filterControllers[edge.ControllerName]; !ok {
				continue
			}
		}

		genNodeID(edge.ControllerName)
		genNodeID(edge.ResourceType)
	}

	for _, edge := range graph.Edges {
		if err = addEdge(edge); err != nil {
			return nil, err
		}
	}

	return res, nil
}
