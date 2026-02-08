// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package workloadproxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"go.uber.org/zap"

	"github.com/siderolabs/omni/internal/pkg/auth"
)

// ProxyProvider is a provider of HTTP proxies for the exposed services.
type ProxyProvider interface {
	GetProxy(alias string) (http.Handler, resource.ID, error)
}

// AccessValidator validates workload proxy requests against the given cluster by the given public key ID and its signed & base64'd form.
type AccessValidator interface {
	ValidateAccess(ctx context.Context, publicKeyID, publicKeyIDSignatureBase64 string, clusterID resource.ID) error
}

// HTTPHandler is an HTTP handler that will proxy matching requests to the workload proxy.
//
// It will pass through the requests that don't match.
type HTTPHandler struct {
	next                http.Handler
	logger              *zap.Logger
	proxyProvider       ProxyProvider
	accessValidator     AccessValidator
	mainURL             *url.URL
	mainDomain          string
	workloadProxyDomain string
	redirectKey         []byte
}

// NewHTTPHandler creates a new HTTP handler that will proxy requests to the workload proxy.
func NewHTTPHandler(
	next http.Handler,
	proxyProvider ProxyProvider,
	accessValidator AccessValidator,
	mainURL *url.URL,
	workloadProxySubdomain string,
	logger *zap.Logger,
	redirectKey []byte,
) (*HTTPHandler, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	if proxyProvider == nil {
		return nil, errors.New("proxy provider is nil")
	}

	if accessValidator == nil {
		return nil, errors.New("access validator is nil")
	}

	if mainURL == nil {
		return nil, errors.New("main URL is nil")
	}

	mainDomain := getMainDomain(mainURL)
	workloadProxyDomain := getWorkloadProxyDomain(workloadProxySubdomain, mainDomain)

	return &HTTPHandler{
		next:                next,
		proxyProvider:       proxyProvider,
		accessValidator:     accessValidator,
		mainURL:             mainURL,
		mainDomain:          mainDomain,
		workloadProxyDomain: workloadProxyDomain,
		logger:              logger,
		redirectKey:         redirectKey,
	}, nil
}

// ServeHTTP implements http.Handler.
func (h *HTTPHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !h.isWorkloadProxyRequest(request) {
		h.next.ServeHTTP(writer, request)

		return
	}

	alias := h.parseServiceAliasFromHost(request)
	if alias == "" {
		http.NotFound(writer, request)

		return
	}

	proxy, clusterID, err := h.proxyProvider.GetProxy(alias)
	if err != nil {
		h.logger.Warn("failed to get proxy", zap.Error(err), zap.String("alias", alias))

		http.Error(writer, "failed to get proxy", http.StatusInternalServerError)

		return
	}

	if !h.checkCookies(writer, request, clusterID) {
		return
	}

	if proxy == nil {
		h.logger.Debug("proxy is nil", zap.String("alias", alias))

		http.NotFound(writer, request)

		return
	}

	proxy.ServeHTTP(writer, request)
}

// isWorkloadProxyRequest checks if the request is for the workload proxy.
//
// It supports two formats:
// - Legacy format: p-g3a4ana-demo.omni.siderolabs.io
// - New format with a dedicated subdomain for all workload services: g3a4ana-demo.proxy-us.omni.siderolabs.io.
func (h *HTTPHandler) isWorkloadProxyRequest(request *http.Request) bool {
	host, _, _ := net.SplitHostPort(request.Host) //nolint:errcheck

	if host == "" {
		host = request.Host
	}

	if strings.HasSuffix(host, "."+h.workloadProxyDomain) {
		return true
	}

	// check for the legacy format
	if strings.HasPrefix(host, LegacyHostPrefix+"-") && strings.HasSuffix(host, "-"+h.mainDomain) {
		return true
	}

	return false
}

func (h *HTTPHandler) checkCookies(writer http.ResponseWriter, request *http.Request, clusterID resource.ID) (valid bool) {
	publicKeyID, publicKeyIDSignatureBase64 := h.getSignatureCookies(request)
	if publicKeyID == "" || publicKeyIDSignatureBase64 == "" {
		h.redirectToLogin(writer, request)

		return false
	}

	if err := h.accessValidator.ValidateAccess(request.Context(), publicKeyID, publicKeyIDSignatureBase64, clusterID); err != nil {
		h.logger.Warn("failed to validate access", zap.Error(err))

		forbiddenURL := h.mainURL.JoinPath("/forbidden").String()

		http.Redirect(writer, request, forbiddenURL, http.StatusSeeOther)

		return false
	}

	return true
}

// parseServiceAliasFromHost parses the service alias from the request host.
//
// The host will have the pattern: p-<alias>-<instance-name>.<main domain>.
func (h *HTTPHandler) parseServiceAliasFromHost(request *http.Request) string {
	hostParts := strings.SplitN(request.Host, ".", 2)
	if len(hostParts) == 0 {
		h.logger.Debug("empty proxy service host", zap.String("host", request.Host))

		return ""
	}

	proxyServiceHostPrefixParts := strings.SplitN(hostParts[0], "-", 3)
	if len(proxyServiceHostPrefixParts) < 2 {
		h.logger.Debug("invalid proxy service host prefix: wrong number of parts", zap.String("host", request.Host), zap.Strings("parts", proxyServiceHostPrefixParts))

		return ""
	}

	if isNewFormat := proxyServiceHostPrefixParts[0] != LegacyHostPrefix; isNewFormat {
		return proxyServiceHostPrefixParts[0]
	}

	// handle legacy format
	if proxyServiceHostPrefixParts[0] != LegacyHostPrefix {
		h.logger.Debug("invalid proxy service host prefix: doesn't start with the prefix", zap.String("host", request.Host), zap.Strings("parts", proxyServiceHostPrefixParts))

		return ""
	}

	return proxyServiceHostPrefixParts[1]
}

func (h *HTTPHandler) getSignatureCookies(request *http.Request) (publicKeyID string, publicKeyIDSignatureBase64 string) {
	for _, cookie := range request.Cookies() {
		switch cookie.Name {
		case PublicKeyIDCookie:
			publicKeyID = cookie.Value
		case PublicKeyIDSignatureBase64Cookie:
			publicKeyIDSignatureBase64 = cookie.Value
		}

		if publicKeyID != "" && publicKeyIDSignatureBase64 != "" {
			break
		}
	}

	return publicKeyID, publicKeyIDSignatureBase64
}

func (h *HTTPHandler) redirectToLogin(writer http.ResponseWriter, request *http.Request) {
	loginURL, err := url.Parse(h.mainURL.String())
	if err != nil {
		h.logger.Warn("failed to redirect to login", zap.Error(err))

		http.Error(writer, "failed to redirect to login", http.StatusInternalServerError)

		return
	}

	reqURL := *request.URL
	reqURL.Scheme = "https"
	reqURL.Host = request.Host

	if reqURL.Port() == "" && loginURL.Port() != "" {
		reqURL.Host = fmt.Sprintf("%s:%s", request.Host, loginURL.Port())
	}

	loginURL.Path = "/authenticate"
	q := loginURL.Query()
	q.Set(auth.RedirectQueryParam, EncodeRedirectURL(reqURL.String(), h.redirectKey))
	q.Set(auth.FlowQueryParam, auth.ProxyAuthFlow)

	loginURL.RawQuery = q.Encode()

	h.logger.Info("redirect workload proxy request to login",
		zap.Stringer("request_url", request.URL),
		zap.Stringer("login_url", loginURL))

	http.Redirect(writer, request, loginURL.String(), http.StatusSeeOther)
}

// getMainDomain returns the main domain from the given URL.
//
// Example: demo.omni.siderolabs.io.
func getMainDomain(url *url.URL) string {
	host, _, _ := net.SplitHostPort(url.Host) //nolint:errcheck

	if host != "" {
		return host
	}

	return url.Host
}

// getWorkloadProxyDomain returns the full domain used by the workload proxy as the parent domain.
//
// Example: proxy-us.omni.siderolabs.io.
func getWorkloadProxyDomain(subdomain string, mainDomain string) string {
	_, right, ok := strings.Cut(mainDomain, ".")
	if !ok {
		return ""
	}

	return subdomain + "." + right
}
