package aictx

import (
	"path/filepath"
	"testing"
)

func TestParseThriftFile(t *testing.T) {
	path := filepath.Join("..", "..", "examples", "demo", "idl", "institution.thrift")
	defs, err := parseThriftFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]RPCDefinition{}
	for _, def := range defs {
		got[def.FullName()] = def
	}
	if _, ok := got["InstitutionService.VerifyInstitution"]; !ok {
		t.Fatalf("missing InstitutionService.VerifyInstitution, got %#v", got)
	}
	if _, ok := got["MerchantService.SubmitInstitutionVerifyResult"]; !ok {
		t.Fatalf("missing MerchantService.SubmitInstitutionVerifyResult, got %#v", got)
	}
}
