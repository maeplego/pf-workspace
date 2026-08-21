package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

type ctxKey struct{}

type User struct {
	Sub           string
	Email         string
	EmailVerified bool
	OrgID         string
}

func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// Options configures OIDC AuthPort (bundled P01 or BYO Auth0 / Entra / Keycloak).
type Options struct {
	DevAuth      bool
	Issuer       string
	InternalBase string
	Audience     string
	Claims       ClaimNames
	// SkipIssuerCheck allows multi-tenant issuers that mint tokens with account-specific iss
	// (rare; prefer exact OIDC_ISSUER). Empty audience still skips aud validation.
	SkipIssuerCheck bool
}

type Middleware struct {
	opts         Options
	jwks         jwk.Set
	jwksMu       sync.RWMutex
	jwksLoaded   time.Time
	jwksURI      string
	userinfoURL  string
	discoveryMu  sync.Mutex
	discoveredAt time.Time
}

func New(devAuth bool, issuer, internalBase, audience string) *Middleware {
	return NewWithOptions(Options{
		DevAuth:      devAuth,
		Issuer:       issuer,
		InternalBase: internalBase,
		Audience:     audience,
		Claims:       DefaultClaimNames(),
	})
}

func NewWithOptions(opts Options) *Middleware {
	if opts.InternalBase == "" {
		opts.InternalBase = opts.Issuer
	}
	if opts.Claims.OrgID == "" {
		opts.Claims = DefaultClaimNames()
	}
	return &Middleware{opts: opts}
}

func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, err := m.authenticate(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
	})
}

func (m *Middleware) authenticate(r *http.Request) (User, error) {
	if h := strings.TrimSpace(r.Header.Get("X-Dev-User-Sub")); h != "" && m.opts.DevAuth {
		email := strings.TrimSpace(r.Header.Get("X-Dev-User-Email"))
		orgID := strings.TrimSpace(r.Header.Get("X-Dev-User-Org"))
		return User{Sub: h, Email: email, EmailVerified: email != "", OrgID: orgID}, nil
	}
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authz, "Bearer ") {
		return User{}, fmt.Errorf("missing bearer")
	}
	token := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	if token == "" {
		return User{}, fmt.Errorf("missing bearer")
	}
	if m.opts.Issuer == "" {
		return User{}, fmt.Errorf("oidc not configured")
	}
	if u, err := m.authenticateJWT(r.Context(), token); err == nil {
		if org := strings.TrimSpace(r.Header.Get("X-Workspace-Org")); org != "" {
			u.OrgID = org
		}
		return u, nil
	}
	u, err := m.authenticateUserInfo(r.Context(), token)
	if err != nil {
		return User{}, err
	}
	if org := strings.TrimSpace(r.Header.Get("X-Workspace-Org")); org != "" {
		u.OrgID = org
	}
	return u, nil
}

func (m *Middleware) authenticateJWT(ctx context.Context, token string) (User, error) {
	set, err := m.jwksSet(ctx)
	if err != nil {
		return User{}, err
	}
	opts := []jwt.ParseOption{jwt.WithKeySet(set)}
	if !m.opts.SkipIssuerCheck {
		opts = append(opts, jwt.WithIssuer(m.opts.Issuer))
	}
	if m.opts.Audience != "" {
		opts = append(opts, jwt.WithAudience(m.opts.Audience))
	}
	tok, err := jwt.Parse([]byte(token), opts...)
	if err != nil {
		return User{}, err
	}
	sub := tok.Subject()
	if sub == "" {
		return User{}, fmt.Errorf("empty sub")
	}
	claims := map[string]any{}
	for k, v := range tok.PrivateClaims() {
		claims[k] = v
	}
	if v, ok := tok.Get("email"); ok {
		claims["email"] = v
	}
	if v, ok := tok.Get("email_verified"); ok {
		claims["email_verified"] = v
	}
	u := User{Sub: sub, OrgID: orgIDFromClaims(claims, m.opts.Claims)}
	if v, ok := claims["email"].(string); ok {
		u.Email = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := claims["email_verified"].(bool); ok {
		u.EmailVerified = v
	}
	return u, nil
}

func (m *Middleware) authenticateUserInfo(ctx context.Context, token string) (User, error) {
	userinfoURL, err := m.userinfoEndpoint(ctx)
	if err != nil {
		return User{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userinfoURL, nil)
	if err != nil {
		return User{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return User{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return User{}, fmt.Errorf("userinfo %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 8192))
	if err != nil {
		return User{}, err
	}
	var claims map[string]any
	if err := json.Unmarshal(body, &claims); err != nil {
		return User{}, err
	}
	sub, _ := claims["sub"].(string)
	if strings.TrimSpace(sub) == "" {
		return User{}, fmt.Errorf("empty sub")
	}
	u := User{
		Sub:   sub,
		OrgID: orgIDFromClaims(claims, m.opts.Claims),
	}
	if v, ok := claims["email"].(string); ok {
		u.Email = strings.ToLower(strings.TrimSpace(v))
	}
	if v, ok := claims["email_verified"].(bool); ok {
		u.EmailVerified = v
	}
	return u, nil
}

func (m *Middleware) ensureDiscovery(ctx context.Context) error {
	m.discoveryMu.Lock()
	defer m.discoveryMu.Unlock()
	if m.jwksURI != "" && m.userinfoURL != "" && time.Since(m.discoveredAt) < 10*time.Minute {
		return nil
	}
	base := strings.TrimRight(m.opts.InternalBase, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/.well-known/openid-configuration", nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		// Fall back to P01-shaped paths when discovery is unavailable (tests / offline).
		m.jwksURI = base + "/.well-known/jwks.json"
		m.userinfoURL = base + "/userinfo"
		m.discoveredAt = time.Now()
		return nil
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		m.jwksURI = base + "/.well-known/jwks.json"
		m.userinfoURL = base + "/userinfo"
		m.discoveredAt = time.Now()
		return nil
	}
	var doc struct {
		JWKSURI           string `json:"jwks_uri"`
		UserinfoEndpoint  string `json:"userinfo_endpoint"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&doc); err != nil {
		return err
	}
	if doc.JWKSURI != "" {
		m.jwksURI = doc.JWKSURI
	} else {
		m.jwksURI = base + "/.well-known/jwks.json"
	}
	if doc.UserinfoEndpoint != "" {
		m.userinfoURL = doc.UserinfoEndpoint
	} else {
		m.userinfoURL = base + "/userinfo"
	}
	m.discoveredAt = time.Now()
	return nil
}

func (m *Middleware) userinfoEndpoint(ctx context.Context) (string, error) {
	if err := m.ensureDiscovery(ctx); err != nil {
		return "", err
	}
	return m.userinfoURL, nil
}

func (m *Middleware) jwksSet(ctx context.Context) (jwk.Set, error) {
	m.jwksMu.RLock()
	if m.jwks != nil && time.Since(m.jwksLoaded) < 5*time.Minute {
		defer m.jwksMu.RUnlock()
		return m.jwks, nil
	}
	m.jwksMu.RUnlock()

	m.jwksMu.Lock()
	defer m.jwksMu.Unlock()
	if m.jwks != nil && time.Since(m.jwksLoaded) < 5*time.Minute {
		return m.jwks, nil
	}
	if err := m.ensureDiscovery(ctx); err != nil {
		return nil, err
	}
	set, err := jwk.Fetch(ctx, m.jwksURI)
	if err != nil {
		return nil, err
	}
	m.jwks = set
	m.jwksLoaded = time.Now()
	return set, nil
}
