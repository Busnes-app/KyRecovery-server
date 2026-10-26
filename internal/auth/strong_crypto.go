package auth

import (
	"context"
	"crypto/fips140"
	"errors"
	"net/http"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
)

// Retain the issuer client's connect-time address policy and trust roots.
func oidcHTTPClient() *http.Client {
	base := httpClient()
	if !fips140.Enabled() {
		return base
	}
	client := *base
	transport := base.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	client.Transport = httpsOIDCTransport{base: transport}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return errors.New("OIDC redirects are refused in strong-crypto mode")
	}
	return &client
}

type httpsOIDCTransport struct{ base http.RoundTripper }

func (t httpsOIDCTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := requireOIDCHTTPS(req.URL.String()); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

func requireOIDCHTTPS(endpoints ...string) error {
	for _, raw := range endpoints {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return errors.New("strong-crypto mode requires direct HTTPS OIDC endpoints without URL credentials")
		}
	}
	return nil
}

func discoverProvider(ctx context.Context, issuer string) (*oidc.Provider, error) {
	provider, err := oidc.NewProvider(oidc.ClientContext(ctx, oidcHTTPClient()), issuer)
	if err != nil {
		return nil, err
	}
	if fips140.Enabled() {
		var metadata struct {
			JWKSURI  string `json:"jwks_uri"`
			Userinfo string `json:"userinfo_endpoint"`
		}
		if err := provider.Claims(&metadata); err != nil {
			return nil, err
		}
		if err := requireOIDCHTTPS(provider.Endpoint().AuthURL, provider.Endpoint().TokenURL, metadata.JWKSURI); err != nil {
			return nil, err
		}
		if metadata.Userinfo != "" {
			if err := requireOIDCHTTPS(metadata.Userinfo); err != nil {
				return nil, err
			}
		}
	}
	return provider, nil
}
