// Copyright (c) 2024 Sidero Labs, Inc.
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
	"github.com/siderolabs/omni/internal/pkg/config"
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
	next            http.Handler
	logger          *zap.Logger
	proxyProvider   ProxyProvider
	accessValidator AccessValidator
	mainURL         *url.URL
	mainDomain      string
}

// NewHTTPHandler creates a new HTTP handler that will proxy requests to the workload proxy.
func NewHTTPHandler(next http.Handler, proxyProvider ProxyProvider, accessValidator AccessValidator, mainURL *url.URL, logger *zap.Logger) (*HTTPHandler, error) {
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

	return &HTTPHandler{
		next:            next,
		proxyProvider:   proxyProvider,
		accessValidator: accessValidator,
		mainURL:         mainURL,
		mainDomain:      getMainDomain(mainURL),
		logger:          logger,
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

	if proxy == nil {
		h.logger.Debug("proxy is nil", zap.String("alias", alias))

		http.NotFound(writer, request)

		return
	}

	h.checkCookies(writer, request, proxy, clusterID)
}

func (h *HTTPHandler) isWorkloadProxyRequest(request *http.Request) bool {
	host, _, _ := net.SplitHostPort(request.Host) //nolint:errcheck

	if host == "" {
		host = request.Host
	}

	return strings.HasPrefix(host, HostPrefix+"-") && strings.HasSuffix(host, "-"+h.mainDomain)
}

func (h *HTTPHandler) checkCookies(writer http.ResponseWriter, request *http.Request, proxy http.Handler, clusterID resource.ID) {
	publicKeyID, publicKeyIDSignatureBase64 := h.getSignatureCookies(request)
	if publicKeyID == "" || publicKeyIDSignatureBase64 == "" {
		h.redirectToLogin(writer, request)

		return
	}

	if err := h.accessValidator.ValidateAccess(request.Context(), publicKeyID, publicKeyIDSignatureBase64, clusterID); err != nil {
		h.logger.Warn("failed to validate access", zap.Error(err))

		forbiddenURL := h.mainURL.JoinPath("/forbidden").String()

		http.Redirect(writer, request, forbiddenURL, http.StatusSeeOther)

		return
	}

	proxy.ServeHTTP(writer, request)
}

// parseServiceAliasFromHost parses the service alias from the request host.
//
// The host will have the pattern: p-<alias>-<instance-name>.<main domain>.
func (h *HTTPHandler) parseServiceAliasFromHost(request *http.Request) (alias string) {
	hostParts := strings.SplitN(request.Host, ".", 2)
	if len(hostParts) == 0 {
		h.logger.Debug("empty proxy service host", zap.String("host", request.Host))

		return ""
	}

	proxyServiceHostPrefixParts := strings.SplitN(hostParts[0], "-", 3)
	if len(proxyServiceHostPrefixParts) < 3 {
		h.logger.Debug("invalid proxy service host prefix: wrong number of parts", zap.String("host", request.Host), zap.Strings("parts", proxyServiceHostPrefixParts))

		return ""
	}

	if proxyServiceHostPrefixParts[0] != HostPrefix {
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
	loginURL, err := url.Parse(config.Config.APIURL)
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

	loginURL.Path = "/omni/authenticate"
	q := loginURL.Query()
	q.Set(auth.RedirectQueryParam, reqURL.String())
	q.Set(auth.FlowQueryParam, auth.ProxyAuthFlow)

	loginURL.RawQuery = q.Encode()

	http.Redirect(writer, request, loginURL.String(), http.StatusSeeOther)
}

func getMainDomain(url *url.URL) string {
	host, _, _ := net.SplitHostPort(url.Host) //nolint:errcheck

	if host != "" {
		return host
	}

	return url.Host
}
