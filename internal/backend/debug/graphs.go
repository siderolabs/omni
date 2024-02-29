// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build sidero.debug

package debug

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/emicklei/dot"

	"github.com/siderolabs/omni/internal/pkg/auth/actor"
)

func (handler *Handler) handleDependencyGraph(w http.ResponseWriter, r *http.Request, withResources bool) {
	if r.Method == http.MethodHead {
		return
	}

	var (
		graph *dot.Graph
		err   error
	)

	if withResources {
		graph, err = handler.generateControllerResourceGraph(r.Context())
	} else {
		graph, err = handler.generateControllerGraph()
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Internal server error: %s", err), http.StatusInternalServerError)

		return
	}

	graph.Write(w)
}

func (handler *Handler) generateControllerGraph() (*dot.Graph, error) {
	depGraph, err := handler.runtime.GetDependencyGraph()
	if err != nil {
		return nil, err
	}

	graph := dot.NewGraph(dot.Directed)

	for _, edge := range depGraph.Edges {
		graph.Node(edge.ControllerName).Box()

		graph.Node(edge.ResourceType).
			Attr("shape", "note").
			Attr("fillcolor", "azure2").
			Attr("style", "filled")
	}

	for _, edge := range depGraph.Edges {
		var idLabels []string

		if edge.ResourceID != "" {
			idLabels = append(idLabels, edge.ResourceID)
		}

		switch edge.EdgeType {
		case controller.EdgeOutputExclusive:
			graph.Edge(graph.Node(edge.ControllerName), graph.Node(edge.ResourceType)).Bold()
		case controller.EdgeOutputShared:
			graph.Edge(graph.Node(edge.ControllerName), graph.Node(edge.ResourceType)).Solid()
		case controller.EdgeInputStrong, controller.EdgeInputQPrimary:
			graph.Edge(graph.Node(edge.ResourceType), graph.Node(edge.ControllerName), idLabels...).Solid()
		case controller.EdgeInputWeak, controller.EdgeInputQMapped:
			graph.Edge(graph.Node(edge.ResourceType), graph.Node(edge.ControllerName), idLabels...).Dotted()
		case controller.EdgeInputDestroyReady:
			// don't show the DestroyReady inputs to reduce the visual clutter
		}
	}

	return graph, nil
}

func (handler *Handler) generateControllerResourceGraph(ctx context.Context) (*dot.Graph, error) {
	depGraph, err := handler.runtime.GetDependencyGraph()
	if err != nil {
		return nil, err
	}

	ctx = actor.MarkContextAsInternalActor(ctx)

	graph := dot.NewGraph(dot.Directed)

	resourceID := func(r resource.Resource) string {
		return fmt.Sprintf("%s/%s/%s", r.Metadata().Namespace(), r.Metadata().Type(), r.Metadata().ID())
	}

	resources := map[resource.Type][]resource.Resource{}

	for _, edge := range depGraph.Edges {
		if _, ok := resources[edge.ResourceType]; ok {
			continue
		}

		rd, err := safe.StateGet[*meta.ResourceDefinition](ctx, handler.state, resource.NewMetadata(meta.NamespaceName, meta.ResourceDefinitionType, strings.ToLower(edge.ResourceType), resource.VersionUndefined))
		if err != nil {
			return nil, err
		}

		items, err := handler.state.List(ctx, resource.NewMetadata(rd.TypedSpec().DefaultNamespace, edge.ResourceType, "", resource.VersionUndefined))
		if err != nil {
			return nil, err
		}

		for _, r := range items.Items {
			resources[edge.ResourceType] = append(resources[edge.ResourceType], r)
		}
	}

	for _, edge := range depGraph.Edges {
		graph.Node(edge.ControllerName).Box()
	}

	for resourceType, resourceList := range resources {
		cluster := graph.Subgraph(resourceType, dot.ClusterOption{})
		cluster.Attr("label", dot.HTML(fmt.Sprintf("<B>%s</B>", resourceType)))

		for _, resource := range resourceList {
			label := fmt.Sprintf("<B>%s</B>", resource.Metadata().ID())

			for k, v := range resource.Metadata().Labels().Raw() {
				label += fmt.Sprintf("<BR/><I>%s</I>: %s", k, v)
			}

			cluster.Node(resourceID(resource)).
				Attr("shape", "note").
				Attr("fillcolor", "azure2").
				Attr("style", "filled").
				Attr("label", dot.HTML(label))
		}
	}

	for _, edge := range depGraph.Edges {
		for _, resource := range resources[edge.ResourceType] {
			if edge.ResourceID != "" && resource.Metadata().ID() != edge.ResourceID {
				continue
			}

			if (edge.EdgeType == controller.EdgeOutputExclusive ||
				edge.EdgeType == controller.EdgeOutputShared) &&
				edge.ControllerName != resource.Metadata().Owner() {
				continue
			}

			switch edge.EdgeType {
			case controller.EdgeOutputExclusive:
				graph.Edge(graph.Node(edge.ControllerName), graph.Subgraph(edge.ResourceType).Node(resourceID(resource))).Solid()
			case controller.EdgeOutputShared:
				graph.Edge(graph.Node(edge.ControllerName), graph.Subgraph(edge.ResourceType).Node(resourceID(resource))).Solid()
			case controller.EdgeInputStrong, controller.EdgeInputQPrimary:
				graph.Edge(graph.Subgraph(edge.ResourceType).Node(resourceID(resource)), graph.Node(edge.ControllerName)).Solid()
			case controller.EdgeInputWeak, controller.EdgeInputQMapped:
				graph.Edge(graph.Subgraph(edge.ResourceType).Node(resourceID(resource)), graph.Node(edge.ControllerName)).Dotted()
			case controller.EdgeInputDestroyReady:
				// don't show the DestroyReady inputs to reduce the visual clutter
			}
		}
	}

	return graph, nil
}
