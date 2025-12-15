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
	"github.com/siderolabs/talos/pkg/machinery/proto"

	"github.com/siderolabs/omni/client/api/omni/resources"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
)

func (s *ResourceServer) Controllers(ctx context.Context, _ *resources.ControllersRequest) (*resources.ControllersResponse, error) {
	_, err := auth.CheckGRPC(ctx, auth.WithRole(role.Admin))
	if err != nil {
		return nil, err
	}

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
	_, err := auth.CheckGRPC(ctx, auth.WithRole(role.Admin))
	if err != nil {
		return nil, err
	}

	graph, err := s.runtime.GetDependencyGraph()
	if err != nil {
		return nil, err
	}

	filterControllers := xslices.ToSet(req.Controllers)
	filterResources := xslices.ToSet(req.Resources)

	res := &resources.DependencyGraphResponse{}

	nodesMap := map[string]string{}

	genNodeID := func(label string) {
		id := fmt.Sprintf("n_%s", label)

		nodesMap[label] = id
	}

	addNode := func(edge controller.DependencyEdge, t resources.DependencyGraphResponse_Node_Type) error {
		if len(req.Resources) != 0 && t == resources.DependencyGraphResponse_Node_RESOURCE {
			if _, ok := filterResources[edge.ResourceType]; !ok {
				return nil
			}
		}

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
			source   string
			target   string
			edgeType controller.DependencyEdgeType
		)

		switch edge.EdgeType {
		case controller.EdgeOutputExclusive:
			source = nodesMap[edge.ControllerName]
			target = nodesMap[edge.ResourceType]
			edgeType = edge.EdgeType
		case controller.EdgeOutputShared:
			source = nodesMap[edge.ControllerName]
			target = nodesMap[edge.ResourceType]
			edgeType = edge.EdgeType
		case controller.EdgeInputStrong, controller.EdgeInputQPrimary:
			source = nodesMap[edge.ResourceType]
			target = nodesMap[edge.ControllerName]
			edgeType = edge.EdgeType
		case controller.EdgeInputWeak, controller.EdgeInputQMapped:
			source = nodesMap[edge.ResourceType]
			target = nodesMap[edge.ControllerName]
			edgeType = edge.EdgeType
		case controller.EdgeInputDestroyReady, controller.EdgeInputQMappedDestroyReady:
			if req.ShowDestroyReady {
				source = nodesMap[edge.ResourceType]
				target = nodesMap[edge.ControllerName]
				edgeType = edge.EdgeType
			}
		}

		if source == "" || target == "" {
			return nil
		}

		res.Edges = append(res.Edges,
			&resources.DependencyGraphResponse_Edge{
				Id:       fmt.Sprintf("e_%s->%s", source, target),
				Source:   source,
				Target:   target,
				EdgeType: int32(edgeType),
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
		addController := true
		addResource := true

		if len(filterControllers) != 0 {
			_, addController = filterControllers[edge.ControllerName]
		}

		if len(filterResources) != 0 {
			_, addResource = filterResources[edge.ResourceType]
		}

		if addController {
			genNodeID(edge.ControllerName)
		}

		if addResource {
			genNodeID(edge.ResourceType)
		}
	}

	for _, edge := range graph.Edges {
		if err = addEdge(edge); err != nil {
			return nil, err
		}
	}

	return res, nil
}
