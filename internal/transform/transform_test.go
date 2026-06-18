package transform

import (
	"testing"

	"github.com/krezzoid/sncf-mcp/internal/navitia"
)

func TestJourneys_ProjectsLeanShape(t *testing.T) {
	resp := &navitia.JourneysResponse{
		Journeys: []navitia.Journey{
			{
				DurationSeconds:   6960,
				NbTransfers:       0,
				DepartureDateTime: "20260620T140000",
				ArrivalDateTime:   "20260620T155600",
				Sections: []navitia.Section{
					{
						Type:              "public_transport",
						DepartureDateTime: "20260620T140000",
						ArrivalDateTime:   "20260620T155600",
						From:              &navitia.Endpoint{Name: "Paris Gare de Lyon"},
						To:                &navitia.Endpoint{Name: "Lyon Part Dieu"},
						DisplayInfo: &navitia.DisplayInformations{
							CommercialMode: "TGV INOUI",
							Headsign:       "6607",
						},
					},
					// A transfer section must be excluded from Legs.
					{Type: "transfer"},
				},
			},
		},
	}

	got := Journeys(resp)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	j := got[0]
	if j.DurationMin != 116 {
		t.Errorf("DurationMin = %d, want 116", j.DurationMin)
	}
	if len(j.Legs) != 1 {
		t.Fatalf("len(Legs) = %d, want 1 (transfer must be dropped)", len(j.Legs))
	}
	if j.Legs[0].Mode != "TGV INOUI" || j.Legs[0].Train != "6607" {
		t.Errorf("leg = %+v, want TGV INOUI / 6607", j.Legs[0])
	}
	if j.Legs[0].From != "Paris Gare de Lyon" || j.Legs[0].To != "Lyon Part Dieu" {
		t.Errorf("endpoints = %q -> %q", j.Legs[0].From, j.Legs[0].To)
	}
}

func TestStations_KeepsOnlyStopAreas(t *testing.T) {
	resp := &navitia.PlacesResponse{
		Places: []navitia.Place{
			{ID: "a", Name: "Lyon Part Dieu", EmbeddedType: "stop_area", Quality: 90},
			{ID: "b", Name: "Rue de Lyon", EmbeddedType: "address", Quality: 50},
		},
	}
	got := Stations(resp)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].ID != "a" {
		t.Errorf("kept %q, want the stop_area", got[0].ID)
	}
}

func TestDepartures_ComputesDelayAndRealtime(t *testing.T) {
	resp := &navitia.DeparturesResponse{
		Departures: []navitia.Departure{
			{
				StopDateTime: navitia.StopDateTime{
					DepartureDateTime:     "20260618T143300",
					BaseDepartureDateTime: "20260618T143000",
					DataFreshness:         "realtime",
				},
				DisplayInfo: &navitia.DisplayInformations{
					CommercialMode: "TER",
					Headsign:       "96521",
					Direction:      "Lyon Perrache (Lyon)",
				},
			},
			{
				StopDateTime: navitia.StopDateTime{
					DepartureDateTime:     "20260618T150000",
					BaseDepartureDateTime: "20260618T150000",
					DataFreshness:         "base_schedule",
				},
				DisplayInfo: &navitia.DisplayInformations{CommercialMode: "TGV INOUI", Headsign: "6612"},
			},
		},
	}

	got := Departures(resp)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].DelayMin != 3 {
		t.Errorf("DelayMin = %d, want 3", got[0].DelayMin)
	}
	if !got[0].Realtime {
		t.Error("first departure should be realtime")
	}
	if got[0].Mode != "TER" || got[0].Train != "96521" || got[0].Direction != "Lyon Perrache (Lyon)" {
		t.Errorf("first departure = %+v", got[0])
	}
	if got[1].DelayMin != 0 || got[1].Realtime {
		t.Errorf("second departure should be on-time and scheduled-only, got %+v", got[1])
	}
}

func TestDisruptions_ProjectsSeverityAndMessage(t *testing.T) {
	resp := &navitia.DisruptionsResponse{
		Disruptions: []navitia.Disruption{
			{
				Status:   "active",
				Severity: &navitia.Severity{Name: "perturbation", Effect: "SIGNIFICANT_DELAYS"},
				Messages: []navitia.DisruptionMessage{{Text: "Retards à prévoir."}},
			},
			// No severity and no messages: must not panic, yields zero values.
			{Status: "active"},
		},
	}

	got := Disruptions(resp)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Severity != "perturbation" || got[0].Effect != "SIGNIFICANT_DELAYS" {
		t.Errorf("first = %+v", got[0])
	}
	if got[0].Message != "Retards à prévoir." {
		t.Errorf("message = %q", got[0].Message)
	}
	if got[1].Severity != "" || got[1].Effect != "" || got[1].Message != "" {
		t.Errorf("second should have empty severity/effect/message, got %+v", got[1])
	}
}
