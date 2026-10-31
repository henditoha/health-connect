#!/usr/bin/env node

/**
 * ============================================================================
 * SATUSEHAT STG - LOAD TEST 10 ENCOUNTERS
 * ============================================================================
 * Mengirim 10 data FHIR Encounter ke Ingress Health-Connect:
 * Endpoint: POST http://localhost:3001/ingress/satusehat
 *
 * Parameter Terpasang:
 * - Organization: Organization/YOUR_SATUSEHAT_ORG_ID (via .env)
 * - Default Practitioner: Practitioner/N10000001 (dr. Dokter Pemeriksa)
 * - Default Patient: Patient/P20395156715
 * - Default Location: Location/4fa0413f-403d-40fb-8be8-1e690a0e8931 (Poli Spesialis Luka)
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

const PAYLOAD_DIR = path.join(__dirname, 'payloads', 'encounters');
if (!fs.existsSync(PAYLOAD_DIR)) {
  fs.mkdirSync(PAYLOAD_DIR, { recursive: true });
}

function generateEncounter(index) {
  const padIndex = String(index).padStart(2, '0');
  const now = new Date();
  now.setMinutes(now.getMinutes() - (10 - index) * 5); // Interval 5 menit
  const isoTime = now.toISOString();

  const regNo = `REG-20261031-${padIndex}-${Math.random().toString(36).substring(2, 6).toUpperCase()}`;

  return {
    resourceType: 'Encounter',
    meta: {
      profile: [
        'https://fhir.kemkes.go.id/r4/StructureDefinition/Encounter'
      ]
    },
    status: 'arrived',
    class: {
      system: 'http://terminology.hl7.org/CodeSystem/v3-ActCode',
      code: 'AMB',
      display: 'Rawat Jalan (Ambulatory)'
    },
    subject: {
      reference: PATIENT_REF,
      display: `Pasien Uji Coba ${padIndex}`
    },
    participant: [
      {
        type: [
          {
            coding: [
              {
                system: 'http://terminology.hl7.org/CodeSystem/v3-ParticipationType',
                code: 'ATND',
                display: 'attender'
              }
            ]
          }
        ],
        individual: {
          reference: PRACTITIONER_REF,
          display: 'Dokter Pemeriksa SATUSEHAT Staging'
        }
      }
    ],
    period: {
      start: isoTime
    },
    location: [
      {
        location: {
          reference: LOCATION_REF,
          display: 'Poli Spesialis Luka'
        }
      }
    ],
    statusHistory: [
      {
        status: 'arrived',
        period: {
          start: isoTime
        }
      }
    ],
    serviceProvider: {
      reference: `Organization/${ORG_ID}`
    },
    identifier: [
      {
        system: `http://sys-ids.kemkes.go.id/encounter/${ORG_ID}`,
        value: regNo
      }
    ]
  };
}

async function sendEncounter(encounter, index) {
  const padIndex = String(index).padStart(2, '0');
  const startTime = Date.now();

  try {
    const res = await fetch(INGRESS_URL, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json'
      },
      body: JSON.stringify(encounter)
    });

    const elapsed = Date.now() - startTime;
    const text = await res.text();

    let satusehatId = '-';
    let statusDesc = 'OK';
    let isSuccess = res.ok;

    try {
      // Parse output format: [SATUSEHAT STG]: {...}
      const jsonStart = text.indexOf('{');
      if (jsonStart !== -1) {
        const parsed = JSON.parse(text.substring(jsonStart));
        if (parsed.id) {
          satusehatId = parsed.id;
        }
        if (parsed.resourceType === 'OperationOutcome') {
          isSuccess = false;
          statusDesc = parsed.issue?.[0]?.diagnostics || parsed.issue?.[0]?.details?.text || 'Validation Error';
        }
      }
    } catch (e) {
      statusDesc = text.substring(0, 80);
    }

    if (isSuccess && res.status === 200) {
      console.log(`[${padIndex}/10] \x1b[32m✔ SUCCESS\x1b[0m (${elapsed}ms) | Reg: ${encounter.identifier[0].value} | Encounter ID: \x1b[36m${satusehatId}\x1b[0m`);
      return { success: true, elapsed, satusehatId, reg: encounter.identifier[0].value };
    } else {
      console.log(`[${padIndex}/10] \x1b[31m✖ FAILED\x1b[0m (${elapsed}ms) | HTTP ${res.status} | ${statusDesc}`);
      return { success: false, elapsed, error: statusDesc, reg: encounter.identifier[0].value };
    }
  } catch (err) {
    const elapsed = Date.now() - startTime;
    console.log(`[${padIndex}/10] \x1b[31m✖ ERROR\x1b[0m (${elapsed}ms) | ${err.message}`);
    return { success: false, elapsed, error: err.message, reg: encounter.identifier[0].value };
  }
}

async function main() {
  console.log('='.repeat(78));
  console.log('🚀 SATUSEHAT STG - LOAD TEST 10 ENCOUNTERS');
  console.log('='.repeat(78));
  console.log(`Ingress Target  : ${INGRESS_URL}`);
  console.log(`Organization ID : ${ORG_ID}`);
  console.log(`Practitioner    : ${PRACTITIONER_REF} (Default SATUSEHAT STG)`);
  console.log(`Location        : ${LOCATION_REF}`);
  console.log(`Patient         : ${PATIENT_REF}`);
  console.log(`Payload Folder  : ${PAYLOAD_DIR}`);
  console.log('='.repeat(78));

  // 1. Generate & Save 10 Payload Files
  console.log('\n📁 Menyiapkan 10 file payload JSON Encounter...');
  const encounters = [];
  for (let i = 1; i <= 10; i++) {
    const enc = generateEncounter(i);
    encounters.push(enc);
    const filename = `encounter_${String(i).padStart(2, '0')}.json`;
    fs.writeFileSync(path.join(PAYLOAD_DIR, filename), JSON.stringify(enc, null, 2), 'utf-8');
  }
  console.log(`✔ 10 file payload tersimpan di ${PAYLOAD_DIR}\n`);

  // 2. Kirim 10 Encounter
  console.log('📡 Memulai pengiriman 10 Encounter ke Health-Connect Gateway...\n');
  const results = [];
  const startAll = Date.now();

  for (let i = 0; i < encounters.length; i++) {
    const result = await sendEncounter(encounters[i], i + 1);
    results.push(result);
    // Delay 200ms agar log terbaca rapi
    await new Promise(r => setTimeout(r, 200));
  }

  const totalTime = Date.now() - startAll;
  const successes = results.filter(r => r.success).length;
  const failures = results.length - successes;
  const avgLatency = Math.round(results.reduce((acc, r) => acc + r.elapsed, 0) / results.length);

  console.log('\n' + '='.repeat(78));
  console.log('📊 RINGKASAN HASIL LOAD TEST:');
  console.log('='.repeat(78));
  console.log(`Total Terkirim : ${results.length}`);
  console.log(`Berhasil (200) : \x1b[32m${successes}\x1b[0m`);
  console.log(`Gagal          : ${failures > 0 ? `\x1b[31m${failures}\x1b[0m` : '0'}`);
  console.log(`Rata-rata Waktu: ${avgLatency} ms / request`);
  console.log(`Total Durasi   : ${(totalTime / 1000).toFixed(2)} detik`);
  console.log('='.repeat(78));

  if (successes > 0) {
    console.log('\n🎉 ID Encounter yang berhasil dibuat di SATUSEHAT STG:');
    results.filter(r => r.success).forEach((r, idx) => {
      console.log(`   ${idx + 1}. ${r.reg} -> ${r.satusehatId}`);
    });
  }
}

main().catch(err => {
  console.error('Fatal error:', err);
  process.exit(1);
});
