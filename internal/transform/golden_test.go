package transform

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/krezzoid/sncf-mcp/internal/navitia"
)

var updateGolden = flag.Bool("update", false, "regenerate golden files under testdata/golden")

// goldenCase pairs a recorded Navitia response with the projection that turns it
// into the lean shape an agent sees.
type goldenCase struct {
	name    string                    // golden-file stem and subtest name
	input   string                    // input fixture under testdata/
	project func([]byte) (any, error) // unmarshal + transform
}

func goldenCases() []goldenCase {
	return []goldenCase{
		{"journeys", "journeys_paris_lyon.json", func(b []byte) (any, error) {
			var r navitia.JourneysResponse
			err := json.Unmarshal(b, &r)
			return Journeys(&r), err
		}},
		{"stations", "places_lyon.json", func(b []byte) (any, error) {
			var r navitia.PlacesResponse
			err := json.Unmarshal(b, &r)
			return Stations(&r), err
		}},
		{"departures", "departures_lyon.json", func(b []byte) (any, error) {
			var r navitia.DeparturesResponse
			err := json.Unmarshal(b, &r)
			return Departures(&r), err
		}},
		{"disruptions", "disruptions.json", func(b []byte) (any, error) {
			var r navitia.DisruptionsResponse
			err := json.Unmarshal(b, &r)
			return Disruptions(&r), err
		}},
	}
}

// TestGolden pins the lean output contract (ADR-0005): a recorded heavy Navitia
// response in, the exact expected lean JSON out. Any change to an output shape
// fails here on purpose. Regenerate intentionally with:
//
//	go test ./internal/transform -update
func TestGolden(t *testing.T) {
	for _, tc := range goldenCases() {
		t.Run(tc.name, func(t *testing.T) {
			in := readTestFile(t, filepath.Join("..", "..", "testdata", tc.input))

			got, err := tc.project(in)
			if err != nil {
				t.Fatalf("project %s: %v", tc.input, err)
			}
			gotJSON, err := json.MarshalIndent(got, "", "  ")
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			gotJSON = append(gotJSON, '\n')

			golden := filepath.Join("..", "..", "testdata", "golden", tc.name+".golden.json")
			if *updateGolden {
				if err := os.MkdirAll(filepath.Dir(golden), 0o750); err != nil {
					t.Fatalf("mkdir golden dir: %v", err)
				}
				if err := os.WriteFile(golden, gotJSON, 0o600); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				t.Logf("updated %s", golden)
				return
			}

			want := readTestFile(t, golden)
			if !bytes.Equal(gotJSON, want) {
				t.Errorf("lean output drifted from the contract (%s).\n"+
					"Run `go test ./internal/transform -update` if this change is intentional.\n"+
					"--- got ---\n%s\n--- want ---\n%s", golden, gotJSON, want)
			}
		})
	}
}

// readTestFile reads a fixture or golden file. Paths are test-controlled.
func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path) //nolint:gosec // test-controlled path, not user input
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
