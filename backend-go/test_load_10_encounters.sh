#!/usr/bin/env bash

# ============================================================================
# SATUSEHAT STG - BASH LOAD TEST 10 ENCOUNTERS
# ============================================================================
set -e

# Load local .env if available
if [ -f "backend-go/.env" ]; then
    set -a
    source backend-go/.env
    set +a
elif [ -f ".env" ]; then
    set -a
    source .env
    set +a
fi

INGRESS_URL="${INGRESS_URL:-http://localhost:3001/ingress/satusehat}"
ORG_ID="${SATUSEHAT_ORG_ID:-YOUR_SATUSEHAT_ORG_ID}"
PRACTITIONER_REF="Practitioner/N10000001"
PATIENT_REF="Patient/P20395156715"
LOCATION_REF="Location/4fa0413f-403d-40fb-8be8-1e690a0e8931"

echo "=============================================================================="
echo "🚀 SATUSEHAT STG - BASH LOAD TEST 10 ENCOUNTERS"
echo "=============================================================================="
echo "Ingress Target  : $INGRESS_URL"
echo "Organization ID : $ORG_ID"
echo "Practitioner    : $PRACTITIONER_REF (Default SATUSEHAT STG)"
echo "Location        : $LOCATION_REF"
echo "Patient         : $PATIENT_REF"
echo "=============================================================================="
echo ""

SUCCESS_COUNT=0
FAIL_COUNT=0

for i in {1..10}; do
  IDX=$(printf "%02d" $i)
  NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  UUID=$(uuidgen | tr '[:upper:]' '[:lower:]' | cut -c1-6)
  REG_NO="REG-20261031-${IDX}-${UUID}"

  PAYLOAD=$(cat <<EOF
{
  "resourceType": "Encounter",
  "meta": {
    "profile": [
      "https://fhir.kemkes.go.id/r4/StructureDefinition/Encounter"
    ]
  },
  "status": "arrived",
  "class": {
    "system": "http://terminology.hl7.org/CodeSystem/v3-ActCode",
    "code": "AMB",
    "display": "Rawat Jalan (Ambulatory)"
  },
  "subject": {
    "reference": "$PATIENT_REF",
    "display": "Pasien Uji Coba $IDX"
  },
  "participant": [
    {
      "type": [
        {
          "coding": [
            {
              "system": "http://terminology.hl7.org/CodeSystem/v3-ParticipationType",
              "code": "ATND",
              "display": "attender"
            }
          ]
        }
      ],
      "individual": {
        "reference": "$PRACTITIONER_REF",
        "display": "Dokter Pemeriksa SATUSEHAT Staging"
      }
    }
  ],
  "period": {
    "start": "$NOW"
  },
  "location": [
    {
      "location": {
        "reference": "$LOCATION_REF",
        "display": "Poli Spesialis Luka"
      }
    }
  ],
  "statusHistory": [
    {
      "status": "arrived",
      "period": {
        "start": "$NOW"
      }
    }
  ],
  "serviceProvider": {
    "reference": "Organization/$ORG_ID"
  },
  "identifier": [
    {
      "system": "http://sys-ids.kemkes.go.id/encounter/$ORG_ID",
      "value": "$REG_NO"
    }
  ]
}
EOF
)

  START_TS=$(date +%s%3N 2>/dev/null || date +%s)
  RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$INGRESS_URL" \
    -H "Content-Type: application/json" \
    -d "$PAYLOAD")
  END_TS=$(date +%s%3N 2>/dev/null || date +%s)
  ELAPSED=$((END_TS - START_TS))

  HTTP_STATUS=$(echo "$RESP" | grep "HTTP_STATUS:" | cut -d':' -f2)
  BODY=$(echo "$RESP" | grep -v "HTTP_STATUS:")

  if [ "$HTTP_STATUS" = "200" ]; then
    ENC_ID=$(echo "$BODY" | grep -o '"id":"[^"]*"' | head -n 1 | cut -d':' -f2 | tr -d '"' || echo "-")
    echo -e "[$IDX/10] \033[32m✔ SUCCESS\033[0m (${ELAPSED}ms) | Reg: $REG_NO | Encounter ID: \033[36m$ENC_ID\033[0m"
    SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
  else
    echo -e "[$IDX/10] \033[31m✖ FAILED\033[0m (${ELAPSED}ms) | HTTP $HTTP_STATUS | $BODY"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi

  sleep 0.2
done

echo ""
echo "=============================================================================="
echo "📊 RINGKASAN HASIL LOAD TEST:"
echo "=============================================================================="
echo "Total Terkirim : 10"
echo -e "Berhasil       : \033[32m$SUCCESS_COUNT\033[0m"
echo -e "Gagal          : \033[31m$FAIL_COUNT\033[0m"
echo "=============================================================================="
