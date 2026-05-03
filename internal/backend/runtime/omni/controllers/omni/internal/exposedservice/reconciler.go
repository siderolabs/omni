// Copyright (c) 2026 Sidero Labs, Inc.
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
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/siderolabs/omni/client/pkg/constants"
	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
)

// dnsLabelRegexp validates an RFC 1123 DNS label: lowercase alphanumeric with optional dashes, no leading/trailing dash, max 63 chars.
var dnsLabelRegexp = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// Reconciler is the reconciler for ExposedService resources.
type Reconciler struct {
	logger                 *zap.Logger
	exposedServices        map[resource.ID]*omni.ExposedService
	usedAliases            map[string]resource.ID
	cluster                string
	workloadProxySubdomain string
	advertisedAPIURL       string
	services               []*corev1.Service
	useOmniSubdomain       bool
}

// NewReconciler creates a new ExposedService reconciler.
//
// The Reconciler is supposed to be used only once.
func NewReconciler(cluster, workloadProxySubdomain, advertisedAPIURL string, useOmniSubdomain bool,
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
		useOmniSubdomain:       useOmniSubdomain,
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

		hostPorts := parseHostPorts(service.Annotations[constants.ExposedServicePortAnnotationKey], logger)

		// Pre-multi-port releases used a bare "<cluster>/<svc>.<ns>" ID. If such a resource still
		// exists for this Service, the matching desired host port keeps that ID so the alias label
		// (and therefore the URL) stays stable across the upgrade. New IDs include the host port.
		legacyID := reconciler.cluster + "/" + svcID

		// If the annotation value is empty, malformed, or every entry is invalid, preserve
		// any existing resources for this Service unchanged. A temporary typo on a working
		// annotation should not tear down the URL — the next valid edit will reconcile it.
		if len(hostPorts) == 0 {
			for id, res := range reconciler.exposedServices {
				if id == legacyID || strings.HasPrefix(id, legacyID+"/") {
					servicesToKeep = append(servicesToKeep, res)
				}
			}

			continue
		}

		multiPort := len(hostPorts) > 1
		legacyRes := reconciler.exposedServices[legacyID]

		for _, port := range hostPorts {
			exposedServiceID := fmt.Sprintf("%s/%d", legacyID, port)
			if legacyRes != nil && int(legacyRes.TypedSpec().Value.Port) == port {
				exposedServiceID = legacyID
			}

			portLogger := logger.With(zap.Int("host_port", port))

			exposedService := reconciler.exposedServices[exposedServiceID]
			if exposedService == nil {
				exposedService = omni.NewExposedService(exposedServiceID)
			}

			if err := reconciler.reconcileService(exposedService, service, port, multiPort, portLogger); err != nil {
				return nil, err
			}

			servicesToKeep = append(servicesToKeep, exposedService)
		}
	}

	return servicesToKeep, nil
}

// parseHostPorts extracts the list of host ports from the annotation value.
//
// The annotation accepts a comma-separated list of entries, where each entry is a bare
// host port or a "host-port:service-port" pair. Only the host port is relevant to Omni
// (the service port part is consumed by kube-service-exposer on the cluster). Bad
// entries are skipped with a warning so that one typo does not invalidate the whole
// annotation. Duplicates are collapsed. The result is sorted ascending so that
// alias-collision tie-breaking ("first port wins") is deterministic.
func parseHostPorts(annotationVal string, logger *zap.Logger) []int {
	seen := make(map[int]struct{})

	var ports []int

	for entry := range strings.SplitSeq(annotationVal, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		hostPortStr, _, _ := strings.Cut(entry, ":")

		port, err := strconv.Atoi(hostPortStr)
		if err != nil || port < 1 || port > 65535 {
			logger.Warn("invalid host port in annotation entry", zap.String("entry", entry))

			continue
		}

		if _, ok := seen[port]; ok {
			continue
		}

		seen[port] = struct{}{}

		ports = append(ports, port)
	}

	slices.Sort(ports)

	return ports
}

// annotationValue returns the value of an annotation, preferring a per-host-port
// suffixed variant ("<baseKey>-<port>") over the unsuffixed one.
func annotationValue(svc *corev1.Service, baseKey string, port int) (string, bool) {
	if v, ok := svc.Annotations[fmt.Sprintf("%s-%d", baseKey, port)]; ok {
		return v, true
	}

	v, ok := svc.Annotations[baseKey]

	return v, ok
}

func (reconciler *Reconciler) reconcileService(exposedService *omni.ExposedService, service *corev1.Service, port int, multiPort bool, logger *zap.Logger) error {
	label, labelOk := annotationValue(service, constants.ExposedServiceLabelAnnotationKey, port)
	if !labelOk {
		label = service.Name + "." + service.Namespace
		if multiPort {
			label = fmt.Sprintf("%s:%d", label, port)
		}
	}

	iconStr, _ := annotationValue(service, constants.ExposedServiceIconAnnotationKey, port)

	icon, err := reconciler.parseIcon(iconStr)
	if err != nil {
		logger.Debug("invalid icon on Service", zap.Error(err))
	}

	explicitAliasOpt := reconciler.resolveExplicitAlias(service, port, multiPort, exposedService.Metadata().ID())

	if err = reconciler.updateExposedService(exposedService, explicitAliasOpt, port, label, icon, logger); err != nil {
		return fmt.Errorf("error updating exposed service: %w", err)
	}

	alias, _ := exposedService.Metadata().Labels().Get(omni.LabelExposedServiceAlias)
	reconciler.usedAliases[alias] = exposedService.Metadata().ID()
	reconciler.exposedServices[exposedService.Metadata().ID()] = exposedService

	return nil
}

// resolveExplicitAlias picks the explicit alias for a (service, host port) pair.
//
// A per-host-port "prefix-<port>" annotation always wins for that port. The unsuffixed
// "prefix" annotation is used as a fallback only when the alias is still free for the
// current resource. In single-port mode that's always the case. In multi-port mode the
// alias is held by whichever resource owns it first: a legacy bare-ID resource being
// reused on upgrade keeps its alias, and otherwise the first port to claim the alias
// wins (which, since iteration is sorted ascending, is the lowest-numbered port). The
// rest fall back to auto-generated aliases. This avoids a noisy "alias already used"
// error in the common case where the user only set one prefix annotation but exposes
// multiple ports.
func (reconciler *Reconciler) resolveExplicitAlias(service *corev1.Service, port int, multiPort bool, exposedServiceID resource.ID) optional.Optional[string] {
	if perPort, ok := service.Annotations[fmt.Sprintf("%s-%d", constants.ExposedServicePrefixAnnotationKey, port)]; ok {
		return optional.Some(perPort)
	}

	base, ok := service.Annotations[constants.ExposedServicePrefixAnnotationKey]
	if !ok {
		return optional.None[string]()
	}

	if !multiPort {
		return optional.Some(base)
	}

	owner, taken := reconciler.usedAliases[strings.ToLower(base)]
	if !taken || owner == exposedServiceID {
		return optional.Some(base)
	}

	return optional.None[string]()
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
		res.TypedSpec().Value.Url = "" // clear the stale URL

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

	// Validate alias as an RFC 1123 DNS label: lowercase alphanumeric with optional dashes,
	// no leading/trailing dash, max 63 chars.
	if !dnsLabelRegexp.MatchString(alias) {
		return "", fmt.Errorf("alias %q is not a valid DNS label (must be lowercase alphanumeric with optional dashes, no leading/trailing dash, max 63 chars)", alias)
	}

	if reconciler.useOmniSubdomain {
		apiURL, err := url.Parse(reconciler.advertisedAPIURL)
		if err != nil {
			return "", fmt.Errorf("invalid advertised API URL: %w", err)
		}

		// Build: <scheme>://<alias>.[<subdomain>.]<omni-host>
		host := apiURL.Host
		if reconciler.workloadProxySubdomain != "" {
			host = reconciler.workloadProxySubdomain + "." + host
		}

		serviceURL := &url.URL{
			Scheme: apiURL.Scheme,
			Host:   alias + "." + host,
		}

		return serviceURL.String(), nil
	}

	// When useOmniSubdomain is false, dashes are additionally forbidden, as they are used to separate the alias from the instance name.
	if strings.Contains(alias, "-") {
		return "", fmt.Errorf("alias %q contains a dash, which is not allowed when useOmniSubdomain is not enabled", alias)
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
