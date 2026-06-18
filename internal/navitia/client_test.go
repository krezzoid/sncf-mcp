package navitia

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// loadFixture reads a JSON fixture from ../../testdata.
func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../testdata/" + name) //nolint:gosec // fixtures are repo-controlled, not user input
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// newTestClient spins up an httptest server that serves `body` for any request
// and returns a Client pointed at it. The handler also asserts that Basic auth
// is set with the API key as the username.
func newTestClient(t *testing.T, body []byte) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := r.BasicAuth()
		if !ok || user != "test-key" {
			t.Errorf("expected basic auth with key as username, got user=%q ok=%v", user, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return New("test-key", WithBaseURL(srv.URL), WithHTTPClient(srv.Client()))
}

func TestJourneys_ParsesFixture(t *testing.T) {
	c := newTestClient(t, loadFixture(t, "journeys_paris_lyon.json"))

	got, err := c.Journeys(context.Background(), "stop_area:from", "stop_area:to", time.Time{}, false)
	if err != nil {
		t.Fatalf("Journeys: %v", err)
	}
	if len(got.Journeys) == 0 {
		t.Fatal("expected at least one journey")
	}

	j := got.Journeys[0]
	if j.NbTransfers != 0 {
		t.Errorf("NbTransfers = %d, want 0", j.NbTransfers)
	}
	if j.DurationSeconds != 6960 {
		t.Errorf("DurationSeconds = %d, want 6960", j.DurationSeconds)
	}
	if len(j.Sections) == 0 || j.Sections[0].DisplayInfo == nil {
		t.Fatal("expected a section with display informations")
	}
	if mode := j.Sections[0].DisplayInfo.CommercialMode; mode != "TGV INOUI" {
		t.Errorf("CommercialMode = %q, want %q", mode, "TGV INOUI")
	}
}

func TestPlaces_ParsesFixture(t *testing.T) {
	// Inline fixture is fine for a small payload.
	body := []byte(`{"places":[{"id":"stop_area:SNCF:87686006","name":"Paris Gare de Lyon (Paris)","embedded_type":"stop_area","quality":90,"stop_area":{"id":"stop_area:SNCF:87686006","name":"Paris Gare de Lyon","label":"Paris Gare de Lyon (Paris)"}}]}`)
	c := newTestClient(t, body)

	got, err := c.Places(context.Background(), "Paris Gare de Lyon")
	if err != nil {
		t.Fatalf("Places: %v", err)
	}
	if len(got.Places) != 1 {
		t.Fatalf("len(Places) = %d, want 1", len(got.Places))
	}
	if got.Places[0].EmbeddedType != "stop_area" {
		t.Errorf("EmbeddedType = %q, want stop_area", got.Places[0].EmbeddedType)
	}
}

func TestDepartures_ParsesFixture(t *testing.T) {
	c := newTestClient(t, loadFixture(t, "departures_lyon.json"))

	got, err := c.Departures(context.Background(), "stop_area:SNCF:87723197")
	if err != nil {
		t.Fatalf("Departures: %v", err)
	}
	if len(got.Departures) != 2 {
		t.Fatalf("len(Departures) = %d, want 2", len(got.Departures))
	}

	d0 := got.Departures[0]
	if d0.DisplayInfo == nil || d0.DisplayInfo.CommercialMode != "TER" {
		t.Errorf("first departure display info = %+v, want TER", d0.DisplayInfo)
	}
	if d0.StopDateTime.BaseDepartureDateTime != "20260618T143000" {
		t.Errorf("base departure = %q, want 20260618T143000", d0.StopDateTime.BaseDepartureDateTime)
	}
	if d0.StopDateTime.DataFreshness != "realtime" {
		t.Errorf("data_freshness = %q, want realtime", d0.StopDateTime.DataFreshness)
	}
}

func TestDisruptions_ParsesFixture(t *testing.T) {
	c := newTestClient(t, loadFixture(t, "disruptions.json"))

	got, err := c.Disruptions(context.Background(), "")
	if err != nil {
		t.Fatalf("Disruptions: %v", err)
	}
	if len(got.Disruptions) != 2 {
		t.Fatalf("len(Disruptions) = %d, want 2", len(got.Disruptions))
	}

	d0 := got.Disruptions[0]
	if d0.Status != "active" {
		t.Errorf("status = %q, want active", d0.Status)
	}
	if d0.Severity == nil || d0.Severity.Effect != "SIGNIFICANT_DELAYS" {
		t.Errorf("severity = %+v, want effect SIGNIFICANT_DELAYS", d0.Severity)
	}
	if len(d0.Messages) == 0 || d0.Messages[0].Text == "" {
		t.Error("expected a non-empty disruption message")
	}
}

// serve starts an httptest server with handler h and returns a Client pointed
// at it. Extra options (e.g. WithRetry) are applied after the test defaults.
func serve(t *testing.T, h http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	base := []Option{WithBaseURL(srv.URL), WithHTTPClient(srv.Client())}
	return New("test-key", append(base, opts...)...)
}

func TestPlaces_EmptyResults(t *testing.T) {
	c := newTestClient(t, []byte(`{"places":[]}`))
	got, err := c.Places(context.Background(), "nowhere-at-all")
	if err != nil {
		t.Fatalf("Places: %v", err)
	}
	if len(got.Places) != 0 {
		t.Errorf("len(Places) = %d, want 0", len(got.Places))
	}
}

func TestGet_APIErrorOnNon2xx(t *testing.T) {
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
	})

	_, err := c.Places(context.Background(), "x")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "not found") {
		t.Errorf("Body = %q, want it to contain the server message", apiErr.Body)
	}
}

func TestGet_RetriesThenSucceeds(t *testing.T) {
	var hits int32
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable) // 503, retryable
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"places":[{"id":"stop_area:Z","name":"Z","embedded_type":"stop_area"}]}`))
	}, WithRetry(3, time.Millisecond))

	got, err := c.Places(context.Background(), "Z")
	if err != nil {
		t.Fatalf("Places: %v", err)
	}
	if len(got.Places) != 1 {
		t.Fatalf("len(Places) = %d, want 1", len(got.Places))
	}
	if n := atomic.LoadInt32(&hits); n != 3 {
		t.Errorf("server hits = %d, want 3 (2 failures + 1 success)", n)
	}
}

func TestGet_RetriesExhaustedReturnsLastError(t *testing.T) {
	var hits int32
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusBadGateway) // 502 on every attempt
	}, WithRetry(2, time.Millisecond))

	_, err := c.Places(context.Background(), "x")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadGateway {
		t.Fatalf("err = %v, want *APIError 502", err)
	}
	if n := atomic.LoadInt32(&hits); n != 3 {
		t.Errorf("server hits = %d, want 3 (1 initial + 2 retries)", n)
	}
}

func TestGet_DoesNotRetry4xx(t *testing.T) {
	var hits int32
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.Error(w, "bad request", http.StatusBadRequest) // 400, not retryable
	}, WithRetry(3, time.Millisecond))

	_, err := c.Places(context.Background(), "x")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("err = %v, want *APIError 400", err)
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Errorf("server hits = %d, want 1 (4xx must not be retried)", n)
	}
}

func TestGet_Honors429RetryAfter(t *testing.T) {
	var hits int32
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) == 1 {
			w.Header().Set("Retry-After", "0") // ask to retry, no real wait
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"places":[]}`))
	}, WithRetry(2, time.Millisecond))

	if _, err := c.Places(context.Background(), "x"); err != nil {
		t.Fatalf("Places: %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 2 {
		t.Errorf("server hits = %d, want 2 (429 then success)", n)
	}
}

func TestParseRetryAfter(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"5", 5 * time.Second},
		{"0", 0},
		{"-3", 0},
		{"not-a-number", 0},
	}
	for _, tc := range cases {
		if got := parseRetryAfter(tc.in); got != tc.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}

	// An HTTP-date in the future yields a positive delay.
	future := time.Now().Add(2 * time.Hour).UTC().Format(http.TimeFormat)
	if got := parseRetryAfter(future); got <= 0 {
		t.Errorf("parseRetryAfter(future date) = %v, want > 0", got)
	}
}

func TestBackoff_RetryAfterAndJitterBounds(t *testing.T) {
	// A positive Retry-After wins outright.
	if got := backoff(time.Second, 3, 5*time.Second); got != 5*time.Second {
		t.Errorf("backoff with Retry-After = %v, want 5s", got)
	}
	// Without Retry-After, equal jitter keeps the delay within [exp/2, exp].
	base := 100 * time.Millisecond
	for attempt := 0; attempt < 4; attempt++ {
		exp := base << attempt
		if exp > maxBackoff {
			exp = maxBackoff
		}
		for i := 0; i < 64; i++ {
			if got := backoff(base, attempt, 0); got < exp/2 || got > exp {
				t.Fatalf("attempt %d: backoff = %v, want within [%v, %v]", attempt, got, exp/2, exp)
			}
		}
	}
}

func TestErrorsDoNotLeakAPIKey(t *testing.T) {
	const key = "super-secret-key-do-not-log"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	c := New(key, WithBaseURL(srv.URL), WithHTTPClient(srv.Client()), WithRetry(0, 0))

	_, err := c.Places(context.Background(), "Paris")
	if err == nil {
		t.Fatal("expected an error from a 500 response")
	}
	if strings.Contains(err.Error(), key) {
		t.Errorf("error leaks the API key: %v", err)
	}
}

func TestGet_StopsRetryingWhenContextDone(t *testing.T) {
	var hits int32
	c := serve(t, func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable) // always 503
	}, WithRetry(5, 500*time.Millisecond))

	// The deadline is far shorter than the backoff, so the wait between retries
	// must be cut short and the context error surfaced.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := c.Places(ctx, "x")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	if n := atomic.LoadInt32(&hits); n > 2 {
		t.Errorf("server hits = %d, want <= 2 (context should cut retries short)", n)
	}
}
