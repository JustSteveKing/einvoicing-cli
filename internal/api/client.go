// Package api is a small client for the einvoicing.dev API, covering exactly
// the operations in openapi.yaml.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the production API.
const DefaultBaseURL = "https://api.einvoicing.dev"

type Client struct {
	BaseURL   string
	Key       string
	UserAgent string
	HTTP      *http.Client
}

func New(baseURL, key, userAgent string) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		Key:       key,
		UserAgent: userAgent,
		HTTP:      &http.Client{Timeout: 60 * time.Second},
	}
}

// SignInChallenge returns the ALTCHA challenge exactly as sent, since it has
// to be echoed back verbatim once solved.
func (c *Client) SignInChallenge(ctx context.Context) (json.RawMessage, error) {
	var challenge json.RawMessage
	err := c.do(ctx, http.MethodPost, "/v1/sign-in-challenges", nil, "", &challenge)
	return challenge, err
}

func (c *Client) RequestSignIn(ctx context.Context, email, altcha string) (SignIn, error) {
	var out SignIn
	err := c.doJSON(ctx, http.MethodPost, "/v1/sign-ins", map[string]string{"email": email, "altcha": altcha}, &out)
	return out, err
}

func (c *Client) ConfirmSignIn(ctx context.Context, signInID, code, keyName, mode string) (SignInResult, error) {
	var out SignInResult
	body := map[string]any{"code": code, "key": map[string]string{"name": keyName, "mode": mode}}
	err := c.doJSON(ctx, http.MethodPost, "/v1/sign-ins/"+url.PathEscape(signInID)+"/confirmation", body, &out)
	return out, err
}

func (c *Client) Account(ctx context.Context) (Account, error) {
	var out Account
	err := c.do(ctx, http.MethodGet, "/v1/account", nil, "", &out)
	return out, err
}

func (c *Client) Usage(ctx context.Context) (Usage, error) {
	var out Usage
	err := c.do(ctx, http.MethodGet, "/v1/usage", nil, "", &out)
	return out, err
}

func (c *Client) Participant(ctx context.Context, id string) (Participant, error) {
	var out Participant
	err := c.do(ctx, http.MethodGet, "/v1/participants/"+url.PathEscape(id), nil, "", &out)
	return out, err
}

func (c *Client) Rulesets(ctx context.Context) ([]Ruleset, error) {
	var out []Ruleset
	err := c.do(ctx, http.MethodGet, "/v1/rulesets", nil, "", &out)
	return out, err
}

func (c *Client) Validate(ctx context.Context, document []byte, ruleset string) (ValidationReport, error) {
	path := "/v1/validations"
	if ruleset != "" {
		path += "?ruleset=" + url.QueryEscape(ruleset)
	}
	var out ValidationReport
	err := c.do(ctx, http.MethodPost, path, bytes.NewReader(document), "application/xml", &out)
	return out, err
}

// Convert sends a ConversionRequest as-is.
func (c *Client) Convert(ctx context.Context, request []byte) (Conversion, error) {
	var out Conversion
	err := c.do(ctx, http.MethodPost, "/v1/conversions", bytes.NewReader(request), "application/json", &out)
	return out, err
}

func (c *Client) Keys(ctx context.Context) ([]APIKey, error) {
	var out []APIKey
	err := c.do(ctx, http.MethodGet, "/v1/keys", nil, "", &out)
	return out, err
}

func (c *Client) CreateKey(ctx context.Context, name, mode string, expiresAt string) (NewAPIKey, error) {
	body := map[string]any{"name": name, "mode": mode}
	if expiresAt != "" {
		body["expires_at"] = expiresAt
	}
	var out NewAPIKey
	err := c.doJSON(ctx, http.MethodPost, "/v1/keys", body, &out)
	return out, err
}

func (c *Client) RevokeKey(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/keys/"+url.PathEscape(id), nil, "", nil)
}

func (c *Client) CheckoutSession(ctx context.Context, plan string) (BillingLink, error) {
	var out BillingLink
	err := c.doJSON(ctx, http.MethodPost, "/v1/billing/checkout-sessions", map[string]string{"plan": plan}, &out)
	return out, err
}

func (c *Client) PortalSession(ctx context.Context) (BillingLink, error) {
	var out BillingLink
	err := c.do(ctx, http.MethodPost, "/v1/billing/portal-sessions", nil, "", &out)
	return out, err
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return c.do(ctx, method, path, bytes.NewReader(encoded), "application/json", out)
}

// do sends a request and decodes the `data` envelope into out, or returns a
// *Problem for any error status.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if c.Key != "" {
		req.Header.Set("Authorization", "Bearer "+c.Key)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		problem := &Problem{Status: resp.StatusCode, RetryAfter: resp.Header.Get("Retry-After")}
		if json.Unmarshal(raw, problem) != nil || problem.Title == "" {
			problem.Title = fmt.Sprintf("The API answered %d.", resp.StatusCode)
		}
		return problem
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data == nil {
		return errors.New("the API returned a response this client does not understand")
	}

	return json.Unmarshal(envelope.Data, out)
}
