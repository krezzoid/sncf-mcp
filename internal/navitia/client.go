package navitia

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the SNCF coverage endpoint of the Navitia API.
const DefaultBaseURL = "https://api.sncf.com/v1/coverage/sncf"

// Client is a typed, context-aware client for the SNCF/Navitia API.
//
// It is intentionally decoupled from anything MCP-related so it can be unit
// tested in isolation against an httptest server (see client_test.go).
type Client struct {
	baseURL     string
	apiKey      string
	http        *http.Client
	maxRetries  int
	baseBackoff time.Duration
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the API base URL (used by tests to point at httptest).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithHTTPClient injects a custom *http.Client (timeouts, transport, etc.).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}

// WithRetry configures retry behavior for transient failures (HTTP 429 and
// 5xx). maxRetries is the number of retries after the initial attempt, and
// baseBackoff is the first backoff delay, doubled on each subsequent retry and
// capped internally. A server-provided Retry-After header takes precedence.
// Pass maxRetries = 0 to disable retries (used by tests for the no-retry path).
func WithRetry(maxRetries int, baseBackoff time.Duration) Option {
	return func(c *Client) {
		c.maxRetries = maxRetries
		c.baseBackoff = baseBackoff
	}
}

// New builds a Client. The API key is required; it is sent as the HTTP Basic
// username with an empty password, per the SNCF integration guide.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:     DefaultBaseURL,
		apiKey:      apiKey,
		http:        &http.Client{Timeout: 15 * time.Second},
		maxRetries:  3,
		baseBackoff: 500 * time.Millisecond,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// APIError is returned for non-2xx responses from the API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("navitia: unexpected status %d: %s", e.StatusCode, e.Body)
}

// maxBackoff caps the exponential backoff between retries.
const maxBackoff = 10 * time.Second

// get performs an authenticated GET against path?query and decodes the JSON
// body into out. path is relative to the base URL, e.g. "/places".
//
// Transient failures (HTTP 429 and 5xx) are retried with backoff per the
// client's retry configuration; a Retry-After header is honored when present.
// Other non-2xx responses (e.g. 4xx) are returned immediately as *APIError.
func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	u := c.baseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}

	var lastErr error
	for attempt := 0; ; attempt++ {
		body, status, retryAfter, err := c.do(ctx, u)
		switch {
		case err != nil:
			lastErr = err
		case status == http.StatusTooManyRequests || (status >= 500 && status <= 599):
			lastErr = &APIError{StatusCode: status, Body: string(body)}
		case status < 200 || status >= 300:
			// Other 4xx are client errors; retrying will not help.
			return &APIError{StatusCode: status, Body: string(body)}
		default:
			if err := json.Unmarshal(body, out); err != nil {
				return fmt.Errorf("navitia: decode response: %w", err)
			}
			return nil
		}

		if attempt >= c.maxRetries {
			return lastErr
		}
		if err := sleepCtx(ctx, backoff(c.baseBackoff, attempt, retryAfter)); err != nil {
			return err
		}
	}
}

// do issues a single authenticated GET and returns the (length-limited) body,
// the status code, and any Retry-After delay advertised by the server.
func (c *Client) do(ctx context.Context, u string) (body []byte, status int, retryAfter time.Duration, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("navitia: build request: %w", err)
	}
	// SNCF/Navitia Basic auth: username = API key, password = empty.
	req.SetBasicAuth(c.apiKey, "")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("navitia: do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ = io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return body, resp.StatusCode, parseRetryAfter(resp.Header.Get("Retry-After")), nil
}

// backoff returns how long to wait before the next retry. A positive
// Retry-After wins; otherwise the delay grows exponentially from base
// (base, 2*base, 4*base, ...), is capped at maxBackoff, and gets "equal jitter"
// (half fixed, half random) so retries don't synchronize. See ADR-0006.
func backoff(base time.Duration, attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	exp := base << attempt
	if exp <= 0 || exp > maxBackoff { // exp <= 0 guards against shift overflow
		exp = maxBackoff
	}
	half := exp / 2
	return half + time.Duration(rand.Int64N(int64(half)+1)) //nolint:gosec // backoff jitter, not security-sensitive
}

// sleepCtx waits for d or until ctx is done, returning ctx.Err() if cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// parseRetryAfter interprets a Retry-After header, which may be an integer
// number of seconds or an HTTP date. It returns 0 when absent or invalid.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// Places resolves a free-text query (e.g. "Paris") into candidate locations,
// biased toward stop areas. This replaces the static-CSV approach used by the
// existing community servers (see ADR-0001).
func (c *Client) Places(ctx context.Context, query string) (*PlacesResponse, error) {
	q := url.Values{}
	q.Set("q", query)
	q.Add("type[]", "stop_area")

	var out PlacesResponse
	if err := c.get(ctx, "/places", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Journeys computes itineraries between two locations. `from` and `to` are
// Navitia IDs (resolve names via Places first) or "lon;lat" coordinates.
// `when` may be the zero time, in which case the API defaults to "now".
func (c *Client) Journeys(ctx context.Context, from, to string, when time.Time, arrival bool) (*JourneysResponse, error) {
	q := url.Values{}
	q.Set("from", from)
	q.Set("to", to)
	if !when.IsZero() {
		q.Set("datetime", when.Format(NavitiaTime))
		if arrival {
			q.Set("datetime_represents", "arrival")
		} else {
			q.Set("datetime_represents", "departure")
		}
	}

	var out JourneysResponse
	if err := c.get(ctx, "/journeys", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Departures returns the upcoming departures from a stop area, ordered by time.
func (c *Client) Departures(ctx context.Context, stopAreaID string) (*DeparturesResponse, error) {
	path := fmt.Sprintf("/stop_areas/%s/departures", url.PathEscape(stopAreaID))
	var out DeparturesResponse
	if err := c.get(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Disruptions returns active disruptions. If stopAreaID is non-empty, results
// are scoped to that stop area (/stop_areas/{id}/disruptions); otherwise all
// active disruptions on the coverage are returned.
func (c *Client) Disruptions(ctx context.Context, stopAreaID string) (*DisruptionsResponse, error) {
	path := "/disruptions"
	if stopAreaID != "" {
		path = fmt.Sprintf("/stop_areas/%s/disruptions", url.PathEscape(stopAreaID))
	}
	var out DisruptionsResponse
	if err := c.get(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
