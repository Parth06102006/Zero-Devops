// Package schema conformance-tests the deploy.jobs message contract against its
// canonical JSON Schema at the repo root (schemas/deploy-jobs-v1.schema.json).
// This package intentionally does not import the missing-contract-style
// internals; it validates the shared schema and shared fixtures so both
// services are proven to accept the same version.
package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Zero_Devops/worker_server/internal/deployments/contract"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

const (
	schemaRel   = "../../../../schemas/deploy-jobs-v1.schema.json"
	fixturesRel = "../../../../schemas/fixtures"
)

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	// Compiling also validates the schema file against its declared
	// draft 2020-12 meta-schema: a broken schema fails here.
	schema, err := jsonschema.Compile(schemaRel)
	if err != nil {
		t.Fatalf("compile shared deploy-jobs schema: %v", err)
	}
	return schema
}

func TestSchemaCompiles(t *testing.T) {
	compileSchema(t)
}

func TestFixturesConformToSharedSchema(t *testing.T) {
	schema := compileSchema(t)

	entries, err := os.ReadDir(fixturesRel)
	if err != nil {
		t.Fatalf("read fixtures dir %s: %v", fixturesRel, err)
	}
	if len(entries) == 0 {
		t.Fatalf("no fixtures found in %s", fixturesRel)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(fixturesRel, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}

			var doc any
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("fixture is not valid JSON: %v", err)
			}

			verr := schema.Validate(doc)
			switch {
			case strings.HasPrefix(entry.Name(), "valid-"):
				if verr != nil {
					t.Fatalf("valid fixture rejected by shared schema: %v", verr)
				}
			case strings.HasPrefix(entry.Name(), "invalid-"):
				if verr == nil {
					t.Fatalf("invalid fixture accepted by shared schema: %s", entry.Name())
				}
			default:
				t.Fatalf("fixture %s must be named valid-* or invalid-*", entry.Name())
			}
		})
	}
}

// TestValidFixturesDecodeAsContract ties the schema back to this module's Go
// implementation: every valid fixture must also survive contract.DecodeV1.
// (The inverse does not hold — Go unmarshal is lenient about unknown fields,
// so invalid-unknown-field.json is rejected by the schema but not by Go.)
func TestValidFixturesDecodeAsContract(t *testing.T) {
	entries, err := os.ReadDir(fixturesRel)
	if err != nil {
		t.Fatalf("read fixtures dir %s: %v", fixturesRel, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "valid-") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(fixturesRel, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := contract.DecodeV1(raw); err != nil {
				t.Fatalf("valid fixture rejected by contract.DecodeV1: %v", err)
			}
		})
	}
}
