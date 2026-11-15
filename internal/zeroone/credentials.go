package zeroone

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Credential modes for the server-side 01Edu read credential.
const (
	ModeJWT      = "jwt"       // ZEROONE_ACCESS_TOKEN already holds a Hasura JWT
	ModeAppToken = "app-token" // ZEROONE_ACCESS_TOKEN holds a Gitea application token exchanged at /api/auth/token
	ModePassword = "password"  // ZEROONE_LOGIN + ZEROONE_PASSWORD exchanged at /api/auth/signin
)

// refreshBefore renews an exchanged JWT this long before it expires.
const refreshBefore = 5 * time.Minute

// Credentials is the server-side credential used for the scheduled live sync.
// None of these values may be logged, returned by the API, or sent to the browser.
type Credentials struct {
	BaseURL     string
	GraphQLURL  string
	AccessToken string
	Login       string
	Password    string
	Role        string
}

// Mode reports which credential will be used, or "" when none is configured.
func (c Credentials) Mode() string {
	token := strings.TrimSpace(c.AccessToken)
	switch {
	case token != "" && looksLikeJWT(token):
		return ModeJWT
	case token != "":
		return ModeAppToken
	case strings.TrimSpace(c.Login) != "" && c.Password != "":
		return ModePassword
	default:
		return ""
	}
}

// Configured reports whether a live-sync credential is present.
func (c Credentials) Configured() bool { return c.Mode() != "" }

// CredentialsFromEnv reads the live-sync credential from the process environment.
//
//	ZEROONE_BASE_URL       school origin (default https://01.nexteraeducation.net)
//	ZEROONE_GRAPHQL_URL    Hasura endpoint (default <base>/api/graphql-engine/v1/graphql)
//	ZEROONE_ACCESS_TOKEN   Gitea application token (preferred) or a ready Hasura JWT
//	ZEROONE_LOGIN          service account login (used only when no access token is set)
//	ZEROONE_PASSWORD       service account password
//	ZEROONE_HASURA_ROLE    optional x-hasura-role; sent only when the JWT allows it
func CredentialsFromEnv() Credentials {
	base := SchoolBaseURL()
	graphql := strings.TrimSpace(os.Getenv("ZEROONE_GRAPHQL_URL"))
	if graphql == "" {
		graphql = base + "/api/graphql-engine/v1/graphql"
	}
	return Credentials{
		BaseURL:     base,
		GraphQLURL:  graphql,
		AccessToken: strings.TrimSpace(os.Getenv("ZEROONE_ACCESS_TOKEN")),
		Login:       strings.TrimSpace(os.Getenv("ZEROONE_LOGIN")),
		Password:    os.Getenv("ZEROONE_PASSWORD"),
		Role:        strings.TrimSpace(os.Getenv("ZEROONE_HASURA_ROLE")),
	}
}

// TokenSource turns the configured credential into a current Hasura JWT and
// renews it before it expires. It is safe for concurrent use.
type TokenSource struct {
	creds      Credentials
	httpClient *http.Client

	mu      sync.Mutex
	jwt     string
	expires time.Time
	role    string
}

// NewTokenSource validates the credential and base URL. It does not call 01Edu.
func NewTokenSource(creds Credentials, httpClient *http.Client) (*TokenSource, error) {
	if !creds.Configured() {
		return nil, errors.New("no 01Edu live-sync credential configured")
	}
	if _, err := schoolOrigin(creds.BaseURL); err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 20 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &TokenSource{creds: creds, httpClient: httpClient}, nil
}

// Mode reports which credential type is in use.
func (t *TokenSource) Mode() string { return t.creds.Mode() }

// Token returns a JWT that is valid for at least a few more minutes.
func (t *TokenSource) Token(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.jwt != "" && (t.expires.IsZero() || time.Until(t.expires) > refreshBefore) {
		return t.jwt, nil
	}
	var (
		jwt string
		err error
	)
	switch t.creds.Mode() {
	case ModeJWT:
		jwt = t.creds.AccessToken
	case ModeAppToken:
		jwt, err = ExchangeAppToken(ctx, t.httpClient, t.creds.BaseURL, t.creds.AccessToken)
	case ModePassword:
		jwt, err = VerifySignIn(ctx, t.httpClient, t.creds.BaseURL, t.creds.Login, t.creds.Password)
		if errors.Is(err, ErrRejected) {
			err = errors.New("01Edu rejected ZEROONE_LOGIN/ZEROONE_PASSWORD")
		}
	default:
		err = errors.New("no 01Edu live-sync credential configured")
	}
	if err != nil {
		return "", err
	}
	_, _, expires, _ := SessionAccess(jwt)
	if !expires.IsZero() && time.Now().After(expires) {
		return "", errors.New("the configured 01Edu JWT has expired; set a Gitea app token in ZEROONE_ACCESS_TOKEN so it can be renewed")
	}
	t.jwt = jwt
	t.expires = expires
	t.role = chooseRole(t.creds.Role, jwt)
	return jwt, nil
}

// Role is the x-hasura-role to send with the current JWT ("" uses the JWT default role).
func (t *TokenSource) Role() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.role
}

// AllowedRoles lists the Hasura roles of the current JWT (empty before the first Token call).
func (t *TokenSource) AllowedRoles() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return AllowedRoles(t.jwt)
}

// Expires is the current JWT expiry, zero when unknown.
func (t *TokenSource) Expires() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.expires
}

// Invalidate forgets the cached JWT so the next Token call renews it.
func (t *TokenSource) Invalidate() {
	t.mu.Lock()
	t.jwt = ""
	t.expires = time.Time{}
	t.mu.Unlock()
}

// CanReadCohort reports whether the current JWT carries a staff role that can
// read other students. A plain `user` JWT only returns its owner's rows.
func (t *TokenSource) CanReadCohort() bool {
	for _, role := range t.AllowedRoles() {
		if CohortProgressRole(role) {
			return true
		}
	}
	return false
}

// chooseRole picks the x-hasura-role header. An explicit role is honoured only
// when the JWT lists it (Hasura rejects anything else). Otherwise a staff
// read role is preferred over the JWT default so cohort-wide reads work.
func chooseRole(requested, jwt string) string {
	allowed := AllowedRoles(jwt)
	if requested = strings.TrimSpace(requested); requested != "" {
		for _, role := range allowed {
			if role == requested {
				return role
			}
		}
		return ""
	}
	preference := []string{"admin_read_only", "admin-read-only", "campus_admin_read_only", "campus-admin-read-only", "campus_admin", "campus-admin", "admin"}
	for _, want := range preference {
		for _, role := range allowed {
			if role == want {
				return role
			}
		}
	}
	return ""
}

// ExchangeAppToken trades a Gitea application token for a Hasura JWT using the
// platform's GET /api/auth/token?token=... endpoint.
func ExchangeAppToken(ctx context.Context, client *http.Client, baseURL, appToken string) (string, error) {
	appToken = strings.TrimSpace(appToken)
	if appToken == "" || len(appToken) > 512 {
		return "", errors.New("invalid 01Edu application token")
	}
	origin, err := schoolOrigin(baseURL)
	if err != nil {
		return "", err
	}
	endpoint := origin + "/api/auth/token?token=" + url.QueryEscape(appToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		// The URL carries the token; never surface it in an error.
		return "", errors.New("01Edu token exchange request failed")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", errors.New("01Edu token exchange response could not be read")
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		token := sessionToken(raw)
		if token == "" {
			return "", errors.New("01Edu token exchange did not return a JWT")
		}
		return token, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", errors.New("01Edu rejected ZEROONE_ACCESS_TOKEN (token exchange returned 401/403)")
	default:
		return "", fmt.Errorf("01Edu token exchange HTTP status %d", resp.StatusCode)
	}
}

func schoolOrigin(baseURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", errors.New("invalid ZEROONE_BASE_URL")
	}
	if u.Scheme != "https" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" {
		return "", errors.New("ZEROONE_BASE_URL must use HTTPS outside localhost")
	}
	return u.Scheme + "://" + u.Host, nil
}

func looksLikeJWT(token string) bool {
	return strings.HasPrefix(token, "eyJ") && strings.Count(token, ".") == 2
}
