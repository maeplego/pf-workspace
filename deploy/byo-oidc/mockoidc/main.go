package main

// Minimal OIDC IdP for BYO Collab labs. Not for production.
// Issues tokens with sub, email, org_id, and organizations[] for AuthPort demos.

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type codeRow struct {
	Sub, Email, OrgID, Redirect, Challenge string
	Orgs                                   []map[string]string
}

var (
	priv    *rsa.PrivateKey
	codes   = map[string]codeRow{}
	codesMu sync.Mutex
	issuer  = "http://127.0.0.1:5556"
)

func main() {
	var err error
	priv, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}
	if v := strings.TrimRight(strings.TrimSpace(os.Getenv("MOCK_OIDC_ISSUER")), "/"); v != "" {
		issuer = v
	}
	addr := ":5556"
	if v := strings.TrimSpace(os.Getenv("MOCK_OIDC_ADDR")); v != "" {
		addr = v
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("GET /.well-known/openid-configuration", discovery)
	mux.HandleFunc("GET /.well-known/jwks.json", jwks)
	mux.HandleFunc("GET /jwks", jwks)
	mux.HandleFunc("GET /authorize", authorize)
	mux.HandleFunc("POST /token", token)
	mux.HandleFunc("GET /userinfo", userinfo)
	log.Printf("mock oidc listening on %s issuer=%s", addr, issuer)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/authorize",
		"token_endpoint":                        issuer + "/token",
		"userinfo_endpoint":                     issuer + "/userinfo",
		"jwks_uri":                              issuer + "/.well-known/jwks.json",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      []string{"openid", "profile", "email", "org", "offline_access"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"code_challenge_methods_supported":      []string{"S256"},
	})
}

func jwks(w http.ResponseWriter, _ *http.Request) {
	n := base64.RawURLEncoding.EncodeToString(priv.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.E)).Bytes())
	writeJSON(w, map[string]any{
		"keys": []map[string]string{{
			"kty": "RSA", "kid": "mock-1", "use": "sig", "alg": "RS256", "n": n, "e": e,
		}},
	})
}

func authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	redirect := q.Get("redirect_uri")
	if redirect == "" {
		http.Error(w, "redirect_uri required", http.StatusBadRequest)
		return
	}
	code := randomID(16)
	orgA := envOr("MOCK_ORG_A", "org-byo-a")
	orgB := envOr("MOCK_ORG_B", "org-byo-b")
	codesMu.Lock()
	codes[code] = codeRow{
		Sub:       envOr("MOCK_SUB", "byo-user-1"),
		Email:     envOr("MOCK_EMAIL", "byo@example.test"),
		OrgID:     orgA,
		Redirect:  redirect,
		Challenge: q.Get("code_challenge"),
		Orgs: []map[string]string{
			{"org_id": orgA, "org_name": "BYO Org A", "role": "owner"},
			{"org_id": orgB, "org_name": "BYO Org B", "role": "member"},
		},
	}
	codesMu.Unlock()
	u, err := url.Parse(redirect)
	if err != nil {
		http.Error(w, "bad redirect", http.StatusBadRequest)
		return
	}
	qq := u.Query()
	qq.Set("code", code)
	qq.Set("state", q.Get("state"))
	u.RawQuery = qq.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	code := r.FormValue("code")
	codesMu.Lock()
	row, ok := codes[code]
	if ok {
		delete(codes, code)
	}
	codesMu.Unlock()
	if !ok {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
		return
	}
	access, err := sign(row, false)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	idTok, err := sign(row, true)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{
		"access_token":  access,
		"id_token":      idTok,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": "mock-refresh-" + randomID(8),
	})
}

func userinfo(w http.ResponseWriter, r *http.Request) {
	authz := r.Header.Get("Authorization")
	if !strings.HasPrefix(authz, "Bearer ") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	tok, err := jwt.Parse(strings.TrimPrefix(authz, "Bearer "), func(t *jwt.Token) (any, error) {
		return &priv.PublicKey, nil
	})
	if err != nil || !tok.Valid {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	claims, _ := tok.Claims.(jwt.MapClaims)
	writeJSON(w, claims)
}

func sign(row codeRow, idToken bool) (string, error) {
	claims := jwt.MapClaims{
		"iss":   issuer,
		"sub":   row.Sub,
		"aud":   "pf-workspace-web",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"iat":   time.Now().Unix(),
		"email": row.Email,
		"name":  "BYO Demo",
		"org_id": row.OrgID,
		"organizations": row.Orgs,
	}
	if idToken {
		claims["email_verified"] = true
	}
	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	t.Header["kid"] = "mock-1"
	return t.SignedString(priv)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func randomID(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func envOr(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
