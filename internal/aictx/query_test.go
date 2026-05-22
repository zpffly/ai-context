package aictx

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestQueryJSONSeparatesAvoidRPCs(t *testing.T) {
	configPath := filepath.Join("..", "..", "examples", "demo", ".ai-context", "config.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"query", "--config", configPath, "--json", "UpdateMerchant"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("query failed with code %d: %s", code, stderr.String())
	}

	var out QueryOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("parse query output: %v\n%s", err, stdout.String())
	}
	if out.SchemaVersion != querySchemaVersion {
		t.Fatalf("unexpected schema version %q", out.SchemaVersion)
	}
	if len(out.Matches) == 0 {
		t.Fatal("expected at least one match")
	}

	var callback *QueryMatch
	for i := range out.Matches {
		if out.Matches[i].Doc.ID == "institution.verify.callback" {
			callback = &out.Matches[i]
			break
		}
	}
	if callback == nil {
		t.Fatalf("missing institution.verify.callback match: %#v", out.Matches)
	}
	assertQueryRPC(t, callback.RPCs.Positive, "MerchantService.SubmitInstitutionVerifyResult")
	assertQueryRPC(t, callback.RPCs.Avoid, "MerchantService.UpdateMerchant")
	assertMissingQueryRPC(t, callback.RPCs.Positive, "MerchantService.UpdateMerchant")
}

func assertQueryRPC(t *testing.T, refs []QueryRPCRef, name string) {
	t.Helper()
	for _, ref := range refs {
		if ref.Name == name {
			return
		}
	}
	t.Fatalf("missing query RPC %s in %#v", name, refs)
}

func assertMissingQueryRPC(t *testing.T, refs []QueryRPCRef, name string) {
	t.Helper()
	for _, ref := range refs {
		if ref.Name == name {
			t.Fatalf("unexpected query RPC %s in %#v", name, refs)
		}
	}
}
