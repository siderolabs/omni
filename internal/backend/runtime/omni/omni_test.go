// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/omni/client/api/omni/specs"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/runtime"
	omniruntime "github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/backend/workloadproxy"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/ctxstore"
)

// using whitelisted for external API access type.
const testResourceType = omni.ClusterType

type testResource = typed.Resource[testResourceSpec, testResourceExtension]

type testResourceSpec = protobuf.ResourceSpec[specs.AuthConfigSpec, *specs.AuthConfigSpec]

type testResourceExtension struct{}

func (testResourceExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             testResourceType,
		Aliases:          []resource.Type{},
		DefaultNamespace: "default",
		PrintColumns:     []meta.PrintColumn{},
	}
}

// NewtestResource creates new StrResource.
func newTestResource(ns resource.Namespace, id resource.ID, spec *specs.AuthConfigSpec) *testResource {
	return typed.NewResource[testResourceSpec, testResourceExtension](
		resource.NewMetadata(ns, testResourceType, id, resource.VersionUndefined),
		protobuf.NewResourceSpec(spec),
	)
}

type OmniRuntimeSuite struct {
	suite.Suite
	runtime   *omniruntime.Runtime
	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
	eg        errgroup.Group
}

func (suite *OmniRuntimeSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	// disable auth in the context
	suite.ctx = ctxstore.WithValue(suite.ctx, auth.EnabledAuthContextKey{Enabled: false})

	var err error

	resourceState := state.WrapCore(namespaced.NewState(inmem.Build))
	logger := zaptest.NewLogger(suite.T())
	clientFactory := talos.NewClientFactory(resourceState, logger)
	dnsService := dns.NewService(resourceState, logger)
	discoveryServiceClient := &discoveryClientMock{}
	workloadProxyReconciler := workloadproxy.NewReconciler(logger, zapcore.InfoLevel)

	suite.runtime, err = omniruntime.New(clientFactory, dnsService, workloadProxyReconciler, nil, nil, nil, nil,
		resourceState, nil, prometheus.NewRegistry(), discoveryServiceClient, nil, logger)

	suite.Require().NoError(err)

	suite.startRuntime()
}

func (suite *OmniRuntimeSuite) startRuntime() {
	suite.runtime.Run(actor.MarkContextAsInternalActor(suite.ctx), &suite.eg)
}

func (suite *OmniRuntimeSuite) TestCrud() {
	id := "test"
	namespace := "test"

	testRes := newTestResource(namespace, id, &specs.AuthConfigSpec{
		Auth0: &specs.AuthConfigSpec_Auth0{Enabled: true, Domain: "test"},
	})

	testRes.Metadata().Labels().Set("label1", "")
	testRes.Metadata().Labels().Set("label2", "something")

	suite.Require().NoError(suite.runtime.Create(suite.ctx, testRes))

	for _, tt := range []struct {
		labels      []string
		expectedLen int
	}{
		{
			[]string{"label1", "label2"},
			1,
		},
		{
			[]string{"!label1"},
			0,
		},
		{
			[]string{"!label3"},
			1,
		},
		{
			[]string{"label2=nope"},
			0,
		},
		{
			[]string{"label2=something"},
			1,
		},
	} {
		list, err := suite.runtime.List(suite.ctx,
			runtime.WithResource(testResourceType),
			runtime.WithNamespace(namespace),
			runtime.WithLabelSelectors(tt.labels...),
		)
		suite.Require().NoError(err)
		suite.Require().Len(list.Items, tt.expectedLen)
	}

	getResource := func() *runtime.Resource {
		resp, err := suite.runtime.Get(suite.ctx, runtime.WithName(id), runtime.WithNamespace(namespace), runtime.WithResource(testResourceType))
		suite.Require().NoError(err)

		resourceResponse, ok := resp.(*runtime.Resource)
		suite.Require().True(ok)

		return resourceResponse
	}

	r := getResource()

	spec, ok := r.Spec.(*specs.AuthConfigSpec)
	suite.Require().True(ok)

	suite.Require().True(spec.Auth0.Enabled)
	suite.Require().Equal("test", spec.Auth0.Domain)

	testRes = newTestResource(namespace, id, &specs.AuthConfigSpec{
		Auth0: &specs.AuthConfigSpec_Auth0{Enabled: true, Domain: "test2"},
	})

	suite.Require().NoError(suite.runtime.Update(suite.ctx, testRes))

	r = getResource()

	spec, ok = r.Spec.(*specs.AuthConfigSpec)
	suite.Require().True(ok)

	suite.Require().Equal("test2", spec.Auth0.Domain)

	suite.Require().NoError(suite.runtime.Delete(suite.ctx, runtime.WithName(id), runtime.WithNamespace(namespace), runtime.WithResource(testResourceType)))

	_, err := suite.runtime.Get(suite.ctx, runtime.WithName(id), runtime.WithNamespace(id), runtime.WithResource(testResourceType))
	suite.Require().Error(err)
}

func (suite *OmniRuntimeSuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.Require().NoError(suite.eg.Wait())
}

func TestOmniRuntimeSuite(t *testing.T) {
	suite.Run(t, new(OmniRuntimeSuite))
}

type discoveryClientMock struct{}

// AffiliateDelete implements the omni.DiscoveryClient interface.
func (d *discoveryClientMock) AffiliateDelete(context.Context, string, string) error {
	return nil
}
