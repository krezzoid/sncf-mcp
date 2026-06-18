//go:build integration

// Integration tests hit the real SNCF API. They are excluded from the default
// build by the `integration` build tag and are skipped unless SNCF_API_KEY is
// set in the environment. Run them with: make integration
package navitia

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestIntegration_PlacesAndJourneys(t *testing.T) {
	key := os.Getenv("SNCF_API_KEY")
	if key == "" {
		t.Skip("SNCF_API_KEY not set; skipping live API integration test")
	}

	c := New(key)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	from := resolveStopArea(ctx, t, c, "Paris Gare de Lyon")
	to := resolveStopArea(ctx, t, c, "Lyon Part Dieu")

	journeys, err := c.Journeys(ctx, from, to, time.Time{}, false)
	if err != nil {
		t.Fatalf("Journeys(%s -> %s): %v", from, to, err)
	}
	if len(journeys.Journeys) == 0 {
		t.Fatal("expected at least one Paris -> Lyon journey")
	}
	if d := journeys.Journeys[0].DurationSeconds; d <= 0 {
		t.Errorf("first journey duration = %ds, want > 0", d)
	}
}

// resolveStopArea returns the first stop-area ID matching name, failing the
// test if the lookup errors or returns nothing.
func resolveStopArea(ctx context.Context, t *testing.T, c *Client, name string) string {
	t.Helper()
	resp, err := c.Places(ctx, name)
	if err != nil {
		t.Fatalf("Places(%q): %v", name, err)
	}
	if len(resp.Places) == 0 {
		t.Fatalf("no places returned for %q", name)
	}
	return resp.Places[0].ID
}
