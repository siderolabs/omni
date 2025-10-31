// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package exposedservice

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// Reconciler is the reconciler for ExposedService resources.
type Reconciler struct {
	logger *zap.Logger

	exposedServices map[resource.ID]*omni.ExposedService
	usedAliases     map[string]resource.ID

	cluster                string
	workloadProxySubdomain string
	advertisedAPIURL       string

	services []*corev1.Service
}

// NewReconciler creates a new ExposedService reconciler.
//
// The Reconciler is supposed to be used only once.
func NewReconciler(cluster, workloadProxySubdomain, advertisedAPIURL string,
	exposedServices []*omni.ExposedService, kubernetesServices []*corev1.Service, logger *zap.Logger,
) (*Reconciler, error) {
	exposedServicesMap := make(map[resource.ID]*omni.ExposedService, len(exposedServices))
	usedAliases := make(map[string]resource.ID, len(exposedServicesMap))

	for _, exposedService := range exposedServices {
		alias, _ := exposedService.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
		exposedServicesMap[exposedService.Metadata().ID()] = exposedService
		usedAliases[alias] = exposedService.Metadata().ID()
	}

	return &Reconciler{
		logger:                 logger,
		usedAliases:            usedAliases,
		exposedServices:        exposedServicesMap,
		cluster:                cluster,
		workloadProxySubdomain: workloadProxySubdomain,
		advertisedAPIURL:       advertisedAPIURL,
		services:               kubernetesServices,
	}, nil
}

// ReconcileServices reconciles the ExposedService resources for the given services and returns the ones that should be kept.
//
// It is supposed to be called only once.
func (reconciler *Reconciler) ReconcileServices() ([]*omni.ExposedService, error) {
	servicesToKeep := make([]*omni.ExposedService, 0, len(reconciler.services))

	for _, service := range reconciler.services {
		svcID := service.Name + "." + service.Namespace
		logger := reconciler.logger.With(zap.String("service", svcID))
		exposedServiceID := reconciler.cluster + "/" + svcID

		exposedService := reconciler.exposedServices[exposedServiceID]
		if exposedService == nil {
			exposedService = omni.NewExposedService(resources.DefaultNamespace, reconciler.cluster+"/"+svcID)
		}

		keep, err := reconciler.reconcileService(exposedService, service, logger)
		if err != nil {
			return nil, err
		}

		if keep {
			servicesToKeep = append(servicesToKeep, exposedService)
		}
	}

	return servicesToKeep, nil
}

func (reconciler *Reconciler) reconcileService(exposedService *omni.ExposedService, service *corev1.Service, logger *zap.Logger) (keep bool, err error) {
	port, err := strconv.Atoi(service.Annotations[constants.ExposedServicePortAnnotationKey])
	if err != nil || port < 1 || port > 65535 {
		logger.Warn("invalid port on Service", zap.String("port", service.Annotations[constants.ExposedServicePortAnnotationKey]))

		return true, nil //nolint:nilerr
	}

	label, labelOk := service.Annotations[constants.ExposedServiceLabelAnnotationKey]
	if !labelOk {
		label = service.Name + "." + service.Namespace
	}

	icon, err := reconciler.parseIcon(service.Annotations[constants.ExposedServiceIconAnnotationKey])
	if err != nil {
		logger.Debug("invalid icon on Service", zap.Error(err))
	}

	explicitAliasOpt := optional.None[string]()
	if explicitAlias, ok := service.Annotations[constants.ExposedServicePrefixAnnotationKey]; ok {
		explicitAliasOpt = optional.Some(explicitAlias)
	}

	if err = reconciler.updateExposedService(exposedService, explicitAliasOpt, port, label, icon, logger); err != nil {
		return false, fmt.Errorf("error updating exposed service: %w", err)
	}

	alias, _ := exposedService.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
	reconciler.usedAliases[alias] = exposedService.Metadata().ID()
	reconciler.exposedServices[exposedService.Metadata().ID()] = exposedService

	return true, nil
}

func (reconciler *Reconciler) updateExposedService(res *omni.ExposedService, explicitAliasOpt optional.Optional[string], port int, label, icon string, logger *zap.Logger) error {
	res.Metadata().Labels().Set(omni.LabelCluster, reconciler.cluster)

	requestedExplicitAlias, explicitAliasRequested := explicitAliasOpt.Get()
	hasExplicitAlias := res.TypedSpec().Value.HasExplicitAlias
	currentAlias, hasExistingAlias := res.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
	requestedExplicitAlias = strings.ToLower(requestedExplicitAlias)
	currentAlias = strings.ToLower(currentAlias)

	// if the service has an explicit alias, but it is now getting a new alias,
	// we need to free it here, so that it can be reused by another service
	abandoningCurrentExplicitAlias := hasExistingAlias && hasExplicitAlias &&
		(!explicitAliasRequested || requestedExplicitAlias != currentAlias)
	if abandoningCurrentExplicitAlias {
		if ownerID, ok := reconciler.usedAliases[currentAlias]; ok && ownerID == res.Metadata().ID() {
			delete(reconciler.usedAliases, currentAlias)
		}
	}

	var (
		alias string
		err   error
	)

	switch {
	// replace the existing alias with the explicit one
	case requestedExplicitAlias != "":
		alias = strings.ToLower(requestedExplicitAlias)

		if currentOwner, ok := reconciler.usedAliases[alias]; ok && currentOwner != res.Metadata().ID() {
			res.TypedSpec().Value.Error = fmt.Sprintf("requested alias %q is already used by another service", alias)

			logger.Warn(res.TypedSpec().Value.Error)

			return nil
		}
	// if the service
	// - has an explicit alias, but it is not requested anymore, go back to a generated alias
	// - has no alias at all yet, generate a new one
	case hasExplicitAlias || !hasExistingAlias:
		if alias, err = reconciler.generateExposedServiceAlias(); err != nil {
			return fmt.Errorf("error generating exposed service alias: %w", err)
		}
	// keep the existing generated alias
	default:
		alias, _ = res.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
	}

	var serviceURL string

	if serviceURL, err = reconciler.buildExposedServiceURL(alias); err != nil {
		// this error might be caused by an invalid prefix set by the user: save it on the resource, do not crash the controller
		res.TypedSpec().Value.Error = fmt.Sprintf("error building exposed service URL for alias %q: %s", alias, err)

		logger.Warn(res.TypedSpec().Value.Error)

		return nil
	}

	res.Metadata().Labels().Set(omni.LabelExposedServiceAlias, alias)

	// migration code to remove the old, now unused label
	res.Metadata().Labels().Delete(omni.SystemLabelPrefix + "has-generated-exposed-service-alias")

	res.TypedSpec().Value.Port = uint32(port)
	res.TypedSpec().Value.Label = label
	res.TypedSpec().Value.IconBase64 = icon
	res.TypedSpec().Value.Url = serviceURL
	res.TypedSpec().Value.Error = ""
	res.TypedSpec().Value.HasExplicitAlias = explicitAliasRequested

	return nil
}

func (reconciler *Reconciler) buildExposedServiceURL(alias string) (string, error) {
	if alias == "" {
		return "", errors.New("empty alias")
	}

	// - dot ('.') is not allowed in the alias for the URL to not be treated as subdomain
	// - dash ('-') is not allowed, as it is used to separate the alias from the instance name
	const notAllowedChars = ".-"

	if strings.ContainsAny(alias, notAllowedChars) {
		return "", fmt.Errorf("alias contains a not allowed character - one of: %q", notAllowedChars)
	}

	apiURLParts := strings.SplitN(reconciler.advertisedAPIURL, "//", 2)
	if len(apiURLParts) != 2 {
		return "", fmt.Errorf("invalid advertised API URL protocol: %s", reconciler.advertisedAPIURL)
	}

	protocol := apiURLParts[0]
	rest := apiURLParts[1]

	restParts := strings.SplitN(rest, ".", 2)
	if len(restParts) != 2 {
		return "", fmt.Errorf("invalid advertised API URL: %s", reconciler.advertisedAPIURL)
	}

	instanceName := restParts[0]
	rest = restParts[1]

	// example: g3a4ana-demo.proxy-us.omni.siderolabs.io
	serviceURL, err := url.Parse(protocol + "//" + alias + "-" + instanceName + "." + reconciler.workloadProxySubdomain + "." + rest)
	if err != nil {
		return "", fmt.Errorf("error parsing final service URL: %w", err)
	}

	return serviceURL.String(), nil
}

func (reconciler *Reconciler) parseIcon(iconBase64 string) (string, error) {
	if iconBase64 == "" {
		return "", nil
	}

	iconBytes, err := base64.StdEncoding.DecodeString(iconBase64)
	if err != nil {
		return "", fmt.Errorf("error decoding icon: %w", err)
	}

	extractGzip := func(data []byte) ([]byte, error) {
		reader, readerErr := gzip.NewReader(bytes.NewReader(data))
		if readerErr != nil {
			return nil, fmt.Errorf("error creating gzip reader: %w", readerErr)
		}

		defer reader.Close() //nolint:errcheck

		return io.ReadAll(reader)
	}

	extractedBytes, err := extractGzip(iconBytes)
	if err == nil {
		// svg is probably not compressed
		iconBytes = extractedBytes
	}

	isValidXML := func(data []byte) bool {
		decoder := xml.NewDecoder(bytes.NewReader(data))

		for {
			if decodeErr := decoder.Decode(new(any)); decodeErr != nil {
				return errors.Is(decodeErr, io.EOF)
			}
		}
	}

	if !isValidXML(iconBytes) {
		return "", errors.New("icon is not a valid SVG")
	}

	return base64.StdEncoding.EncodeToString(iconBytes), nil
}

func (reconciler *Reconciler) generateExposedServiceAlias() (string, error) {
	attempts := 100

	for range attempts {
		alias := rand.String(6)

		if _, ok := reconciler.usedAliases[alias]; ok {
			continue
		}

		return alias, nil
	}

	return "", fmt.Errorf("failed to generate exposed service alias after %d attempts", attempts)
}
