package zeroone

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ErrRejected means 01Edu refused the login or password.
var ErrRejected = errors.New("01Edu rejected the sign-in")

const defaultSchoolBaseURL = "https://01.nexteraeducation.net"

// SchoolBaseURL is the official site origin used to check a password.
func SchoolBaseURL() string {
	base := strings.TrimRight(strings.TrimSpace(defaultSchoolBaseURL), "/")
	if env := strings.TrimRight(strings.TrimSpace(os.Getenv("ZEROONE_BASE_URL")), "/"); env != "" {
		base = env
	}
	return base
}

// VerifySignIn checks a login and password with the official 01Edu sign-in.
// The returned session token is for a follow-up read. The password is not included in errors.
func VerifySignIn(ctx context.Context, client *http.Client, baseURL, login, password string) (string, error) {
	login = strings.TrimSpace(login)
	if login == "" || password == "" || len(login) > 200 || len(password) > 200 {
		return "", ErrRejected
	}
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", errors.New("invalid 01Edu sign-in URL")
	}
	if u.Scheme != "https" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" {
		return "", errors.New("01Edu sign-in requires HTTPS outside localhost")
	}
	endpoint := u.Scheme + "://" + u.Host + "/api/auth/signin"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(login, password)
	req.Header.Set("Accept", "application/json")
	if client == nil {
		client = &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("01Edu sign-in request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("01Edu sign-in response: %w", err)
	}
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		token := sessionToken(raw)
		if token == "" {
			return "", errors.New("01Edu sign-in did not return a session")
		}
		return token, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return "", ErrRejected
	default:
		return "", fmt.Errorf("01Edu sign-in HTTP status %d", resp.StatusCode)
	}
}

func sessionToken(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	text = strings.Trim(text, `"`)
	if strings.HasPrefix(text, "eyJ") {
		return text
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	for _, key := range []string{"token", "jwt", "access_token"} {
		if value, ok := body[key].(string); ok && strings.HasPrefix(value, "eyJ") {
			return value
		}
	}
	return ""
}
