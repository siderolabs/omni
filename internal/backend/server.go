// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

// Package backend contains all internal backend code.
package backend

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	pgpcrypto "github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/cosi-project/runtime/api/v1alpha1"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	protobufserver "github.com/cosi-project/runtime/pkg/state/protobuf/server"
	"github.com/crewjam/saml/samlsp"
	"github.com/fsnotify/fsnotify"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	service "github.com/siderolabs/discovery-service/pkg/service"
	"github.com/siderolabs/go-api-signature/pkg/pgp"
	"github.com/siderolabs/go-api-signature/pkg/serviceaccount"
	"github.com/siderolabs/go-retry/retry"
	talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	resapi "github.com/siderolabs/omni/client/api/omni/resources"
	"github.com/siderolabs/omni/client/pkg/access"
	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	authres "github.com/siderolabs/omni/client/pkg/omni/resources/auth"
	omnires "github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	"github.com/siderolabs/omni/client/pkg/panichandler"
	"github.com/siderolabs/omni/internal/backend/debug"
	"github.com/siderolabs/omni/internal/backend/dns"
	"github.com/siderolabs/omni/internal/backend/factory"
	grpcomni "github.com/siderolabs/omni/internal/backend/grpc"
	"github.com/siderolabs/omni/internal/backend/grpc/router"
	"github.com/siderolabs/omni/internal/backend/health"
	"github.com/siderolabs/omni/internal/backend/imagefactory"
	"github.com/siderolabs/omni/internal/backend/k8sproxy"
	"github.com/siderolabs/omni/internal/backend/logging"
	"github.com/siderolabs/omni/internal/backend/monitoring"
	"github.com/siderolabs/omni/internal/backend/oidc"
	"github.com/siderolabs/omni/internal/backend/runtime"
	"github.com/siderolabs/omni/internal/backend/runtime/kubernetes"
	"github.com/siderolabs/omni/internal/backend/runtime/omni"
	"github.com/siderolabs/omni/internal/backend/runtime/talos"
	"github.com/siderolabs/omni/internal/backend/saml"
	"github.com/siderolabs/omni/internal/backend/workloadproxy"
	"github.com/siderolabs/omni/internal/frontend"
	"github.com/siderolabs/omni/internal/memconn"
	"github.com/siderolabs/omni/internal/pkg/auth"
	"github.com/siderolabs/omni/internal/pkg/auth/actor"
	"github.com/siderolabs/omni/internal/pkg/auth/auth0"
	"github.com/siderolabs/omni/internal/pkg/auth/handler"
	"github.com/siderolabs/omni/internal/pkg/auth/interceptor"
	"github.com/siderolabs/omni/internal/pkg/auth/role"
	serviceaccountmgmt "github.com/siderolabs/omni/internal/pkg/auth/serviceaccount"
	"github.com/siderolabs/omni/internal/pkg/cache"
	"github.com/siderolabs/omni/internal/pkg/compress"
	"github.com/siderolabs/omni/internal/pkg/config"
	"github.com/siderolabs/omni/internal/pkg/errgroup"
	"github.com/siderolabs/omni/internal/pkg/grpcutil"
	"github.com/siderolabs/omni/internal/pkg/kms"
	"github.com/siderolabs/omni/internal/pkg/machineevent"
	"github.com/siderolabs/omni/internal/pkg/siderolink"
	"github.com/siderolabs/omni/internal/pkg/xcontext"
)

// Server is main backend entrypoint that starts REST API, WebSocket and Serves static contents.
type Server struct {
	omniRuntime             *omni.Runtime
	logger                  *zap.Logger
	logHandler              *siderolink.LogHandler
	authConfig              *authres.Config
	dnsService              *dns.Service
	workloadProxyReconciler *workloadproxy.Reconciler
	imageFactoryClient      *imagefactory.Client
	auditor                 Auditor

	linkCounterDeltaCh chan<- siderolink.LinkCounterDeltas
	siderolinkEventsCh chan<- *omnires.MachineStatusSnapshot
	installEventCh     chan<- resource.ID

	proxyServer         Proxy
	bindAddress         string
	metricsBindAddress  string
	pprofBindAddress    string
	k8sProxyBindAddress string
	keyFile             string
	certFile            string
}

// NewServer creates new HTTP server.
func NewServer(
	bindAddress, metricsBindAddress, k8sProxyBindAddress, pprofBindAddress string,
	dnsService *dns.Service,
	workloadProxyReconciler *workloadproxy.Reconciler,
	imageFactoryClient *imagefactory.Client,
	linkCounterDeltaCh chan<- siderolink.LinkCounterDeltas,
	siderolinkEventsCh chan<- *omnires.MachineStatusSnapshot,
	installEventCh chan<- resource.ID,
	omniRuntime *omni.Runtime,
	talosRuntime *talos.Runtime,
	logHandler *siderolink.LogHandler,
	authConfig *authres.Config,
	keyFile, certFile string,
	proxyServer Proxy,
	auditor Auditor,
	logger *zap.Logger,
) (*Server, error) {
	s := &Server{
		omniRuntime:             omniRuntime,
		logger:                  logger.With(logging.Component("server")),
		logHandler:              logHandler,
		authConfig:              authConfig,
		dnsService:              dnsService,
		workloadProxyReconciler: workloadProxyReconciler,
		imageFactoryClient:      imageFactoryClient,
		auditor:                 auditor,
		linkCounterDeltaCh:      linkCounterDeltaCh,
		siderolinkEventsCh:      siderolinkEventsCh,
		installEventCh:          installEventCh,
		proxyServer:             proxyServer,
		bindAddress:             bindAddress,
		metricsBindAddress:      metricsBindAddress,
		pprofBindAddress:        pprofBindAddress,
		k8sProxyBindAddress:     k8sProxyBindAddress,
		keyFile:                 keyFile,
		certFile:                certFile,
	}

	k8sruntime, err := kubernetes.New(omniRuntime.State())
	if err != nil {
		return nil, err
	}

	prometheus.MustRegister(k8sruntime)

	runtime.Install(kubernetes.Name, k8sruntime)
	runtime.Install(talos.Name, talosRuntime)
	runtime.Install(omni.Name, s.omniRuntime)

	return s, nil
}

// RegisterRuntime adds a runtime.
func (s *Server) RegisterRuntime(name string, r runtime.Runtime) {
	runtime.Install(name, r)
}

// Run runs HTTP server.
func (s *Server) Run(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	s.omniRuntime.Run(ctx, eg)

	runtimeState := s.omniRuntime.State()
	oidcStorage := oidc.NewStorage(runtimeState, s.logger)

	eg.Go(func() error { return oidcStorage.Run(ctx) })

	oidcProvider, err := oidc.NewProvider(oidcStorage)
	if err != nil {
		return err
	}

	mux, err := s.makeMux(oidcProvider) //nolint:contextcheck
	if err != nil {
		return err
	}

	serverOptions, err := s.buildServerOptions() //nolint:contextcheck
	if err != nil {
		return err
	}

	services := grpcomni.MakeServiceServers(
		s.omniRuntime.State(),
		s.omniRuntime.CachedState(),
		s.logHandler,
		oidcProvider,
		oidcStorage,
		s.dnsService,
		s.imageFactoryClient,
		s.logger,
		s.auditor,
	)

	actualSrv, gtwyDialsTo, err := s.serverAndGateway(ctx, services, mux, serverOptions...)
	if err != nil {
		return err
	}

	proxyServer, prxDialsTo, err := s.makeProxyServer(ctx, eg)
	if err != nil {
		return err
	}

	var crtData *certData

	if s.certFile != "" && s.keyFile != "" {
		crtData = &certData{certFile: s.certFile, keyFile: s.keyFile}
	}

	workloadProxyHandler, err := s.workloadProxyHandler(mux)
	if err != nil {
		return fmt.Errorf("failed to create workload proxy handler: %w", err)
	}

	apiSrv := s.makeAPIServer(workloadProxyHandler, proxyServer, crtData)

	fns := []func() error{
		func() error { return proxyServer.Serve(ctx, gtwyDialsTo) },
		func() error { return actualSrv.Serve(ctx, prxDialsTo) },
		func() error { return apiSrv.Run(ctx) },
		func() error { return runMetricsServer(ctx, s.metricsBindAddress, s.logger) },
		func() error {
			return runK8sProxyServer(ctx, s.k8sProxyBindAddress, oidcStorage, crtData, s.omniRuntime.State(), s.auditor, s.logger)
		},
		func() error { return s.proxyServer.Run(ctx, apiSrv.Handler(), s.logger) },
		func() error { return s.logHandler.Start(ctx) },
		func() error { return s.runMachineAPI(ctx) },
		func() error { return s.auditor.RunCleanup(ctx) },
	}

	if s.pprofBindAddress != "" {
		fns = append(fns, func() error { return runPprofServer(ctx, s.pprofBindAddress, s.logger) })
	}

	for _, fn := range fns {
		eg.Go(fn)
	}

	if err = runLocalResourceServer(ctx, runtimeState, serverOptions, eg, s.logger); err != nil {
		return fmt.Errorf("failed to run local resource server: %w", err)
	}

	if config.Config.EmbeddedDiscoveryService.Enabled {
		eg.Go(func() error {
			if err = runEmbeddedDiscoveryService(ctx, s.logger); err != nil {
				return fmt.Errorf("failed to run discovery server over Siderolink: %w", err)
			}

			return nil
		})
	}

	if config.Config.InitialServiceAccount.Enabled {
		if err = s.createInitialServiceAccount(ctx); err != nil {
			return err
		}
	}

	return eg.Wait()
}

func (s *Server) makeMux(oidcProvider *oidc.Provider) (*http.ServeMux, error) {
	imageFactoryHandler := handler.NewAuthConfig(
		handler.NewSignature(
			&factory.Handler{
				State:  s.omniRuntime.State(),
				Logger: s.logger.With(logging.Component("factory_proxy")),
			},
			s.authenticatorFunc(),
			s.logger,
		),
		authres.Enabled(s.authConfig),
		s.logger,
	)

	samlHandler, err := func() (*samlsp.Middleware, error) {
		if !s.authConfig.TypedSpec().Value.Saml.Enabled {
			return nil, nil //nolint:nilnil
		}

		return saml.NewHandler(s.omniRuntime.State(), s.authConfig.TypedSpec().Value.Saml, s.logger)
	}()
	if err != nil {
		return nil, err
	}

	mux, err := makeMux(imageFactoryHandler, oidcProvider, samlHandler, s.omniRuntime, s.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create mux: %w", err)
	}

	return mux, err
}

func (s *Server) serverAndGateway(
	ctx context.Context,
	services iter.Seq2[grpcomni.ServiceServer, error],
	registerTo *http.ServeMux,
	serverOptions ...grpc.ServerOption,
) (*grpcServer, *memconn.Transport, error) {
	actualSrv, err := grpcomni.NewServer(services, serverOptions...)
	if err != nil {
		return nil, nil, err
	}

	gtwyDialsTo, err := grpcomni.RegisterGateway(ctx, services, registerTo, s.logger)
	if err != nil {
		return nil, nil, err
	}

	return &grpcServer{
		server: actualSrv,
		logger: s.logger,
	}, gtwyDialsTo, nil
}

func (s *Server) makeProxyServer(ctx context.Context, eg *errgroup.Group) (*grpcServer, *memconn.Transport, error) {
	transport := memconn.NewTransport("grpc-conn")

	rtr, err := router.NewRouter(
		transport,
		s.omniRuntime.State(),
		s.dnsService,
		authres.Enabled(s.authConfig),
		s.auditor,
		interceptor.NewSignature(s.authenticatorFunc(), s.logger).Unary(),
	)
	if err != nil {
		return nil, nil, err
	}

	prometheus.MustRegister(rtr)

	eg.Go(func() error { return rtr.ResourceWatcher(ctx, s.omniRuntime.State(), s.logger) })

	srv := router.NewServer(rtr,
		router.Interceptors(s.logger),
		grpc.ChainStreamInterceptor(
			grpcutil.StreamSetAuditData(),
			// enabled is always true here because we are interested in audit data rather than auth process
			interceptor.NewAuthConfig(true, s.logger).Stream(),
		),
		grpc.MaxRecvMsgSize(constants.GRPCMaxMessageSize),
	)

	return &grpcServer{
		server: srv,
		logger: s.logger,
	}, transport, nil
}

// buildServerOptions builds the gRPC server options.
//
// Recovery is installed as the first middleware in the chain to handle panics (via defer and recover()) in all subsequent middlewares.
//
// Logging is installed as the first middleware (even before recovery middleware) in the chain
// so that request in the form it was received and status sent on the wire is logged (error/success).
// It also tracks the whole duration of the request, including other middleware overhead.
func (s *Server) buildServerOptions() ([]grpc.ServerOption, error) {
	recoveryOpt := grpc_recovery.WithRecoveryHandler(recoveryHandler(s.logger))
	messageProducer := grpcutil.LogLevelOverridingMessageProducer(grpc_zap.DefaultMessageProducer)
	logLevelOverrideUnaryInterceptor, logLevelOverrideStreamInterceptor := grpcutil.LogLevelInterceptors()

	grpc_prometheus.EnableHandlingTimeHistogram(grpc_prometheus.WithHistogramBuckets([]float64{0.001, 0.01, 0.1, 1, 10, 30, 60, 120, 300, 600}))

	unaryInterceptors := []grpc.UnaryServerInterceptor{
		grpc_ctxtags.UnaryServerInterceptor(),
		logLevelOverrideUnaryInterceptor,
		grpc_zap.UnaryServerInterceptor(s.logger, grpc_zap.WithMessageProducer(messageProducer)),
		grpcutil.SetUserAgent(),
		grpcutil.SetRealPeerAddress(),
		grpcutil.SetAuditData(),
		grpcutil.InterceptBodyToTags(
			grpcutil.NewHook(
				grpcutil.NewRewriter(resourceServerCreate),
				grpcutil.NewRewriter(resourceServerUpdate),
				grpcutil.NewRewriter(cosiResourceServerCreate),
				grpcutil.NewRewriter(cosiResourceServerUpdate),
			),
			1024,
		),
		grpc_prometheus.UnaryServerInterceptor,
		grpc_recovery.UnaryServerInterceptor(recoveryOpt),
	}

	streamInterceptors := []grpc.StreamServerInterceptor{
		grpc_ctxtags.StreamServerInterceptor(),
		logLevelOverrideStreamInterceptor,
		grpc_zap.StreamServerInterceptor(s.logger, grpc_zap.WithMessageProducer(messageProducer)),
		grpcutil.StreamSetUserAgent(),
		grpcutil.StreamSetRealPeerAddress(),
		grpcutil.StreamSetAuditData(),
		grpcutil.StreamIntercept(
			grpcutil.StreamHooks{
				RecvMsg: grpcutil.StreamInterceptRequestBodyToTags(
					grpcutil.NewHook(
						grpcutil.NewRewriter(resourceServerCreate),
						grpcutil.NewRewriter(resourceServerUpdate),
						grpcutil.NewRewriter(cosiResourceServerCreate),
						grpcutil.NewRewriter(cosiResourceServerUpdate),
					),
					1024,
				),
			},
		),
		grpc_prometheus.StreamServerInterceptor,
		grpc_recovery.StreamServerInterceptor(recoveryOpt),
	}

	authInterceptors, err := s.getAuthInterceptors()
	if err != nil {
		return nil, err
	}

	for _, authInterceptor := range authInterceptors {
		unaryInterceptors = append(unaryInterceptors, authInterceptor.Unary())
		streamInterceptors = append(streamInterceptors, authInterceptor.Stream())
	}

	return []grpc.ServerOption{
		grpc.MaxRecvMsgSize(constants.GRPCMaxMessageSize),
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
		grpc.SharedWriteBuffer(true),
	}, nil
}

type interceptorCreator interface {
	Unary() grpc.UnaryServerInterceptor
	Stream() grpc.StreamServerInterceptor
}

func (s *Server) getAuthInterceptors() ([]interceptorCreator, error) {
	authEnabled := authres.Enabled(s.authConfig)

	result := []interceptorCreator{interceptor.NewAuthConfig(authEnabled, s.logger)}

	if !authEnabled {
		return result, nil
	}

	// auth is enabled, add signature and jwt interceptors
	result = append(result, interceptor.NewSignature(s.authenticatorFunc(), s.logger))

	switch {
	case s.authConfig.TypedSpec().Value.Auth0.Enabled:
		verifier, err := auth0.NewIDTokenVerifier(s.authConfig.TypedSpec().Value.GetAuth0().Domain)
		if err != nil {
			return nil, err
		}

		result = append(result, interceptor.NewJWT(verifier, s.logger))

	case s.authConfig.TypedSpec().Value.Saml.Enabled:
		result = append(result, interceptor.NewSAML(s.omniRuntime.State(), s.logger))
	}

	return result, nil
}

func (s *Server) authenticatorFunc() auth.AuthenticatorFunc {
	return func(ctx context.Context, fingerprint string) (*auth.Authenticator, error) {
		ctx = actor.MarkContextAsInternalActor(ctx)

		ptr := authres.NewPublicKey(resources.DefaultNamespace, fingerprint).Metadata()

		pubKey, err := safe.StateGet[*authres.PublicKey](ctx, s.omniRuntime.State(), ptr)
		if err != nil {
			return nil, err
		}

		if pubKey.TypedSpec().Value.Expiration.AsTime().Before(time.Now()) {
			return nil, errors.New("public key expired")
		}

		if !pubKey.TypedSpec().Value.Confirmed {
			return nil, errors.New("public key not confirmed")
		}

		userID, labelExists := pubKey.Metadata().Labels().Get(authres.LabelPublicKeyUserID)
		if !labelExists {
			return nil, errors.New("public key has no user ID label")
		}

		key, err := pgpcrypto.NewKeyFromArmored(string(pubKey.TypedSpec().Value.GetPublicKey()))
		if err != nil {
			return nil, err
		}

		verifier, err := pgp.NewKey(key)
		if err != nil {
			return nil, err
		}

		user, err := safe.StateGet[*authres.User](ctx, s.omniRuntime.State(), resource.NewMetadata(resources.DefaultNamespace, authres.UserType, userID, resource.VersionUndefined))
		if err != nil {
			return nil, err
		}

		finalRole, err := role.Min(role.Role(user.TypedSpec().Value.GetRole()), role.Role(pubKey.TypedSpec().Value.GetRole()))
		if err != nil {
			return nil, err
		}

		if config.Config.Auth.Suspended {
			finalRole = role.Reader
		}

		return &auth.Authenticator{
			UserID:   userID,
			Identity: pubKey.TypedSpec().Value.GetIdentity().GetEmail(),
			Role:     finalRole,
			Verifier: verifier,
		}, nil
	}
}

func (s *Server) runMachineAPI(ctx context.Context) error {
	wgAddress := config.Config.SiderolinkWireguardBindAddress

	params := siderolink.Params{
		WireguardEndpoint:  wgAddress,
		AdvertisedEndpoint: config.Config.SiderolinkWireguardAdvertisedAddress,
		APIEndpoint:        config.Config.MachineAPIBindAddress,
		Cert:               config.Config.MachineAPICertFile,
		Key:                config.Config.MachineAPIKeyFile,
		EventSinkPort:      strconv.Itoa(config.Config.EventSinkPort),
	}

	omniState := s.omniRuntime.State()
	machineEventHandler := machineevent.NewHandler(omniState, s.logger, s.siderolinkEventsCh, s.installEventCh)

	slink, err := siderolink.NewManager(
		ctx,
		omniState,
		siderolink.DefaultWireguardHandler,
		params,
		s.logger.With(logging.Component("siderolink")).WithOptions(
			zap.AddStacktrace(zapcore.ErrorLevel), // prevent warn level from printing stack traces
		),
		s.logHandler,
		machineEventHandler,
		s.linkCounterDeltaCh,
	)
	if err != nil {
		return err
	}

	kms := kms.NewManager(
		omniState,
		s.logger.With(logging.Component("kms")).WithOptions(
			zap.AddStacktrace(zapcore.ErrorLevel), // prevent warn level from printing stack traces
		),
	)

	prometheus.MustRegister(slink)

	// start API listener
	lis, err := params.NewListener()
	if err != nil {
		return fmt.Errorf("error listening for Siderolink gRPC API: %w", err)
	}

	eg, groupCtx := errgroup.WithContext(ctx)

	server := grpc.NewServer(
		grpc.SharedWriteBuffer(true),
	)

	slink.Register(server)
	kms.Register(server)

	eg.Go(func() error {
		return slink.Run(groupCtx,
			siderolink.ListenHost,
			strconv.Itoa(config.Config.EventSinkPort),
			strconv.Itoa(talosconstants.TrustdPort),
			strconv.Itoa(config.Config.LogServerPort),
		)
	})

	grpcutil.RunServer(groupCtx, server, lis, eg, s.logger)

	return eg.Wait()
}

func (s *Server) workloadProxyHandler(next http.Handler) (http.Handler, error) {
	roleProvider, err := workloadproxy.NewAccessPolicyRoleProvider(s.omniRuntime.State())
	if err != nil {
		return nil, fmt.Errorf("failed to create access policy role provider: %w", err)
	}

	pgpSignatureValidator, err := workloadproxy.NewPGPAccessValidator(s.omniRuntime.State(), roleProvider,
		s.logger.With(logging.Component("pgp_access_validator")))
	if err != nil {
		return nil, fmt.Errorf("failed to create pgp signature validator: %w", err)
	}

	mainURL, err := url.Parse(config.Config.APIURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse API URL: %w", err)
	}

	return workloadproxy.NewHTTPHandler(
		next,
		s.workloadProxyReconciler,
		pgpSignatureValidator,
		mainURL,
		config.Config.WorkloadProxying.Subdomain,
		s.logger.With(logging.Component("workload_proxy_handler")),
	)
}

func (s *Server) makeAPIServer(regular http.Handler, grpcServer *grpcServer, data *certData) *apiServer {
	wrap := func(fn func(w http.ResponseWriter, req *http.Request)) http.Handler {
		if data != nil {
			return http.HandlerFunc(fn)
		}

		// If we don't have TLS data, wrap the handler in http2.Server
		return h2c.NewHandler(http.HandlerFunc(fn), &http2.Server{})
	}

	srv := &http.Server{
		Addr: s.bindAddress,
		Handler: wrap(func(w http.ResponseWriter, req *http.Request) {
			if req.ProtoMajor == 2 && strings.HasPrefix(
				req.Header.Get("Content-Type"), "application/grpc") {
				// grpcServer provides top-level gRPC proxy handler.
				grpcServer.ServeHTTP(w, setRealIPRequest(req))

				return
			}

			// handler contains "regular" HTTP handlers
			regular.ServeHTTP(w, req)
		}),
	}

	return &apiServer{
		srv:    srv,
		cert:   data,
		logger: s.logger.With(zap.String("server", s.bindAddress), zap.String("server_type", "api")),
	}
}

type apiServer struct {
	srv    *http.Server
	cert   *certData
	logger *zap.Logger
}

func (s *apiServer) Run(ctx context.Context) error {
	return (&server{server: s.srv, certData: s.cert}).Run(ctx, s.logger)
}

func (s *apiServer) Handler() http.Handler { return s.srv.Handler }

func recoveryHandler(logger *zap.Logger) grpc_recovery.RecoveryHandlerFunc {
	return func(p any) error {
		if logger != nil {
			logger.Error("grpc panic", zap.Any("panic", p), zap.Stack("stack"))
		}

		return status.Errorf(codes.Internal, "%v", p)
	}
}

func cosiResourceServerCreate(req *v1alpha1.CreateRequest) (*v1alpha1.CreateRequest, bool) {
	if isSensitiveResource(req.Resource) {
		req.Resource.Spec = nil

		return req, true
	}

	return nil, false
}

func cosiResourceServerUpdate(req *v1alpha1.UpdateRequest) (*v1alpha1.UpdateRequest, bool) {
	if isSensitiveResource(req.NewResource) {
		req.NewResource.Spec = nil

		return req, true
	}

	return nil, false
}

func resourceServerCreate(resCopy *resapi.CreateRequest) (*resapi.CreateRequest, bool) {
	if isSensitiveSpec(resCopy.Resource) {
		resCopy.Resource.Spec = ""

		return resCopy, true
	}

	return nil, false
}

func resourceServerUpdate(resCopy *resapi.UpdateRequest) (*resapi.UpdateRequest, bool) {
	if isSensitiveSpec(resCopy.Resource) {
		resCopy.Resource.Spec = ""

		return resCopy, true
	}

	return nil, false
}

func isSensitiveResource(res *v1alpha1.Resource) bool {
	protoR, err := protobuf.Unmarshal(res)
	if err != nil {
		return false
	}

	properResource, err := protobuf.UnmarshalResource(protoR)
	if err != nil {
		return false
	}

	resDef, ok := properResource.(meta.ResourceDefinitionProvider)
	if !ok || resDef.ResourceDefinition().Sensitivity == meta.Sensitive {
		// If we have !ok we do not know if this resource have Sensitive field, so we will mask it anyway.
		return true
	}

	return false
}

func isSensitiveSpec(resource *resapi.Resource) bool {
	res, err := grpcomni.CreateResource(resource)
	if err != nil {
		return false
	}

	resDef, ok := res.(meta.ResourceDefinitionProvider)
	if !ok || resDef.ResourceDefinition().Sensitivity == meta.Sensitive {
		// If we have !ok we do not know if this resource have Sensitive field, so we will mask it anyway.
		return true
	}

	return false
}

func makeMux(imageHandler, oidcHandler http.Handler, samlHandler *samlsp.Middleware, omniRuntime *omni.Runtime, logger *zap.Logger) (*http.ServeMux, error) {
	mux := http.NewServeMux()

	muxHandle := func(route string, handler http.Handler, value string) {
		mux.Handle(route, monitoring.NewHandler(
			logging.NewHandler(handler, logger.With(zap.String("handler", value))),
			prometheus.Labels{"handler": value},
		))
	}

	muxHandle(
		"/",
		compress.Handler(
			frontend.NewStaticHandler(7200),
			gzip.BestCompression,
		),
		"static",
	)

	if samlHandler != nil {
		saml.RegisterHandlers(samlHandler, mux, logger)
	}

	muxHandle("/image/", imageHandler, "image")

	omnictlHndlr, err := getOmnictlDownloads("./omnictl/")
	if err != nil {
		return nil, err
	}

	talosctlHandler, err := makeTalosctlHandler(omniRuntime.State(), logger)
	if err != nil {
		return nil, err
	}

	muxHandle("/omnictl/", http.StripPrefix("/omnictl/", omnictlHndlr), "files")
	muxHandle("/talosctl/downloads", talosctlHandler, "talosctl-downloads")
	// actually enabled only in debug build
	muxHandle("/debug/", debug.NewHandler(omniRuntime.GetCOSIRuntime(), omniRuntime.State()), "debug")

	// OIDC Provider
	mux.Handle("/oidc/",
		http.StripPrefix("/oidc",
			monitoring.NewHandler(
				logging.NewHandler(
					oidcHandler,
					logger.With(zap.String("handler", "debug")),
				),
				prometheus.Labels{"handler": "debug"},
			),
		),
	)

	// Health checks
	muxHandle("/healthz", health.NewHandler(omniRuntime.State(), logger), "health")

	return mux, nil
}

func getOmnictlDownloads(dir string) (http.Handler, error) {
	readDir, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: %w", dir, err)
	}

	for _, entry := range readDir {
		name := entry.Name()

		if !entry.Type().IsRegular() {
			return nil, fmt.Errorf("entry %q is not a regular file in %q", name, dir)
		}
	}

	return http.FileServer(http.Dir(dir)), nil
}

func runMetricsServer(ctx context.Context, bindAddress string, logger *zap.Logger) error {
	var metricsMux http.ServeMux

	metricsMux.Handle("/metrics", promhttp.Handler())

	metricsServer := &http.Server{
		Addr:    bindAddress,
		Handler: &metricsMux,
	}

	logger = logger.With(zap.String("server", bindAddress), zap.String("server_type", "metrics"))

	return (&server{server: metricsServer}).Run(ctx, logger)
}

type oidcStore interface {
	GetPublicKeyByID(keyID string) (any, error)
}

func runK8sProxyServer(
	ctx context.Context,
	bindAddress string,
	oidcStorage oidcStore,
	data *certData,
	runtimeState state.State,
	wrapper k8sproxy.MiddlewareWrapper,
	logger *zap.Logger,
) error {
	keyFunc := func(_ context.Context, keyID string) (any, error) {
		return oidcStorage.GetPublicKeyByID(keyID)
	}

	clusterUUIDResolver := func(ctx context.Context, clusterID string) (resource.ID, error) {
		ctx = actor.MarkContextAsInternalActor(ctx)

		uuid, resolveErr := safe.StateGetByID[*omnires.ClusterUUID](ctx, runtimeState, clusterID)
		if resolveErr != nil {
			return "", fmt.Errorf("failed to resolve cluster ID to UUID: %w", resolveErr)
		}

		return uuid.TypedSpec().Value.Uuid, nil
	}

	k8sProxyHandler, err := k8sproxy.NewHandler(keyFunc, clusterUUIDResolver, wrapper, logger)
	if err != nil {
		return err
	}

	prometheus.MustRegister(k8sProxyHandler)

	k8sProxy := monitoring.NewHandler(
		logging.NewHandler(
			k8sProxyHandler,
			logger.With(zap.String("handler", "k8s_proxy")),
		),
		prometheus.Labels{"handler": "k8s-proxy"},
	)

	k8sProxyServer := &http.Server{
		Addr:    bindAddress,
		Handler: k8sProxy,
	}

	logger = logger.With(zap.String("server", bindAddress), zap.String("server_type", "k8s_proxy"))

	return (&server{server: k8sProxyServer, certData: data}).Run(ctx, logger)
}

// setRealIPRequest extracts ip from the request and sets it to the X-Forwarded-For header if there is no
// existing X-Forwarded-For.
func setRealIPRequest(req *http.Request) *http.Request {
	if req.Header.Get("X-Forwarded-For") != "" {
		return req
	}

	actualIP, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req
	}

	newReq := req.Clone(req.Context())

	newReq.Header.Set("X-Forwarded-For", actualIP)

	return newReq
}

type server struct {
	server   *http.Server
	certData *certData
}

type certData struct {
	cert     tls.Certificate
	certFile string
	keyFile  string
	mu       sync.Mutex
	loaded   bool
}

func (c *certData) load() error {
	cert, err := tls.LoadX509KeyPair(c.certFile, c.keyFile)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.loaded = true
	c.cert = cert

	return nil
}

func (c *certData) getCert() (*tls.Certificate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.loaded {
		return nil, fmt.Errorf("the cert wasn't loaded yet")
	}

	return &c.cert, nil
}

func (c *certData) runWatcher(ctx context.Context, logger *zap.Logger) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("error creating fsnotify watcher: %w", err)
	}
	defer w.Close() //nolint:errcheck

	if err = w.Add(c.certFile); err != nil {
		return fmt.Errorf("error adding watch for file %s: %w", c.certFile, err)
	}

	if err = w.Add(c.keyFile); err != nil {
		return fmt.Errorf("error adding watch for file %s: %w", c.keyFile, err)
	}

	handleEvent := func(e fsnotify.Event) error {
		defer func() {
			if err = c.load(); err != nil {
				logger.Error("failed to load certs", zap.Error(err))

				return
			}

			logger.Info("reloaded certs")
		}()

		if !e.Has(fsnotify.Remove) && !e.Has(fsnotify.Rename) {
			return nil
		}

		if err = w.Remove(e.Name); err != nil {
			logger.Error("failed to remove file watch, it may have been deleted", zap.String("file", e.Name), zap.Error(err))
		}

		if err = w.Add(e.Name); err != nil {
			return fmt.Errorf("error adding watch for file %s: %w", e.Name, err)
		}

		return nil
	}

	for {
		select {
		case e := <-w.Events:
			if err = handleEvent(e); err != nil {
				return err
			}
		case err = <-w.Errors:
			return fmt.Errorf("received fsnotify error: %w", err)
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *server) Run(ctx context.Context, logger *zap.Logger) error {
	logger.Info("server starting")
	defer logger.Info("server stopped")

	stop := xcontext.AfterFuncSync(ctx, func() { //nolint:contextcheck
		logger.Info("server stopping")

		shutdownCtx, shutdownCtxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCtxCancel()

		err := s.shutdown(shutdownCtx)
		if err != nil {
			logger.Error("failed to gracefully stop server", zap.Error(err))
		}
	})

	defer stop()

	if err := s.listenAndServe(ctx, logger); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

func (s *server) listenAndServe(ctx context.Context, logger *zap.Logger) error {
	if s.certData == nil {
		return s.server.ListenAndServe()
	}

	if err := s.certData.load(); err != nil {
		return err
	}

	s.server.TLSConfig = &tls.Config{
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return s.certData.getCert()
		},
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eg := panichandler.NewErrGroup()

	eg.Go(func() error {
		for {
			err := s.certData.runWatcher(ctx, logger)

			if err == nil {
				return nil
			}

			logger.Error("cert watcher crashed, restarting in 5 seconds", zap.Error(err))

			time.Sleep(time.Second * 5)
		}
	})

	eg.Go(func() error {
		defer cancel()

		return s.server.ListenAndServeTLS("", "")
	})

	return eg.Wait()
}

func (s *server) shutdown(ctx context.Context) error {
	err := s.server.Shutdown(ctx)
	if !errors.Is(ctx.Err(), err) {
		return err
	}

	if closeErr := s.server.Close(); closeErr != nil {
		return fmt.Errorf("failed to close server: %w", closeErr)
	}

	return err
}

func runLocalResourceServer(ctx context.Context, st state.CoreState, serverOptions []grpc.ServerOption, eg *errgroup.Group, logger *zap.Logger) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", config.Config.LocalResourceServerPort))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	unaryInterceptor := grpc.UnaryServerInterceptor(func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if ok && md.Get(constants.InfraProviderMetadataKey) != nil {
			return handler(actor.MarkContextAsInfraProvider(
				ctx,
				md.Get(constants.InfraProviderMetadataKey)[0],
			), req)
		}

		return handler(actor.MarkContextAsInternalActor(ctx), req)
	})

	streamInterceptor := grpc.StreamServerInterceptor(func(srv interface{}, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(ctx)
		if ok && md.Get(constants.InfraProviderMetadataKey) != nil {
			return handler(srv, &grpc_middleware.WrappedServerStream{
				ServerStream:   ss,
				WrappedContext: actor.MarkContextAsInfraProvider(ss.Context(), md.Get(constants.InfraProviderMetadataKey)[0]),
			})
		}

		return handler(srv, &grpc_middleware.WrappedServerStream{
			ServerStream:   ss,
			WrappedContext: actor.MarkContextAsInternalActor(ss.Context()),
		})
	})

	serverOptions = append([]grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryInterceptor),
		grpc.ChainStreamInterceptor(streamInterceptor),
		grpc.SharedWriteBuffer(true),
	}, serverOptions...)

	grpcServer := grpc.NewServer(serverOptions...)

	readOnlyState := state.WrapCore(state.Filter(st, func(ctx context.Context, access state.Access) error {
		md, ok := metadata.FromIncomingContext(ctx)
		if ok && md.Get(constants.InfraProviderMetadataKey) != nil {
			return nil
		}

		if !access.Verb.Readonly() {
			return status.Error(codes.PermissionDenied, "only read-only access is permitted")
		}

		return nil
	}))

	v1alpha1.RegisterStateServer(grpcServer, protobufserver.NewState(readOnlyState))

	logger.Info("starting local resource server")

	grpcutil.RunServer(ctx, grpcServer, listener, eg, logger)

	return nil
}

func (s *Server) createInitialServiceAccount(ctx context.Context) error {
	serviceAccountEmail := config.Config.InitialServiceAccount.Name + access.ServiceAccountNameSuffix

	identity, err := safe.ReaderGetByID[*authres.Identity](ctx, s.omniRuntime.State(), serviceAccountEmail)
	if err != nil && !state.IsNotFoundError(err) {
		return err
	}

	// already created, skip
	if identity != nil {
		return nil
	}

	key, err := pgp.GenerateKey(config.Config.InitialServiceAccount.Name, "automation initial", serviceAccountEmail, config.Config.InitialServiceAccount.Lifetime)
	if err != nil {
		return fmt.Errorf("failed to create initial service account key, generate failed: %w", err)
	}

	k, err := key.ArmorPublic()
	if err != nil {
		return fmt.Errorf("failed to create initial service account key, armor failed: %w", err)
	}

	_, err = serviceaccountmgmt.Create(
		ctx,
		s.omniRuntime.State(),
		config.Config.InitialServiceAccount.Name,
		config.Config.InitialServiceAccount.Role,
		false,
		[]byte(k),
	)
	if err != nil {
		return fmt.Errorf("failed to create initial service account key: %w", err)
	}

	data, err := serviceaccount.Encode(config.Config.InitialServiceAccount.Name, key)
	if err != nil {
		return fmt.Errorf("failed to create initial service account key, failed to encode: %w", err)
	}

	if err = os.WriteFile(config.Config.InitialServiceAccount.KeyPath, []byte(data), 0o640); err != nil {
		return fmt.Errorf(
			"failed to create initial service account key, failed to write key to path %q: %w",
			config.Config.InitialServiceAccount.KeyPath,
			err,
		)
	}

	return nil
}

// runEmbeddedDiscoveryService runs an embedded discovery service over Siderolink.
func runEmbeddedDiscoveryService(ctx context.Context, logger *zap.Logger) error {
	logLevel, err := zapcore.ParseLevel(config.Config.EmbeddedDiscoveryService.LogLevel)
	if err != nil {
		logLevel = zapcore.WarnLevel

		logger.Warn("failed to parse log level, fallback", zap.String("fallback_level", logLevel.String()), zap.Error(err))
	}

	registerer := prometheus.WrapRegistererWithPrefix("discovery_", prometheus.DefaultRegisterer)

	if err = retry.Constant(30*time.Second, retry.WithUnits(time.Second)).RetryWithContext(ctx, func(context.Context) error {
		err = service.Run(ctx, service.Options{
			ListenAddr:        net.JoinHostPort(siderolink.ListenHost, strconv.Itoa(config.Config.EmbeddedDiscoveryService.Port)),
			GCInterval:        time.Minute,
			MetricsRegisterer: registerer,

			SnapshotsEnabled: config.Config.EmbeddedDiscoveryService.SnapshotsEnabled,
			SnapshotInterval: config.Config.EmbeddedDiscoveryService.SnapshotInterval,
			SnapshotPath:     config.Config.EmbeddedDiscoveryService.SnapshotPath,
		}, logger.WithOptions(zap.IncreaseLevel(logLevel)).With(logging.Component("discovery_service")))

		if errors.Is(err, syscall.EADDRNOTAVAIL) {
			return retry.ExpectedError(err)
		}

		return err
	}); err != nil {
		return fmt.Errorf("failed to start discovery service: %w", err)
	}

	return nil
}

func runPprofServer(ctx context.Context, bindAddress string, l *zap.Logger) error {
	mux := &http.ServeMux{}

	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{
		Addr:    bindAddress,
		Handler: mux,
	}

	l = l.With(zap.String("server", bindAddress), zap.String("server_type", "pprof"))

	return (&server{server: srv}).Run(ctx, l)
}

//nolint:unparam
func makeTalosctlHandler(state state.State, logger *zap.Logger) (http.Handler, error) {
	// The list of versions does not update very often, so we can cache it.
	cacher := cache.Value[releaseData]{Duration: time.Hour}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type result struct {
			ReleaseData *releaseData `json:"release_data,omitempty"`
			Status      string       `json:"status"`
		}

		writeResult := func(a any, code int) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)

			if err := json.NewEncoder(w).Encode(a); err != nil {
				logger.Error("failed to encode result", zap.Error(err))
			}
		}

		ctx := actor.MarkContextAsInternalActor(r.Context())

		data, err := cacher.GetOrUpdate(func() (releaseData, error) { return getReleaseData(ctx, state) })
		if err != nil {
			logger.Error("failed to get latest talosctl release", zap.Error(err))
			writeResult(result{Status: "failed to get latest talosctl release"}, http.StatusInternalServerError)

			return
		}

		writeResult(result{
			ReleaseData: &data,
			Status:      "ok",
		}, http.StatusOK)
	}), nil
}

func getReleaseData(ctx context.Context, state state.State) (releaseData, error) {
	all, err := safe.StateListAll[*omnires.TalosVersion](ctx, state)
	if err != nil {
		return releaseData{}, fmt.Errorf("failed to list all talos versions: %w", err)
	}

	if all.Len() == 0 {
		return releaseData{}, errors.New("no talos versions found")
	}

	versionNames := make([]string, 0, all.Len())

	for val := range all.All() {
		version := val.TypedSpec().Value.Version
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}

		versionNames = append(versionNames, version)
	}

	releases, err := getGithubReleases(versionNames...)
	if err != nil {
		return releaseData{}, err
	}

	return releases, nil
}

func getGithubReleases(tags ...string) (releaseData, error) {
	if len(tags) == 0 {
		return releaseData{}, errors.New("no tags provided")
	}

	versions := make(map[string][]talosctlAsset, len(tags))

	for _, tag := range tags {
		assets := make([]talosctlAsset, 0, len(assetsData))

		for _, asset := range assetsData {
			assets = append(assets, talosctlAsset{
				Name: asset.name,
				URL:  fmt.Sprintf("https://github.com/siderolabs/talos/releases/download/%s/%s", tag, asset.urlPart),
			})
		}

		versions[tag] = assets
	}

	return releaseData{
		AvailableVersions: versions,
		DefaultVersion:    tags[len(tags)-1],
	}, nil
}

type releaseData struct {
	AvailableVersions map[string][]talosctlAsset `json:"available_versions"`
	DefaultVersion    string                     `json:"default_version,omitempty"`
}

type talosctlAsset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

var assetsData = []struct {
	name    string
	urlPart string
}{
	{
		"Apple",
		"talosctl-darwin-amd64",
	},
	{
		"Apple Silicon",
		"talosctl-darwin-arm64",
	},
	{
		"Linux",
		"talosctl-linux-amd64",
	},
	{
		"Linux ARM",
		"talosctl-linux-armv7",
	},
	{
		"Linux ARM64",
		"talosctl-linux-arm64",
	},
	{
		"Windows",
		"talosctl-windows-amd64.exe",
	},
}

// Auditor is a common interface for audit log.
type Auditor interface {
	RunCleanup(context.Context) error
	ReadAuditLog(start, end time.Time) (io.ReadCloser, error)
	router.TalosAuditor
	k8sproxy.MiddlewareWrapper
}

type grpcServer struct {
	server *grpc.Server
	logger *zap.Logger
}

func (s *grpcServer) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.server.ServeHTTP(w, r) }
func (s *grpcServer) Serve(ctx context.Context, from listenerBuilder) error {
	listenFrom, err := from.Listener()
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	s.logger.Info("internal API server starting", zap.String("address", listenFrom.Addr().String()))
	defer s.logger.Info("internal API server stopped")

	defer xcontext.AfterFuncSync(ctx, func() {
		s.logger.Info("internal API server stopping")

		// Since we use a memconn transport and ServeHTTP, we can't use the graceful shutdown
		s.server.Stop()
	})()

	if err = s.server.Serve(listenFrom); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

type listenerBuilder interface{ Listener() (net.Listener, error) }
