// Package navitia provides a typed client for the SNCF API, which is a hosted
// instance of the open-source Navitia journey-planning engine.
//
// API reference: https://doc.navitia.io/
// SNCF coverage base URL: https://api.sncf.com/v1/coverage/sncf/
//
// Authentication is HTTP Basic, where the username is the API key and the
// password is empty (see ADR-0003 and the SNCF integration docs).
package navitia

// NavitiaTime is the datetime format used throughout the Navitia API:
// ISO 8601 "basic" form, e.g. "20260620T140000".
const NavitiaTime = "20060102T150405"

// --- /places (autocomplete & geocoding) ---------------------------------
//
// We resolve user-supplied station names to stable Navitia IDs via this
// endpoint instead of shipping a static CSV of coordinates. See ADR-0001.

// PlacesResponse is the payload returned by GET /places?q=...
type PlacesResponse struct {
	Places []Place `json:"places"`
}

// Place is a single autocomplete match. EmbeddedType tells you which of the
// nested objects is populated (we mostly care about "stop_area").
type Place struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	EmbeddedType string    `json:"embedded_type"`
	Quality      int       `json:"quality"`
	StopArea     *StopArea `json:"stop_area,omitempty"`
}

// StopArea is a named group of stop points (typically a station).
type StopArea struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Label string `json:"label"`
	Coord *Coord `json:"coord,omitempty"`
}

// Coord is a WGS84 coordinate. Navitia encodes lat/lon as strings.
type Coord struct {
	Lat string `json:"lat"`
	Lon string `json:"lon"`
}

// --- /journeys -----------------------------------------------------------

// JourneysResponse is the payload returned by GET /journeys?from=...&to=...
type JourneysResponse struct {
	Journeys []Journey `json:"journeys"`
}

// Journey is one end-to-end itinerary made of one or more sections.
type Journey struct {
	DurationSeconds   int       `json:"duration"`
	NbTransfers       int       `json:"nb_transfers"`
	DepartureDateTime string    `json:"departure_date_time"`
	ArrivalDateTime   string    `json:"arrival_date_time"`
	Sections          []Section `json:"sections"`
}

// Section is one leg of a journey (a train ride, a transfer, a walk, ...).
type Section struct {
	Type              string               `json:"type"`
	DepartureDateTime string               `json:"departure_date_time"`
	ArrivalDateTime   string               `json:"arrival_date_time"`
	From              *Endpoint            `json:"from,omitempty"`
	To                *Endpoint            `json:"to,omitempty"`
	DisplayInfo       *DisplayInformations `json:"display_informations,omitempty"`
}

// Endpoint is the origin or destination of a section.
type Endpoint struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DisplayInformations carries the human-facing labels for a public-transport
// section (line, train number, mode, direction).
type DisplayInformations struct {
	CommercialMode string `json:"commercial_mode"`
	PhysicalMode   string `json:"physical_mode"`
	Headsign       string `json:"headsign"`
	Direction      string `json:"direction"`
	Label          string `json:"label"`
}

// --- /stop_areas/{id}/departures ----------------------------------------

// DeparturesResponse is the payload returned by the departures endpoint.
type DeparturesResponse struct {
	Departures []Departure `json:"departures"`
}

// Departure is a single upcoming departure from a stop area.
type Departure struct {
	StopDateTime StopDateTime         `json:"stop_date_time"`
	DisplayInfo  *DisplayInformations `json:"display_informations,omitempty"`
}

// StopDateTime carries the scheduled (base) and real-time departure stamps.
// DataFreshness is "realtime" when the times reflect live data, or
// "base_schedule" when only the timetable is known.
type StopDateTime struct {
	DepartureDateTime     string `json:"departure_date_time"`
	BaseDepartureDateTime string `json:"base_departure_date_time"`
	DataFreshness         string `json:"data_freshness"`
}

// --- /disruptions --------------------------------------------------------

// DisruptionsResponse is the payload returned by the disruptions endpoint.
type DisruptionsResponse struct {
	Disruptions []Disruption `json:"disruptions"`
}

// Disruption describes a service perturbation.
type Disruption struct {
	ID       string              `json:"id"`
	Status   string              `json:"status"`
	Severity *Severity           `json:"severity,omitempty"`
	Messages []DisruptionMessage `json:"messages"`
}

// Severity classifies how serious a disruption is.
type Severity struct {
	Name   string `json:"name"`
	Effect string `json:"effect"`
}

// DisruptionMessage is one human-readable note attached to a disruption.
type DisruptionMessage struct {
	Text string `json:"text"`
}
