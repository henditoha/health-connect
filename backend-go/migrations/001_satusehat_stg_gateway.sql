-- ============================================================================
-- MIGRATION: SATUSEHAT Multi-Tenant Gateway & SATUSEHAT STG Destination
-- Kredensial: org_id, client_id, client_secret
-- Otomatisasi: OAuth2 Token & Auto-Refresh Token via In-Memory TTL Cache
-- Kompatibel: SQLite, PostgreSQL, MySQL
-- ============================================================================

-- 1. Route Group: SATUSEHAT KEMENKES
INSERT INTO route_groups (id, name, description)
SELECT 'grp-satusehat', 'SATUSEHAT KEMENKES', 'Integrasi SATUSEHAT Kemenkes FHIR R4 Multi-Tenant & Single-Tenant'
WHERE NOT EXISTS (SELECT 1 FROM route_groups WHERE name = 'SATUSEHAT KEMENKES');

-- 2. Channel: SATUSEHAT Multi-Tenant Gateway
-- Menyimpan variabel channel ORG_ID, CLIENT_ID, CLIENT_SECRET, AUTH_URL, FHIR_BASE_URL
DELETE FROM routes WHERE id = 'satusehat';
INSERT INTO routes (id, name, description, group_name, active, variables)
VALUES (
    'satusehat',
    'SATUSEHAT Multi-Tenant Gateway',
    '1 Ingress Gateway untuk Multi-RS & 19 Resources SATUSEHAT Staging dengan Token Auto-Refresh',
    'SATUSEHAT KEMENKES',
    1,
    '{"ORG_ID":"YOUR_SATUSEHAT_ORG_ID","CLIENT_ID":"YOUR_SATUSEHAT_CLIENT_ID","CLIENT_SECRET":"YOUR_SATUSEHAT_CLIENT_SECRET","AUTH_URL":"https://api-satusehat-stg.kemkes.go.id/oauth2/v1/accesstoken?grant_type=client_credentials","FHIR_BASE_URL":"https://api-satusehat-stg.kemkes.go.id/fhir-r4/v1","ENV":"STAGING"}'
);

-- 3. Endpoints untuk Route 'satusehat'
DELETE FROM endpoints WHERE route_id = 'satusehat';

-- 3a. Source Endpoint (HTTP POST Ingress)
INSERT INTO endpoints (id, route_id, role, protocol, format, config)
VALUES (
    'ep-satusehat-src',
    'satusehat',
    'source',
    'http',
    'json',
    '{"path":"/ingress/satusehat","ackBehavior":"deferred"}'
);

-- 3b. Target Destination: SATUSEHAT STG
INSERT INTO endpoints (id, route_id, role, protocol, format, config)
VALUES (
    'ep-satusehat-tgt',
    'satusehat',
    'target',
    'http',
    'json',
    '{"name":"SATUSEHAT STG","enabled":true,"url":"https://api-satusehat-stg.kemkes.go.id/fhir-r4/v1","host":"https://api-satusehat-stg.kemkes.go.id","path":"/fhir-r4/v1","authType":"bearer","token":"{{SATUSEHAT_TOKEN}}","org_id":"{{ORG_ID}}","client_id":"{{CLIENT_ID}}","client_secret":"{{CLIENT_SECRET}}"}'
);

-- 4. Transformer: Dynamic Dispatch & Otomatisasi Token / Refresh Token
DELETE FROM transformers WHERE route_id = 'satusehat';
INSERT INTO transformers (id, route_id, source_script, script, inbound_template, outbound_template)
VALUES (
    'tr-satusehat',
    'satusehat',
    '// 1. Parse Inbound Request from SIMRS
var raw = JSON.parse(input);

// Extract routing parameters
var hospitalCode = raw.hospital_code || raw.hospitalCode || raw.tenant_id || "";
var resourceType = raw.resource || raw.resourceType || "";
var httpMethod = (raw.method || "POST").toUpperCase();
var resourceId = raw.resource_id || raw.id || "";
var fhirData = raw.data || raw.payload || raw;

if (!resourceType && fhirData && fhirData.resourceType) {
    resourceType = fhirData.resourceType;
}
if (!resourceType) {
    throw new Error("Missing resourceType in request payload (e.g. Encounter, Patient, Condition, Observation)");
}

// 2. Resolve Credentials (org_id, client_id, client_secret)
var orgId = "";
var clientId = "";
var clientSecret = "";

if (hospitalCode) {
    var tenantsJson = getGlobalVar("TENANT_CREDENTIALS");
    if (tenantsJson) {
        try {
            var tenants = JSON.parse(tenantsJson);
            var tenant = tenants[hospitalCode];
            if (tenant) {
                var satusehatCfg = tenant.satusehat || tenant;
                orgId = satusehatCfg.org_id || "";
                clientId = satusehatCfg.client_id || "";
                clientSecret = satusehatCfg.client_secret || "";
            }
        } catch (e) {}
    }
}

if (!clientId || !clientSecret) {
    orgId = orgId || getRouteVar("ORG_ID") || getGlobalVar("SATUSEHAT_ORG_ID") || "";
    clientId = clientId || getRouteVar("CLIENT_ID") || getGlobalVar("SATUSEHAT_CLIENT_ID") || "";
    clientSecret = clientSecret || getRouteVar("CLIENT_SECRET") || getGlobalVar("SATUSEHAT_CLIENT_SECRET") || "";
}

if (!clientId || !clientSecret) {
    throw new Error("SATUSEHAT STG credentials missing: configure ORG_ID, CLIENT_ID, CLIENT_SECRET in Channel Variables or TENANT_CREDENTIALS for: " + (hospitalCode || "DEFAULT"));
}

// 3. Automated OAuth2 Token & Auto-Refresh Management (In-Memory Cache with TTL)
var cacheKey = "satusehat_stg_token_" + (hospitalCode || orgId || clientId);
var token = cache.get(cacheKey);

if (!token) {
    var authUrl = getRouteVar("AUTH_URL") || getGlobalVar("SATUSEHAT_AUTH_URL") || "https://api-satusehat-stg.kemkes.go.id/oauth2/v1/accesstoken?grant_type=client_credentials";
    var formBody = "client_id=" + encodeURIComponent(clientId) + "&client_secret=" + encodeURIComponent(clientSecret);
    
    var authRes = http.request({
        url: authUrl,
        method: "POST",
        headers: {
            "Content-Type": "application/x-www-form-urlencoded"
        },
        body: formBody,
        timeout: 15
    });

    if (authRes.status !== 200) {
        throw new Error("Failed to authenticate with SATUSEHAT STG (" + authRes.status + "): " + authRes.body);
    }

    var authData = JSON.parse(authRes.body);
    if (!authData.access_token) {
        throw new Error("SATUSEHAT STG Auth response missing access_token: " + authRes.body);
    }

    token = authData.access_token;
    var expiresIn = parseInt(authData.expires_in, 10) || 3600;
    // Auto-refresh token 300 seconds (5 mins) before expiry
    var cacheTtl = Math.max(300, expiresIn - 300);
    cache.set(cacheKey, token, cacheTtl);
}

// 4. Enrich FHIR Resource with Organization ID if applicable
if (orgId && typeof fhirData === "object" && fhirData !== null) {
    if (resourceType === "Encounter" && !fhirData.serviceProvider) {
        fhirData.serviceProvider = {
            "reference": "Organization/" + orgId
        };
    }
    delete fhirData.hospital_code;
    delete fhirData.hospitalCode;
    delete fhirData.tenant_id;
    delete fhirData.resource;
    delete fhirData.resource_id;
    delete fhirData.method;
}

// 5. Build Dynamic SATUSEHAT STG Target URL
var rawBaseUrl = getRouteVar("FHIR_BASE_URL") || getGlobalVar("SATUSEHAT_FHIR_URL") || "https://api-satusehat-stg.kemkes.go.id/fhir-r4/v1";
var baseUrl = rawBaseUrl.replace(/\/+$/, "");
var targetUrl = baseUrl + "/" + resourceType;
if (resourceId && (httpMethod === "PUT" || httpMethod === "GET")) {
    targetUrl += "/" + resourceId;
}

// 6. Return Dynamic Dispatch Envelope to Engine
return JSON.stringify({
    __dispatch: {
        url: targetUrl,
        method: httpMethod,
        headers: {
            "Authorization": "Bearer " + token,
            "Content-Type": "application/json"
        }
    },
    payload: fhirData
});',
    '// Same script executed for destination
// 1. Parse Inbound Request from SIMRS
var raw = JSON.parse(input);

var hospitalCode = raw.hospital_code || raw.hospitalCode || raw.tenant_id || "";
var resourceType = raw.resource || raw.resourceType || "";
var httpMethod = (raw.method || "POST").toUpperCase();
var resourceId = raw.resource_id || raw.id || "";
var fhirData = raw.data || raw.payload || raw;

if (!resourceType && fhirData && fhirData.resourceType) {
    resourceType = fhirData.resourceType;
}
if (!resourceType) {
    throw new Error("Missing resourceType in request payload (e.g. Encounter, Patient, Condition, Observation)");
}

var orgId = "";
var clientId = "";
var clientSecret = "";

if (hospitalCode) {
    var tenantsJson = getGlobalVar("TENANT_CREDENTIALS");
    if (tenantsJson) {
        try {
            var tenants = JSON.parse(tenantsJson);
            var tenant = tenants[hospitalCode];
            if (tenant) {
                var satusehatCfg = tenant.satusehat || tenant;
                orgId = satusehatCfg.org_id || "";
                clientId = satusehatCfg.client_id || "";
                clientSecret = satusehatCfg.client_secret || "";
            }
        } catch (e) {}
    }
}

if (!clientId || !clientSecret) {
    orgId = orgId || getRouteVar("ORG_ID") || getGlobalVar("SATUSEHAT_ORG_ID") || "";
    clientId = clientId || getRouteVar("CLIENT_ID") || getGlobalVar("SATUSEHAT_CLIENT_ID") || "";
    clientSecret = clientSecret || getRouteVar("CLIENT_SECRET") || getGlobalVar("SATUSEHAT_CLIENT_SECRET") || "";
}

if (!clientId || !clientSecret) {
    throw new Error("SATUSEHAT STG credentials missing: configure ORG_ID, CLIENT_ID, CLIENT_SECRET in Channel Variables or TENANT_CREDENTIALS for: " + (hospitalCode || "DEFAULT"));
}

var cacheKey = "satusehat_stg_token_" + (hospitalCode || orgId || clientId);
var token = cache.get(cacheKey);

if (!token) {
    var authUrl = getRouteVar("AUTH_URL") || getGlobalVar("SATUSEHAT_AUTH_URL") || "https://api-satusehat-stg.kemkes.go.id/oauth2/v1/accesstoken?grant_type=client_credentials";
    var formBody = "client_id=" + encodeURIComponent(clientId) + "&client_secret=" + encodeURIComponent(clientSecret);
    
    var authRes = http.request({
        url: authUrl,
        method: "POST",
        headers: {
            "Content-Type": "application/x-www-form-urlencoded"
        },
        body: formBody,
        timeout: 15
    });

    if (authRes.status !== 200) {
        throw new Error("Failed to authenticate with SATUSEHAT STG (" + authRes.status + "): " + authRes.body);
    }

    var authData = JSON.parse(authRes.body);
    if (!authData.access_token) {
        throw new Error("SATUSEHAT STG Auth response missing access_token: " + authRes.body);
    }

    token = authData.access_token;
    var expiresIn = parseInt(authData.expires_in, 10) || 3600;
    var cacheTtl = Math.max(300, expiresIn - 300);
    cache.set(cacheKey, token, cacheTtl);
}

if (orgId && typeof fhirData === "object" && fhirData !== null) {
    if (resourceType === "Encounter" && !fhirData.serviceProvider) {
        fhirData.serviceProvider = {
            "reference": "Organization/" + orgId
        };
    }
    delete fhirData.hospital_code;
    delete fhirData.hospitalCode;
    delete fhirData.tenant_id;
    delete fhirData.resource;
    delete fhirData.resource_id;
    delete fhirData.method;
}

var rawBaseUrl = getRouteVar("FHIR_BASE_URL") || getGlobalVar("SATUSEHAT_FHIR_URL") || "https://api-satusehat-stg.kemkes.go.id/fhir-r4/v1";
var baseUrl = rawBaseUrl.replace(/\/+$/, "");
var targetUrl = baseUrl + "/" + resourceType;
if (resourceId && (httpMethod === "PUT" || httpMethod === "GET")) {
    targetUrl += "/" + resourceId;
}

return JSON.stringify({
    __dispatch: {
        url: targetUrl,
        method: httpMethod,
        headers: {
            "Authorization": "Bearer " + token,
            "Content-Type": "application/json"
        }
    },
    payload: fhirData
});',
    '{}',
    '{}'
);

-- 5. Global Variables untuk SATUSEHAT Staging
INSERT INTO global_variables (id, key_name, value_data, is_secret, description)
SELECT 'var-satusehat-auth', 'SATUSEHAT_AUTH_URL', 'https://api-satusehat-stg.kemkes.go.id/oauth2/v1/accesstoken?grant_type=client_credentials', 0, 'URL OAuth2 Access Token SATUSEHAT Kemenkes Staging'
WHERE NOT EXISTS (SELECT 1 FROM global_variables WHERE key_name = 'SATUSEHAT_AUTH_URL');

INSERT INTO global_variables (id, key_name, value_data, is_secret, description)
SELECT 'var-satusehat-fhir', 'SATUSEHAT_FHIR_URL', 'https://api-satusehat-stg.kemkes.go.id/fhir-r4/v1', 0, 'Base URL FHIR R4 Resources SATUSEHAT Staging'
WHERE NOT EXISTS (SELECT 1 FROM global_variables WHERE key_name = 'SATUSEHAT_FHIR_URL');
