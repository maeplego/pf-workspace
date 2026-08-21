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

type Middleware struct {
	devAuth      bool
	issuer       string
	internalBase string
	audience     string
	jwks         jwk.Set
	jwksMu       sync.RWMutex
	jwksLoaded   time.Time
}

func New(devAuth bool, issuer, internalBase, audience string) *Middleware {
	if internalBase == "" {
		internalBase = issuer
	}
	return &Middleware{devAuth: devAuth, issuer: issuer, internalBase: internalBase, audience: audience}
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
	if h := strings.TrimSpace(r.Header.Get("X-Dev-User-Sub")); h != "" && m.devAuth {
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
	if m.issuer == "" {
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
	opts := []jwt.ParseOption{jwt.WithKeySet(set), jwt.WithIssuer(m.issuer)}
	if m.audience != "" {
		opts = append(opts, jwt.WithAudience(m.audience))
	}
	tok, err := jwt.Parse([]byte(token), opts...)
	if err != nil {
		return User{}, err
	}
	sub := tok.Subject()
	if sub == "" {
		return User{}, fmt.Errorf("empty sub")
	}
	u := User{Sub: sub}
	if v, ok := tok.Get("email"); ok {
		if s, ok := v.(string); ok {
			u.Email = strings.ToLower(strings.TrimSpace(s))
		}
	}
	if v, ok := tok.Get("email_verified"); ok {
		if b, ok := v.(bool); ok {
			u.EmailVerified = b
		}
	}
	if v, ok := tok.Get("org_id"); ok {
		if s, ok := v.(string); ok {
			u.OrgID = strings.TrimSpace(s)
		}
	}
	return u, nil
}

func (m *Middleware) authenticateUserInfo(ctx context.Context, token string) (User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.internalBase+"/userinfo", nil)
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
	body, err := io.ReadAll(io.LimitReader(res.Body, 4096))
	if err != nil {
		return User{}, err
	}
	var ui struct {
		Sub           string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		OrgID         string `json:"org_id"`
	}
	if err := json.Unmarshal(body, &ui); err != nil {
		return User{}, err
	}
	if ui.Sub == "" {
		return User{}, fmt.Errorf("empty sub")
	}
	return User{
		Sub:           ui.Sub,
		Email:         strings.ToLower(strings.TrimSpace(ui.Email)),
		EmailVerified: ui.EmailVerified,
		OrgID:         strings.TrimSpace(ui.OrgID),
	}, nil
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
	url := m.internalBase + "/.well-known/jwks.json"
	set, err := jwk.Fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	m.jwks = set
	m.jwksLoaded = time.Now()
	return set, nil
}
