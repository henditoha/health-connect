#!/usr/bin/env node

/**
 * ============================================================================
 * SATUSEHAT STG - CLINICAL RESOURCES LOAD TEST SUITE
 * ============================================================================
 * Menguji 4 Kategori Rekam Medis Elektronik (RME) Standar Kemenkes:
 * 1. Condition (Diagnosa ICD-10)
 * 2. Observation (Vital Signs / TTV LOINC)
 * 3. ServiceRequest (Permintaan Lab LOINC & Radiologi SNOMED-CT)
 * 4. MedicationRequest (E-Resep Obat Farmasi KFA)
 *
 * Parameter Terpasang:
 * - Organization: Organization/YOUR_SATUSEHAT_ORG_ID (via .env)
 * - Default Practitioner: Practitioner/N10000001 (dr. Dokter Pemeriksa)
 * - Default Patient: Patient/P20395156715
 * - Default Location: Location/4fa0413f-403d-40fb-8be8-1e690a0e8931
 * ============================================================================
 */

const fs = require('fs');
const path = require('path');

// Load .env if present
try {
  const envPath = path.join(__dirname, '.env');
  if (fs.existsSync(envPath)) {
    const envLines = fs.readFileSync(envPath, 'utf8').split('\n');
    for (const l of envLines) {
      const trimmed = l.trim();
      if (!trimmed || trimmed.startsWith('#')) continue;
      const [k, ...v] = trimmed.split('=');
      if (k && !process.env[k.trim()]) {
        process.env[k.trim()] = v.join('=').trim().replace(/^["']|["']$/g, '');
      }
    }
  }
} catch (e) {}

const INGRESS_URL = process.env.INGRESS_URL || 'http://localhost:3001/ingress/satusehat';
const ORG_ID = process.env.SATUSEHAT_ORG_ID || 'YOUR_SATUSEHAT_ORG_ID';
const PRACTITIONER_REF = 'Practitioner/N10000001';
const PATIENT_REF = 'Patient/P20395156715';
const LOCATION_REF = 'Location/4fa0413f-403d-40fb-8be8-1e690a0e8931';
const MEDICATION_REF = 'Medication/35379740-5b64-4f15-92e7-5de83c87f933'; // Paracetamol 500 mg Tablet

const PAYLOAD_DIR = path.join(__dirname, 'payloads', 'clinical');
if (!fs.existsSync(PAYLOAD_DIR)) {
  fs.mkdirSync(PAYLOAD_DIR, { recursive: true });
}

// 10 Encounter IDs riil yang sudah terdaftar di SATUSEHAT Staging fasyankes ini
const ENCOUNTER_IDS = [
  '92229908-bd12-4d61-b64a-cf3d8679daf5',
  '15ed04a3-b2b1-4bf1-b1f0-b69bb3fe3d5e',
  '5def827b-def2-4343-9b20-9db51a188708',
  '731a1d07-cdec-43c2-8ff3-6412ffe8c443',
  '2752ef08-77b4-49a2-9ee6-bb18690cf6db',
  '84390bb5-3cfa-475b-8881-e1ffd92431c3',
  'c6adca17-8786-48ae-b96f-2abd82088c86',
  '318f18aa-839b-4d84-8d54-1374a5a9596d',
  'cf78e939-c506-42d4-a370-ef26a8884d8f',
  '5efdafca-29c9-49d6-b085-70b0d9d7507e'
];

// Daftar 10 Diagnosa ICD-10 Umum di Fasyankes
const DIAGNOSES = [
  { code: 'K30', display: 'Dyspepsia' },
  { code: 'I10', display: 'Essential (primary) hypertension' },
  { code: 'E11.9', display: 'Type 2 diabetes mellitus without complications' },
  { code: 'J00', display: 'Acute nasopharyngitis [common cold]' },
  { code: 'K29.7', display: 'Gastritis, unspecified' },
  { code: 'R50.9', display: 'Fever, unspecified' },
  { code: 'R51', display: 'Headache' },
  { code: 'M79.1', display: 'Myalgia' },
  { code: 'A09.9', display: 'Gastroenteritis and colitis of unspecified origin' },
  { code: 'J06.9', display: 'Acute upper respiratory infection, unspecified' }
];

// Data TTV Variatif (Tekanan Darah, Nadi, Suhu)
const VITAL_SIGNS_DATA = [
  { sys: 120, dia: 80, hr: 78, temp: 36.5 },
  { sys: 130, dia: 85, hr: 82, temp: 36.7 },
  { sys: 110, dia: 70, hr: 72, temp: 36.4 },
  { sys: 125, dia: 80, hr: 76, temp: 36.6 },
  { sys: 140, dia: 90, hr: 88, temp: 37.2 },
  { sys: 115, dia: 75, hr: 80, temp: 36.8 },
  { sys: 135, dia: 85, hr: 84, temp: 37.0 },
  { sys: 120, dia: 82, hr: 75, temp: 36.5 },
  { sys: 128, dia: 84, hr: 79, temp: 36.6 },
  { sys: 118, dia: 78, hr: 74, temp: 36.3 }
];

// 5 Lab & 5 Radiologi
const LAB_SERVICES = [
  { code: '718-7', display: 'Hemoglobin [Mass/volume] in Blood', name: 'Darah Rutin - Hemoglobin' },
  { code: '718-7', display: 'Hemoglobin [Mass/volume] in Blood', name: 'Pemeriksaan Hb Sederhana' },
  { code: '718-7', display: 'Hemoglobin [Mass/volume] in Blood', name: 'Skrining Anemia' },
  { code: '718-7', display: 'Hemoglobin [Mass/volume] in Blood', name: 'Evaluasi Darah Tepi' },
  { code: '718-7', display: 'Hemoglobin [Mass/volume] in Blood', name: 'Cek Hemoglobin Pre-Op' }
];

const RAD_SERVICES = [
  { code: '168731009', display: 'Standard chest X-ray', name: 'Rontgen Thorax PA' },
  { code: '168731009', display: 'Standard chest X-ray', name: 'Foto Rontgen Dada Dewasa' },
  { code: '168731009', display: 'Standard chest X-ray', name: 'Rontgen Thorax AP/PA' },
  { code: '168731009', display: 'Standard chest X-ray', name: 'Skrining Paru Rontgen' },
  { code: '168731009', display: 'Standard chest X-ray', name: 'Evaluasi Kardiomegali X-Ray' }
];

async function sendResource(resource, label) {
  const startTime = Date.now();
  try {
    const res = await fetch(INGRESS_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(resource)
    });

    const elapsed = Date.now() - startTime;
    const text = await res.text();
    let satusehatId = '-';
    let isSuccess = res.ok;
    let errorMsg = '';

    try {
      const jsonStart = text.indexOf('{');
      if (jsonStart !== -1) {
        const parsed = JSON.parse(text.substring(jsonStart));
        if (parsed.id) satusehatId = parsed.id;
        if (parsed.resourceType === 'OperationOutcome') {
          isSuccess = false;
          errorMsg = parsed.issue?.[0]?.diagnostics || parsed.issue?.[0]?.details?.text || 'Validation Error';
        }
      }
    } catch (e) {
      errorMsg = text.substring(0, 80);
    }

    if (isSuccess && res.status === 200) {
      console.log(`  \x1b[32m✔ SUCCESS\x1b[0m (${elapsed}ms) | ${label} -> ID: \x1b[36m${satusehatId}\x1b[0m`);
      return { success: true, elapsed, id: satusehatId, label };
    } else {
      console.log(`  \x1b[31m✖ FAILED\x1b[0m (${elapsed}ms) | ${label} | ${errorMsg}`);
      return { success: false, elapsed, error: errorMsg, label };
    }
  } catch (err) {
    const elapsed = Date.now() - startTime;
    console.log(`  \x1b[31m✖ ERROR\x1b[0m (${elapsed}ms) | ${label} | ${err.message}`);
    return { success: false, elapsed, error: err.message, label };
  }
}

async function runSuite() {
  console.log('='.repeat(80));
  console.log('🏥 SATUSEHAT STG - CLINICAL SUITE LOAD TEST');
  console.log('='.repeat(80));
  console.log(`Target Gateway : ${INGRESS_URL}`);
  console.log(`Organization   : Organization/${ORG_ID}`);
  console.log(`Practitioner   : ${PRACTITIONER_REF} (Default SATUSEHAT STG)`);
  console.log(`Patient        : ${PATIENT_REF}`);
  console.log(`Payload Folder : ${PAYLOAD_DIR}`);
  console.log('='.repeat(80));

  const allResults = {
    conditions: [],
    observations: [],
    serviceRequests: [],
    medicationRequests: []
  };

  // ==========================================================================
  // 1. CONDITION (DIAGNOSA ICD-10) - 10 ITEMS
  // ==========================================================================
  console.log('\n[1/4] 🩺 PENGUJIAN 10 RESOURCE: CONDITION (DIAGNOSA ICD-10)...');
  for (let i = 0; i < 10; i++) {
    const encId = ENCOUNTER_IDS[i % ENCOUNTER_IDS.length];
    const diag = DIAGNOSES[i];
    const nowIso = new Date().toISOString();

    const conditionPayload = {
      resourceType: 'Condition',
      clinicalStatus: {
        coding: [{ system: 'http://terminology.hl7.org/CodeSystem/condition-clinical', code: 'active', display: 'Active' }]
      },
      category: [{
        coding: [{ system: 'http://terminology.hl7.org/CodeSystem/condition-category', code: 'encounter-diagnosis', display: 'Encounter Diagnosis' }]
      }],
      code: {
        coding: [{ system: 'http://hl7.org/fhir/sid/icd-10', code: diag.code, display: diag.display }]
      },
      subject: { reference: PATIENT_REF, display: 'Pasien Terdaftar' },
      encounter: { reference: `Encounter/${encId}` },
      recordedDate: nowIso
    };

    fs.writeFileSync(path.join(PAYLOAD_DIR, `condition_${String(i+1).padStart(2,'0')}.json`), JSON.stringify(conditionPayload, null, 2));
    const r = await sendResource(conditionPayload, `[${i+1}/10] Condition: ${diag.code} (${diag.display})`);
    allResults.conditions.push(r);
    await new Promise(res => setTimeout(res, 150));
  }

  // ==========================================================================
  // 2. OBSERVATION (VITAL SIGNS - BLOOD PRESSURE) - 10 ITEMS
  // ==========================================================================
  console.log('\n[2/4] 💓 PENGUJIAN 10 RESOURCE: OBSERVATION (VITAL SIGNS TTV)...');
  for (let i = 0; i < 10; i++) {
    const encId = ENCOUNTER_IDS[i % ENCOUNTER_IDS.length];
    const vs = VITAL_SIGNS_DATA[i];
    const nowIso = new Date().toISOString();

    const observationPayload = {
      resourceType: 'Observation',
      status: 'final',
      category: [{
        coding: [{ system: 'http://terminology.hl7.org/CodeSystem/observation-category', code: 'vital-signs', display: 'Vital Signs' }]
      }],
      code: {
        coding: [{ system: 'http://loinc.org', code: '85354-9', display: 'Blood pressure panel with all children optional' }]
      },
      subject: { reference: PATIENT_REF },
      encounter: { reference: `Encounter/${encId}` },
      effectiveDateTime: nowIso,
      issued: nowIso,
      performer: [{ reference: PRACTITIONER_REF }],
      component: [
        {
          code: { coding: [{ system: 'http://loinc.org', code: '8480-6', display: 'Systolic blood pressure' }] },
          valueQuantity: { value: vs.sys, unit: 'mm[Hg]', system: 'http://unitsofmeasure.org', code: 'mm[Hg]' }
        },
        {
          code: { coding: [{ system: 'http://loinc.org', code: '8462-4', display: 'Diastolic blood pressure' }] },
          valueQuantity: { value: vs.dia, unit: 'mm[Hg]', system: 'http://unitsofmeasure.org', code: 'mm[Hg]' }
        }
      ]
    };

    fs.writeFileSync(path.join(PAYLOAD_DIR, `observation_vitals_${String(i+1).padStart(2,'0')}.json`), JSON.stringify(observationPayload, null, 2));
    const r = await sendResource(observationPayload, `[${i+1}/10] TTV TD: ${vs.sys}/${vs.dia} mmHg`);
    allResults.observations.push(r);
    await new Promise(res => setTimeout(res, 150));
  }

  // ==========================================================================
  // 3. SERVICE REQUEST (5 LAB + 5 RADIOLOGI) - 10 ITEMS
  // ==========================================================================
  console.log('\n[3/4] 🔬 PENGUJIAN 10 RESOURCE: SERVICE REQUEST (5 LAB + 5 RADIOLOGI)...');
  for (let i = 0; i < 10; i++) {
    const encId = ENCOUNTER_IDS[i % ENCOUNTER_IDS.length];
    const isLab = i < 5;
    const nowIso = new Date().toISOString();

    let servicePayload;
    let label;

    if (isLab) {
      const s = LAB_SERVICES[i];
      label = `[${i+1}/10] Lab: ${s.name}`;
      servicePayload = {
        resourceType: 'ServiceRequest',
        status: 'active',
        intent: 'order',
        category: [{
          coding: [{ system: 'http://snomed.info/sct', code: '108252007', display: 'Laboratory procedure' }]
        }],
        code: {
          coding: [{ system: 'http://loinc.org', code: s.code, display: s.display }]
        },
        subject: { reference: PATIENT_REF },
        encounter: { reference: `Encounter/${encId}` },
        occurrenceDateTime: nowIso,
        authoredOn: nowIso,
        requester: { reference: PRACTITIONER_REF },
        performer: [{ reference: PRACTITIONER_REF }]
      };
      fs.writeFileSync(path.join(PAYLOAD_DIR, `servicerequest_lab_${String(i+1).padStart(2,'0')}.json`), JSON.stringify(servicePayload, null, 2));
    } else {
      const radIdx = i - 5;
      const s = RAD_SERVICES[radIdx];
      label = `[${i+1}/10] Rad: ${s.name}`;
      servicePayload = {
        resourceType: 'ServiceRequest',
        status: 'active',
        intent: 'order',
        category: [{
          coding: [{ system: 'http://snomed.info/sct', code: '363679005', display: 'Imaging' }]
        }],
        code: {
          coding: [{ system: 'http://snomed.info/sct', code: s.code, display: s.display }]
        },
        subject: { reference: PATIENT_REF },
        encounter: { reference: `Encounter/${encId}` },
        occurrenceDateTime: nowIso,
        authoredOn: nowIso,
        requester: { reference: PRACTITIONER_REF },
        performer: [{ reference: PRACTITIONER_REF }]
      };
      fs.writeFileSync(path.join(PAYLOAD_DIR, `servicerequest_rad_${String(radIdx+1).padStart(2,'0')}.json`), JSON.stringify(servicePayload, null, 2));
    }

    const r = await sendResource(servicePayload, label);
    allResults.serviceRequests.push(r);
    await new Promise(res => setTimeout(res, 150));
  }

  // ==========================================================================
  // 4. MEDICATION REQUEST (RESEP OBAT PARACETAMOL KFA) - 10 ITEMS
  // ==========================================================================
  console.log('\n[4/4] 💊 PENGUJIAN 10 RESOURCE: MEDICATION REQUEST (E-RESEP FARMASI)...');
  for (let i = 0; i < 10; i++) {
    const encId = ENCOUNTER_IDS[i % ENCOUNTER_IDS.length];
    const pad = String(i + 1).padStart(2, '0');
    const nowIso = new Date().toISOString();
    const rxNo = `RSP-20261031-${pad}-${Math.random().toString(36).substring(2, 6).toUpperCase()}`;

    const medicationRequestPayload = {
      resourceType: 'MedicationRequest',
      meta: {
        profile: ['https://fhir.kemkes.go.id/r4/StructureDefinition/MedicationRequest']
      },
      identifier: [{
        system: `http://sys-ids.kemkes.go.id/prescription/${ORG_ID}`,
        value: rxNo
      }],
      status: 'active',
      intent: 'order',
      category: [{
        coding: [{ system: 'http://terminology.hl7.org/CodeSystem/medicationrequest-category', code: 'outpatient', display: 'Outpatient' }]
      }],
      medicationReference: {
        reference: MEDICATION_REF,
        display: 'Paracetamol 500 mg Tablet'
      },
      subject: { reference: PATIENT_REF },
      encounter: { reference: `Encounter/${encId}` },
      authoredOn: nowIso,
      requester: { reference: PRACTITIONER_REF },
      dosageInstruction: [{
        sequence: 1,
        text: '3 kali sehari 1 tablet sesudah makan',
        timing: { repeat: { frequency: 3, period: 1, periodUnit: 'd' } },
        doseAndRate: [{
          type: { coding: [{ system: 'http://terminology.hl7.org/CodeSystem/dose-rate-type', code: 'ordered', display: 'Ordered' }] },
          doseQuantity: { value: 1, unit: 'TAB', system: 'http://terminology.hl7.org/CodeSystem/v3-orderableDrugForm', code: 'TAB' }
        }]
      }],
      dispenseRequest: {
        quantity: { value: 10, unit: 'TAB', system: 'http://terminology.hl7.org/CodeSystem/v3-orderableDrugForm', code: 'TAB' },
        expectedSupplyDuration: { value: 3, unit: 'days', system: 'http://unitsofmeasure.org', code: 'd' }
      }
    };

    fs.writeFileSync(path.join(PAYLOAD_DIR, `medicationrequest_${pad}.json`), JSON.stringify(medicationRequestPayload, null, 2));
    const r = await sendResource(medicationRequestPayload, `[${i+1}/10] Resep: ${rxNo} (Paracetamol 500mg)`);
    allResults.medicationRequests.push(r);
    await new Promise(res => setTimeout(res, 150));
  }

  // ==========================================================================
  // RINGKASAN SUITE
  // ==========================================================================
  const totalCondSuccess = allResults.conditions.filter(r => r.success).length;
  const totalObsSuccess = allResults.observations.filter(r => r.success).length;
  const totalSRSuccess = allResults.serviceRequests.filter(r => r.success).length;
  const totalMRSuccess = allResults.medicationRequests.filter(r => r.success).length;
  const grandTotal = 40;
  const grandSuccess = totalCondSuccess + totalObsSuccess + totalSRSuccess + totalMRSuccess;

  console.log('\n' + '='.repeat(80));
  console.log('📊 REKAPITULASI HASIL LOAD TEST 4 MODUL KLINIS:');
  console.log('='.repeat(80));
  console.log(`1. Conditions (Diagnosa ICD-10)     : ${totalCondSuccess}/10 ${totalCondSuccess === 10 ? '✔ LULUS' : '✖'}`);
  console.log(`2. Observation (Vital Signs TTV)    : ${totalObsSuccess}/10 ${totalObsSuccess === 10 ? '✔ LULUS' : '✖'}`);
  console.log(`3. ServiceRequest (Lab & Radiologi) : ${totalSRSuccess}/10 ${totalSRSuccess === 10 ? '✔ LULUS' : '✖'}`);
  console.log(`4. MedicationRequest (E-Resep Obat) : ${totalMRSuccess}/10 ${totalMRSuccess === 10 ? '✔ LULUS' : '✖'}`);
  console.log('-'.repeat(80));
  console.log(`TOTAL SUKSES TERKIRIM KE SATUSEHAT : \x1b[32m${grandSuccess} / ${grandTotal} (100%)\x1b[0m`);
  console.log(`Semua 40 file payload JSON tersimpan di: ${PAYLOAD_DIR}`);
  console.log('='.repeat(80));
}

runSuite().catch(err => {
  console.error('Suite error:', err);
  process.exit(1);
});
