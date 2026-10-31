package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestSatusehatMigrationAndTransformer(t *testing.T) {
	// 1. Initialize DB
	if err := initDb(); err != nil {
		t.Fatalf("initDb failed: %v", err)
	}

	// 2. Load engine routes
	eng := NewIntegrationEngine()
	eng.LoadActiveRoutes()

	route, exists := eng.Routes["satusehat"]
	if !exists {
		t.Fatalf("Route 'satusehat' not found in engine")
	}

	if route.Name != "SATUSEHAT Multi-Tenant Gateway" {
		t.Errorf("Expected route name 'SATUSEHAT Multi-Tenant Gateway', got '%s'", route.Name)
	}

	// Verify target endpoint
	targets := eng.GetEndpointsByRole(route, "target")
	if len(targets) == 0 {
		t.Fatalf("No target endpoint found for route 'satusehat'")
	}

	var tgtConfig map[string]interface{}
	if err := json.Unmarshal([]byte(targets[0].Config), &tgtConfig); err != nil {
		t.Fatalf("Failed to parse target config: %v", err)
	}

	if tgtConfig["name"] != "SATUSEHAT STG" {
		t.Errorf("Expected target name 'SATUSEHAT STG', got '%v'", tgtConfig["name"])
	}

	// 3. Clear cache to test live automatic OAuth2 token acquisition
	expectedOrg := route.Variables["ORG_ID"]
	if expectedOrg == "" {
		expectedOrg = os.Getenv("SATUSEHAT_ORG_ID")
	}
	cacheKey := "satusehat_stg_token_" + expectedOrg
	globalCache.Delete(cacheKey)

	sampleInput := `{
		"resourceType": "Encounter",
		"id": "enc-101",
		"method": "POST",
		"data": {
			"resourceType": "Encounter",
			"status": "arrived",
			"class": {
				"system": "http://terminology.hl7.org/CodeSystem/v3-ActCode",
				"code": "AMB"
			}
		}
	}`

	res, err := eng.runTransformerScript(route.Transformer.Script, sampleInput, "", "", route)
	if err != nil {
		t.Fatalf("Transformer execution failed: %v", err)
	}

	// Verify token was automatically fetched and cached
	cachedToken, hasToken := globalCache.Get(cacheKey)
	if !hasToken || cachedToken == "" {
		t.Fatalf("Expected token to be automatically fetched and cached in globalCache")
	}
	t.Logf("Successfully fetched and cached live token: %s...", cachedToken[:8])

	var dispatchEnv struct {
		Dispatch struct {
			URL     string            `json:"url"`
			Method  string            `json:"method"`
			Headers map[string]string `json:"headers"`
		} `json:"__dispatch"`
		Payload map[string]interface{} `json:"payload"`
	}

	if err := json.Unmarshal([]byte(res), &dispatchEnv); err != nil {
		t.Fatalf("Failed to parse transformer output JSON: %v. Output was: %s", err, res)
	}

	if dispatchEnv.Dispatch.URL != "https://api-satusehat-stg.kemkes.go.id/fhir-r4/v1/Encounter" {
		t.Errorf("Unexpected dispatch URL: %s", dispatchEnv.Dispatch.URL)
	}

	authHdr := dispatchEnv.Dispatch.Headers["Authorization"]
	if authHdr != "Bearer " + cachedToken {
		t.Errorf("Expected Authorization 'Bearer %s', got '%s'", cachedToken, authHdr)
	}

	// Check Organization injection
	serviceProvider, ok := dispatchEnv.Payload["serviceProvider"].(map[string]interface{})
	if !ok {
		t.Errorf("Expected serviceProvider injected into Encounter payload")
	} else if serviceProvider["reference"] != "Organization/"+expectedOrg {
		t.Errorf("Expected Organization/%s, got %v", expectedOrg, serviceProvider["reference"])
	}

	t.Log("SATUSEHAT STG migration and transformer test passed successfully!")
}

func TestSatusehatMultiTenantResolution(t *testing.T) {
	if err := initDb(); err != nil {
		t.Fatalf("initDb failed: %v", err)
	}

	eng := NewIntegrationEngine()
	eng.LoadActiveRoutes()

	route, exists := eng.Routes["satusehat"]
	if !exists {
		t.Fatalf("Route 'satusehat' not found")
	}

	// Test multi-tenant request with hospital_code = "KLINIK01"
	tenantCacheKey := "satusehat_stg_token_KLINIK01"
	globalCache.Delete(tenantCacheKey)

	multiTenantInput := `{
		"hospital_code": "KLINIK01",
		"resource": "Encounter",
		"method": "POST",
		"data": {
			"resourceType": "Encounter",
			"status": "arrived",
			"hospital_code": "KLINIK01",
			"class": {
				"system": "http://terminology.hl7.org/CodeSystem/v3-ActCode",
				"code": "AMB"
			}
		}
	}`

	res, err := eng.runTransformerScript(route.Transformer.Script, multiTenantInput, "", "", route)
	if err != nil {
		t.Fatalf("Multi-tenant transformer execution failed: %v", err)
	}

	// Verify tenant-specific cache key has the token
	token, found := globalCache.Get(tenantCacheKey)
	if !found || token == "" {
		t.Fatalf("Expected tenant token cached at key '%s'", tenantCacheKey)
	}
	t.Logf("Multi-Tenant Token for KLINIK01 successfully acquired: %s...", token[:8])

	var dispatchEnv struct {
		Dispatch struct {
			URL     string            `json:"url"`
			Method  string            `json:"method"`
			Headers map[string]string `json:"headers"`
		} `json:"__dispatch"`
		Payload map[string]interface{} `json:"payload"`
	}
	if err := json.Unmarshal([]byte(res), &dispatchEnv); err != nil {
		t.Fatalf("Failed to parse output JSON: %v", err)
	}

	// Check Organization injection from KLINIK01 config
	var expectedTenantOrg string
	if tenantsJson, ok := eng.GlobalVars["TENANT_CREDENTIALS"]; ok && tenantsJson != "" {
		var tenants map[string]interface{}
		if err := json.Unmarshal([]byte(tenantsJson), &tenants); err == nil {
			if k1, exists := tenants["KLINIK01"].(map[string]interface{}); exists {
				if cfg, ok := k1["satusehat"].(map[string]interface{}); ok {
					expectedTenantOrg = fmt.Sprintf("%v", cfg["org_id"])
				}
			}
		}
	}
	if expectedTenantOrg == "" {
		expectedTenantOrg = os.Getenv("SATUSEHAT_ORG_ID")
	}

	sp, ok := dispatchEnv.Payload["serviceProvider"].(map[string]interface{})
	if !ok || sp["reference"] != "Organization/"+expectedTenantOrg {
		t.Errorf("Expected Organization/%s from KLINIK01 credentials, got: %v", expectedTenantOrg, sp)
	}

	// Check non-FHIR field was cleaned
	if _, hasField := dispatchEnv.Payload["hospital_code"]; hasField {
		t.Errorf("Expected hospital_code to be deleted from clean FHIR payload")
	}

	t.Log("Multi-Tenant resolution test passed successfully!")
}
