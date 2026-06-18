// Package tools defines the MCP tool handlers exposed by the server. Each
// handler is a thin orchestration layer: it calls the typed navitia client and
// projects the result through the transform package. No business logic lives in
// the MCP wiring itself, which keeps these handlers easy to test.
package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/krezzoid/sncf-mcp/internal/navitia"
	"github.com/krezzoid/sncf-mcp/internal/transform"
)

// Handlers carries the dependencies shared by all tool handlers.
type Handlers struct {
	Client *navitia.Client
}

// New builds a Handlers value.
func New(client *navitia.Client) *Handlers {
	return &Handlers{Client: client}
}

// --- find_station --------------------------------------------------------

// FindStationInput is the argument schema for the find_station tool.
type FindStationInput struct {
	Query string `json:"query" jsonschema:"a station or city name to resolve to candidate stations, e.g. 'Lyon Part Dieu' or 'Paris'; returns ranked stop areas whose id can be passed to plan_journey"`
}

// FindStationOutput is the structured result of find_station.
type FindStationOutput struct {
	Stations []transform.LeanStation `json:"stations"`
}

// FindStation resolves a free-text query into candidate stations via the
// Navitia /places endpoint (see ADR-0001).
func (h *Handlers) FindStation(ctx context.Context, _ *mcp.CallToolRequest, in FindStationInput) (*mcp.CallToolResult, FindStationOutput, error) {
	if strings.TrimSpace(in.Query) == "" {
		return toolErrorf("Provide a station or city name to search for, e.g. \"Lyon Part Dieu\"."), FindStationOutput{Stations: []transform.LeanStation{}}, nil
	}
	resp, err := h.Client.Places(ctx, in.Query)
	if err != nil {
		return nil, FindStationOutput{}, fmt.Errorf("find_station: %w", err)
	}
	return nil, FindStationOutput{Stations: transform.Stations(resp)}, nil
}

// --- plan_journey --------------------------------------------------------

// PlanJourneyInput is the argument schema for the plan_journey tool.
type PlanJourneyInput struct {
	From    string `json:"from" jsonschema:"departure: a station/city name (resolved automatically) or a stop_area id from find_station, e.g. 'Paris Gare de Lyon'"`
	To      string `json:"to" jsonschema:"arrival: a station/city name (resolved automatically) or a stop_area id from find_station, e.g. 'Lyon Part Dieu'"`
	When    string `json:"when,omitempty" jsonschema:"optional RFC3339 datetime, e.g. '2026-06-20T14:00:00+02:00'; defaults to now"`
	Arrival bool   `json:"arrival,omitempty" jsonschema:"if true, 'when' is the desired arrival time rather than the departure time"`
}

// PlanJourneyOutput is the structured result of plan_journey.
type PlanJourneyOutput struct {
	Journeys []transform.LeanJourney `json:"journeys"`
}

// PlanJourney resolves both endpoints, computes itineraries, and returns a
// compact projection suitable for an LLM.
func (h *Handlers) PlanJourney(ctx context.Context, _ *mcp.CallToolRequest, in PlanJourneyInput) (*mcp.CallToolResult, PlanJourneyOutput, error) {
	if strings.TrimSpace(in.From) == "" || strings.TrimSpace(in.To) == "" {
		return toolErrorf("Both 'from' and 'to' are required — give a station or city name for each."), PlanJourneyOutput{Journeys: []transform.LeanJourney{}}, nil
	}

	var when time.Time
	if in.When != "" {
		t, err := time.Parse(time.RFC3339, in.When)
		if err != nil {
			return toolErrorf("Couldn't parse 'when' %q — expected an RFC3339 datetime like \"2026-06-20T14:00:00+02:00\".", in.When), PlanJourneyOutput{Journeys: []transform.LeanJourney{}}, nil
		}
		when = t
	}

	fromID, err := h.resolveStation(ctx, in.From)
	if err != nil {
		if errors.Is(err, errStationNotFound) {
			return unresolved(in.From, " (the departure)"), PlanJourneyOutput{Journeys: []transform.LeanJourney{}}, nil
		}
		return nil, PlanJourneyOutput{}, fmt.Errorf("plan_journey: resolve from: %w", err)
	}
	toID, err := h.resolveStation(ctx, in.To)
	if err != nil {
		if errors.Is(err, errStationNotFound) {
			return unresolved(in.To, " (the arrival)"), PlanJourneyOutput{Journeys: []transform.LeanJourney{}}, nil
		}
		return nil, PlanJourneyOutput{}, fmt.Errorf("plan_journey: resolve to: %w", err)
	}

	resp, err := h.Client.Journeys(ctx, fromID, toID, when, in.Arrival)
	if err != nil {
		return nil, PlanJourneyOutput{}, fmt.Errorf("plan_journey: %w", err)
	}
	return nil, PlanJourneyOutput{Journeys: transform.Journeys(resp)}, nil
}

// errStationNotFound signals that a free-text name matched no stop area. The
// tool handlers turn it into a helpful message (see unresolved) rather than
// surfacing it as a raw error.
var errStationNotFound = errors.New("no matching station")

// resolveStation returns the best-matching stop-area ID for a free-text name.
// Navitia returns places ordered by quality, so the first stop_area wins. When
// nothing matches, it returns an error wrapping errStationNotFound.
func (h *Handlers) resolveStation(ctx context.Context, name string) (string, error) {
	resp, err := h.Client.Places(ctx, name)
	if err != nil {
		return "", err
	}
	for _, p := range resp.Places {
		if p.EmbeddedType == "stop_area" {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("%w: %q", errStationNotFound, name)
}

// toolErrorf builds a non-fatal IsError tool result carrying a helpful, agent-
// readable message. Used for invalid input and unresolved stations so the model
// can recover, rather than surfacing a raw Go error.
func toolErrorf(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}},
	}
}

// unresolved builds a helpful result for a station name that could not be
// resolved, steering the agent toward recovery (rephrase, or call find_station).
// role describes which input failed, e.g. " (the departure)", and may be empty.
func unresolved(name, role string) *mcp.CallToolResult {
	return toolErrorf("Couldn't find a station matching %q%s. Check the spelling, try a more "+
		"specific name (e.g. \"Lyon Part Dieu\" rather than \"Lyon\"), or call find_station to "+
		"list candidates and pass a returned id.", name, role)
}

// --- next_departures -----------------------------------------------------

// NextDeparturesInput is the argument schema for the next_departures tool.
type NextDeparturesInput struct {
	Station string `json:"station" jsonschema:"a station name to list upcoming departures for (resolved automatically), e.g. 'Lyon Part Dieu'; use find_station first if the name may be ambiguous"`
}

// NextDeparturesOutput is the structured result of next_departures.
type NextDeparturesOutput struct {
	Departures []transform.LeanDeparture `json:"departures"`
}

// NextDepartures resolves a station name to a stop area and returns its
// upcoming departures, projected to a compact, real-time-aware shape.
func (h *Handlers) NextDepartures(ctx context.Context, _ *mcp.CallToolRequest, in NextDeparturesInput) (*mcp.CallToolResult, NextDeparturesOutput, error) {
	if strings.TrimSpace(in.Station) == "" {
		return toolErrorf("Provide a station name to list departures for, e.g. \"Lyon Part Dieu\"."), NextDeparturesOutput{Departures: []transform.LeanDeparture{}}, nil
	}
	id, err := h.resolveStation(ctx, in.Station)
	if err != nil {
		if errors.Is(err, errStationNotFound) {
			return unresolved(in.Station, ""), NextDeparturesOutput{Departures: []transform.LeanDeparture{}}, nil
		}
		return nil, NextDeparturesOutput{}, fmt.Errorf("next_departures: %w", err)
	}
	resp, err := h.Client.Departures(ctx, id)
	if err != nil {
		return nil, NextDeparturesOutput{}, fmt.Errorf("next_departures: %w", err)
	}
	return nil, NextDeparturesOutput{Departures: transform.Departures(resp)}, nil
}

// --- get_disruptions -----------------------------------------------------

// GetDisruptionsInput is the argument schema for the get_disruptions tool.
type GetDisruptionsInput struct {
	Station string `json:"station,omitempty" jsonschema:"optional station name to scope disruptions to (resolved automatically); omit for all active disruptions on the network"`
}

// GetDisruptionsOutput is the structured result of get_disruptions.
type GetDisruptionsOutput struct {
	Disruptions []transform.LeanDisruption `json:"disruptions"`
}

// GetDisruptions returns active service disruptions, optionally scoped to a
// single station. An empty Station returns network-wide disruptions.
func (h *Handlers) GetDisruptions(ctx context.Context, _ *mcp.CallToolRequest, in GetDisruptionsInput) (*mcp.CallToolResult, GetDisruptionsOutput, error) {
	var id string
	if in.Station != "" {
		var err error
		id, err = h.resolveStation(ctx, in.Station)
		if err != nil {
			if errors.Is(err, errStationNotFound) {
				return unresolved(in.Station, ""), GetDisruptionsOutput{Disruptions: []transform.LeanDisruption{}}, nil
			}
			return nil, GetDisruptionsOutput{}, fmt.Errorf("get_disruptions: %w", err)
		}
	}
	resp, err := h.Client.Disruptions(ctx, id)
	if err != nil {
		return nil, GetDisruptionsOutput{}, fmt.Errorf("get_disruptions: %w", err)
	}
	return nil, GetDisruptionsOutput{Disruptions: transform.Disruptions(resp)}, nil
}
