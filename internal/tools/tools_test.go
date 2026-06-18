package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/krezzoid/sncf-mcp/internal/navitia"
)

// fakeNavitia serves canned responses keyed by URL path so we can exercise the
// handler orchestration (resolve -> journeys -> transform) hermetically.
func fakeNavitia(t *testing.T) *navitia.Client {
	t.Helper()
	places := `{"places":[{"id":"stop_area:X","name":"X","embedded_type":"stop_area","quality":90}]}`
	journeys := `{"journeys":[{"duration":3600,"nb_transfers":0,"departure_date_time":"20260620T140000","arrival_date_time":"20260620T150000","sections":[{"type":"public_transport","departure_date_time":"20260620T140000","arrival_date_time":"20260620T150000","from":{"name":"A"},"to":{"name":"B"},"display_informations":{"commercial_mode":"TGV INOUI","headsign":"123"}}]}]}`
	departures := `{"departures":[{"display_informations":{"commercial_mode":"TER","direction":"Lyon Perrache (Lyon)","headsign":"96521"},"stop_date_time":{"departure_date_time":"20260618T143300","base_departure_date_time":"20260618T143000","data_freshness":"realtime"}}]}`
	disruptions := `{"disruptions":[{"id":"x","status":"active","severity":{"name":"perturbation","effect":"SIGNIFICANT_DELAYS"},"messages":[{"text":"Retards à prévoir."}]}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/places"):
			_, _ = w.Write([]byte(places))
		case strings.Contains(r.URL.Path, "/journeys"):
			_, _ = w.Write([]byte(journeys))
		case strings.Contains(r.URL.Path, "/departures"):
			_, _ = w.Write([]byte(departures))
		case strings.Contains(r.URL.Path, "/disruptions"):
			_, _ = w.Write([]byte(disruptions))
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return navitia.New("test-key", navitia.WithBaseURL(srv.URL), navitia.WithHTTPClient(srv.Client()))
}

func TestPlanJourney_EndToEnd(t *testing.T) {
	h := New(fakeNavitia(t))
	_, out, err := h.PlanJourney(context.Background(), nil, PlanJourneyInput{From: "Paris", To: "Lyon"})
	if err != nil {
		t.Fatalf("PlanJourney: %v", err)
	}
	if len(out.Journeys) != 1 {
		t.Fatalf("len(Journeys) = %d, want 1", len(out.Journeys))
	}
	if out.Journeys[0].DurationMin != 60 {
		t.Errorf("DurationMin = %d, want 60", out.Journeys[0].DurationMin)
	}
}

func TestFindStation_EndToEnd(t *testing.T) {
	h := New(fakeNavitia(t))
	_, out, err := h.FindStation(context.Background(), nil, FindStationInput{Query: "X"})
	if err != nil {
		t.Fatalf("FindStation: %v", err)
	}
	if len(out.Stations) != 1 || out.Stations[0].ID != "stop_area:X" {
		t.Fatalf("stations = %+v", out.Stations)
	}
}

func TestNextDepartures_EndToEnd(t *testing.T) {
	h := New(fakeNavitia(t))
	_, out, err := h.NextDepartures(context.Background(), nil, NextDeparturesInput{Station: "Lyon Part Dieu"})
	if err != nil {
		t.Fatalf("NextDepartures: %v", err)
	}
	if len(out.Departures) != 1 {
		t.Fatalf("len(Departures) = %d, want 1", len(out.Departures))
	}
	if out.Departures[0].DelayMin != 3 || !out.Departures[0].Realtime {
		t.Errorf("departure = %+v, want a 3-minute real-time delay", out.Departures[0])
	}
}

func TestGetDisruptions_EndToEnd(t *testing.T) {
	h := New(fakeNavitia(t))
	_, out, err := h.GetDisruptions(context.Background(), nil, GetDisruptionsInput{})
	if err != nil {
		t.Fatalf("GetDisruptions: %v", err)
	}
	if len(out.Disruptions) != 1 {
		t.Fatalf("len(Disruptions) = %d, want 1", len(out.Disruptions))
	}
	if out.Disruptions[0].Effect != "SIGNIFICANT_DELAYS" {
		t.Errorf("effect = %q, want SIGNIFICANT_DELAYS", out.Disruptions[0].Effect)
	}
}

func TestGetDisruptions_ScopedToStation(t *testing.T) {
	// A non-empty Station resolves to a stop area first, then scopes the
	// disruptions query to it.
	h := New(fakeNavitia(t))
	_, out, err := h.GetDisruptions(context.Background(), nil, GetDisruptionsInput{Station: "Lyon Part Dieu"})
	if err != nil {
		t.Fatalf("GetDisruptions(scoped): %v", err)
	}
	if len(out.Disruptions) != 1 || out.Disruptions[0].Status != "active" {
		t.Fatalf("disruptions = %+v", out.Disruptions)
	}
}

// servePlaces builds Handlers backed by a fake server that returns placesJSON
// for any request — enough to exercise the station-resolution paths.
func servePlaces(t *testing.T, placesJSON string) *Handlers {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(placesJSON))
	}))
	t.Cleanup(srv.Close)
	return New(navitia.New("test-key", navitia.WithBaseURL(srv.URL), navitia.WithHTTPClient(srv.Client())))
}

// resultText concatenates the text content blocks of a tool result.
func resultText(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestFindStation_EmptyResults(t *testing.T) {
	h := servePlaces(t, `{"places":[]}`)
	_, out, err := h.FindStation(context.Background(), nil, FindStationInput{Query: "nowhere-at-all"})
	if err != nil {
		t.Fatalf("FindStation: %v", err)
	}
	if len(out.Stations) != 0 {
		t.Errorf("len(Stations) = %d, want 0", len(out.Stations))
	}
}

func TestFindStation_FiltersNonStopAreas(t *testing.T) {
	places := `{"places":[
		{"id":"admin:fr:69123","name":"Lyon","embedded_type":"administrative_region","quality":95},
		{"id":"stop_area:SNCF:87723197","name":"Lyon Part-Dieu","embedded_type":"stop_area","quality":90},
		{"id":"addr:4.85;45.76","name":"Rue de Lyon","embedded_type":"address","quality":40}
	]}`
	h := servePlaces(t, places)
	_, out, err := h.FindStation(context.Background(), nil, FindStationInput{Query: "Lyon"})
	if err != nil {
		t.Fatalf("FindStation: %v", err)
	}
	// The admin region (higher quality) and the address must be dropped.
	if len(out.Stations) != 1 || out.Stations[0].ID != "stop_area:SNCF:87723197" {
		t.Fatalf("stations = %+v, want only the stop_area", out.Stations)
	}
}

func TestPlanJourney_NoStationFound_ReturnsHelpfulResult(t *testing.T) {
	// Only a non-station match for the departure -> graceful, helpful result
	// rather than a raw Go error.
	h := servePlaces(t, `{"places":[{"id":"addr:1","name":"Rue de Lyon","embedded_type":"address","quality":40}]}`)
	res, out, err := h.PlanJourney(context.Background(), nil, PlanJourneyInput{From: "Rue de Lyon", To: "Lyon"})
	if err != nil {
		t.Fatalf("got a Go error, want a graceful result: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("res = %+v, want a non-nil IsError result", res)
	}
	if len(out.Journeys) != 0 {
		t.Errorf("len(Journeys) = %d, want 0", len(out.Journeys))
	}
	if txt := resultText(res); !strings.Contains(txt, "find_station") {
		t.Errorf("message %q should steer the agent to find_station", txt)
	}
}

func TestNextDepartures_NoStationFound_ReturnsHelpfulResult(t *testing.T) {
	h := servePlaces(t, `{"places":[]}`)
	res, out, err := h.NextDepartures(context.Background(), nil, NextDeparturesInput{Station: "Nowhere"})
	if err != nil {
		t.Fatalf("got a Go error, want a graceful result: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("res = %+v, want a non-nil IsError result", res)
	}
	if len(out.Departures) != 0 {
		t.Errorf("len(Departures) = %d, want 0", len(out.Departures))
	}
}

func TestPlanJourney_PropagatesRealError(t *testing.T) {
	// A 5xx from the API is a real failure and must surface as a Go error,
	// not be masked by the graceful "not found" handling.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	h := New(navitia.New("test-key", navitia.WithBaseURL(srv.URL), navitia.WithHTTPClient(srv.Client()), navitia.WithRetry(0, 0)))
	if _, _, err := h.PlanJourney(context.Background(), nil, PlanJourneyInput{From: "X", To: "Y"}); err == nil {
		t.Fatal("expected a Go error for a 5xx API failure")
	}
}

func TestFindStation_EmptyQuery(t *testing.T) {
	res, out, err := New(nil).FindStation(context.Background(), nil, FindStationInput{Query: "  "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("res = %+v, want a non-nil IsError result", res)
	}
	if len(out.Stations) != 0 {
		t.Errorf("len(Stations) = %d, want 0", len(out.Stations))
	}
}

func TestPlanJourney_EmptyEndpoints(t *testing.T) {
	res, _, err := New(nil).PlanJourney(context.Background(), nil, PlanJourneyInput{From: "Paris", To: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("res = %+v, want a non-nil IsError result", res)
	}
}

func TestPlanJourney_BadDatetime(t *testing.T) {
	res, _, err := New(nil).PlanJourney(context.Background(), nil,
		PlanJourneyInput{From: "Paris", To: "Lyon", When: "tomorrow morning"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("res = %+v, want a non-nil IsError result", res)
	}
	if txt := resultText(res); !strings.Contains(txt, "RFC3339") {
		t.Errorf("message %q should mention the expected RFC3339 format", txt)
	}
}

func TestNextDepartures_EmptyStation(t *testing.T) {
	res, _, err := New(nil).NextDepartures(context.Background(), nil, NextDeparturesInput{Station: ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("res = %+v, want a non-nil IsError result", res)
	}
}
