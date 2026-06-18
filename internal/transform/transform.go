// Package transform projects the verbose Navitia API payloads into compact,
// LLM-friendly shapes. Navitia journeys responses can be very large; sending
// them verbatim wastes the model's context window. We keep only what an agent
// needs to reason about a trip. See ADR-0001 / the README rationale.
//
// The Lean* types below are the agent-facing output contract: their JSON field
// names and semantics are stable, and changes to them are breaking (see
// ADR-0005). The golden-file tests in golden_test.go pin these shapes so that
// accidental drift fails CI.
package transform

import (
	"time"

	"github.com/krezzoid/sncf-mcp/internal/navitia"
)

// LeanJourney is a compact representation of a single itinerary.
type LeanJourney struct {
	Departure   string    `json:"departure"`        // RFC3339 local time
	Arrival     string    `json:"arrival"`          // RFC3339 local time
	DurationMin int       `json:"duration_minutes"` // total minutes
	Transfers   int       `json:"transfers"`
	Legs        []LeanLeg `json:"legs"`
}

// LeanLeg is one public-transport leg of a journey (transfers/walks are folded
// into the transfer count rather than listed).
type LeanLeg struct {
	Mode      string `json:"mode"`  // e.g. "TGV INOUI", "TER"
	Train     string `json:"train"` // headsign / train number
	From      string `json:"from"`
	To        string `json:"to"`
	Departure string `json:"departure"` // RFC3339 local time
	Arrival   string `json:"arrival"`   // RFC3339 local time
}

// LeanStation is a compact representation of a /places match.
type LeanStation struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Quality int    `json:"quality"`
}

// Journeys converts a JourneysResponse into a slice of LeanJourney.
func Journeys(resp *navitia.JourneysResponse) []LeanJourney {
	if resp == nil {
		return nil
	}
	out := make([]LeanJourney, 0, len(resp.Journeys))
	for _, j := range resp.Journeys {
		lj := LeanJourney{
			Departure:   parseTime(j.DepartureDateTime),
			Arrival:     parseTime(j.ArrivalDateTime),
			DurationMin: j.DurationSeconds / 60,
			Transfers:   j.NbTransfers,
		}
		for _, s := range j.Sections {
			if s.Type != "public_transport" || s.DisplayInfo == nil {
				continue
			}
			leg := LeanLeg{
				Mode:      s.DisplayInfo.CommercialMode,
				Train:     s.DisplayInfo.Headsign,
				Departure: parseTime(s.DepartureDateTime),
				Arrival:   parseTime(s.ArrivalDateTime),
			}
			if s.From != nil {
				leg.From = s.From.Name
			}
			if s.To != nil {
				leg.To = s.To.Name
			}
			lj.Legs = append(lj.Legs, leg)
		}
		out = append(out, lj)
	}
	return out
}

// Stations converts a PlacesResponse into a slice of LeanStation, keeping only
// stop-area matches (the resolvable targets for journey planning).
func Stations(resp *navitia.PlacesResponse) []LeanStation {
	if resp == nil {
		return nil
	}
	out := make([]LeanStation, 0, len(resp.Places))
	for _, p := range resp.Places {
		if p.EmbeddedType != "stop_area" {
			continue
		}
		out = append(out, LeanStation{ID: p.ID, Name: p.Name, Quality: p.Quality})
	}
	return out
}

// parisLoc is the SNCF coverage timezone. Navitia "basic" datetimes are local
// to it but carry no offset, so we attach it explicitly when rendering (see
// ADR-0005). It falls back to UTC if the zone database is unavailable; the
// binary embeds it via time/tzdata, so that should not happen in practice.
var parisLoc = loadParis()

func loadParis() *time.Location {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		return time.UTC
	}
	return loc
}

// parseTime converts a Navitia "basic" datetime (local to Europe/Paris, with no
// offset) into an RFC3339 string carrying the correct offset. On a parse failure
// it returns the input unchanged so we never silently drop data.
func parseTime(s string) string {
	t, err := time.ParseInLocation(navitia.NavitiaTime, s, parisLoc)
	if err != nil {
		return s
	}
	return t.Format(time.RFC3339)
}

// --- departures ----------------------------------------------------------

// LeanDeparture is a compact, real-time-aware view of a single departure.
type LeanDeparture struct {
	Direction string `json:"direction"`     // terminus / headsign direction
	Mode      string `json:"mode"`          // commercial mode, e.g. "TER", "TGV INOUI"
	Train     string `json:"train"`         // headsign / train number
	Scheduled string `json:"scheduled"`     // RFC3339 timetabled departure
	Expected  string `json:"expected"`      // RFC3339 real-time departure (== scheduled if none)
	DelayMin  int    `json:"delay_minutes"` // minutes late, clamped at 0
	Realtime  bool   `json:"realtime"`      // whether backed by real-time data
}

// Departures converts a DeparturesResponse into a slice of LeanDeparture.
func Departures(resp *navitia.DeparturesResponse) []LeanDeparture {
	if resp == nil {
		return nil
	}
	out := make([]LeanDeparture, 0, len(resp.Departures))
	for _, d := range resp.Departures {
		sdt := d.StopDateTime
		scheduled := sdt.BaseDepartureDateTime
		if scheduled == "" {
			scheduled = sdt.DepartureDateTime
		}
		ld := LeanDeparture{
			Scheduled: parseTime(scheduled),
			Expected:  parseTime(sdt.DepartureDateTime),
			DelayMin:  delayMinutes(scheduled, sdt.DepartureDateTime),
			Realtime:  sdt.DataFreshness == "realtime",
		}
		if d.DisplayInfo != nil {
			ld.Direction = d.DisplayInfo.Direction
			ld.Mode = d.DisplayInfo.CommercialMode
			ld.Train = d.DisplayInfo.Headsign
		}
		out = append(out, ld)
	}
	return out
}

// --- disruptions ---------------------------------------------------------

// LeanDisruption is a compact view of a single service disruption.
type LeanDisruption struct {
	Status   string `json:"status"`   // e.g. "active", "future", "past"
	Severity string `json:"severity"` // severity name, e.g. "perturbation"
	Effect   string `json:"effect"`   // e.g. "SIGNIFICANT_DELAYS", "NO_SERVICE"
	Message  string `json:"message"`  // first human-readable message, if any
}

// Disruptions converts a DisruptionsResponse into a slice of LeanDisruption.
func Disruptions(resp *navitia.DisruptionsResponse) []LeanDisruption {
	if resp == nil {
		return nil
	}
	out := make([]LeanDisruption, 0, len(resp.Disruptions))
	for _, d := range resp.Disruptions {
		ld := LeanDisruption{Status: d.Status}
		if d.Severity != nil {
			ld.Severity = d.Severity.Name
			ld.Effect = d.Severity.Effect
		}
		for _, m := range d.Messages {
			if m.Text != "" {
				ld.Message = m.Text
				break
			}
		}
		out = append(out, ld)
	}
	return out
}

// delayMinutes returns how many whole minutes `actual` is after `base`. It
// returns 0 if either stamp is missing/unparseable or the train is not late.
func delayMinutes(base, actual string) int {
	if base == "" || actual == "" {
		return 0
	}
	b, err1 := time.Parse(navitia.NavitiaTime, base)
	a, err2 := time.Parse(navitia.NavitiaTime, actual)
	if err1 != nil || err2 != nil {
		return 0
	}
	if a.After(b) {
		return int(a.Sub(b).Minutes())
	}
	return 0
}
