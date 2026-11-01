package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestDicomProcedureDescriptionAutoCopy(t *testing.T) {
	item := DicomWorklistItem{
		AccessionNumber: "ACC1001",
		PatientID:       "RM0099",
		PatientName:     "Sutrisno^Bambang",
		Modality:        "DX",
		ProcedureDesc:   "Foto Thorax AP/PA Dewasa",
		// study_desc, req_proc_desc, and series_desc intentionally left empty
	}

	item.ApplyProcedureDescriptionCopy()

	if item.ProcedureDesc != "Foto Thorax AP/PA Dewasa" {
		t.Fatalf("Expected ProcedureDesc to be preserved, got '%s'", item.ProcedureDesc)
	}
	if item.StudyDesc != item.ProcedureDesc {
		t.Fatalf("Expected StudyDesc to be auto-copied from ProcedureDesc, got '%s'", item.StudyDesc)
	}
	if item.ReqProcDesc != item.ProcedureDesc {
		t.Fatalf("Expected ReqProcDesc to be auto-copied from ProcedureDesc, got '%s'", item.ReqProcDesc)
	}
	if item.SeriesDesc != item.ProcedureDesc {
		t.Fatalf("Expected SeriesDesc to be auto-copied from ProcedureDesc, got '%s'", item.SeriesDesc)
	}
	if !strings.HasPrefix(item.StudyUID, "2.25.") {
		t.Fatalf("Expected ISO standard StudyUID with prefix 2.25., got '%s'", item.StudyUID)
	}
}

func TestDicomCEchoAndCFindNetworkHandshake(t *testing.T) {
	// Initialize test database
	if err := initDb(); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	// Clean up prior test data
	db.Exec("DELETE FROM dicom_worklist_items WHERE accession_number = 'ACC-TEST-999'")

	testPort := 11119
	server := &DicomServer{
		config: DicomServerConfig{
			Port:    testPort,
			AETitle: "HEALTH_CONNECT",
			Enabled: true,
		},
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start test DICOM server on port %d: %v", testPort, err)
	}
	defer server.Stop()

	// Seed test worklist item
	testItem := &DicomWorklistItem{
		AccessionNumber: "ACC-TEST-999",
		PatientID:       "RM-TEST-01",
		PatientName:     "Kusuma^Widodo",
		BirthDate:       "19850512",
		Gender:          "M",
		Modality:        "CT",
		ProcedureDesc:   "CT Scan Kepala Polos Non Kontras",
		ScheduledDate:   time.Now().Format("20060102"),
		ScheduledTime:   "100000",
		StationAETitle:  "CT_SCAN_01",
		Status:          "SCHEDULED",
	}
	if err := SaveDicomWorklistItem(testItem); err != nil {
		t.Fatalf("Failed to save test worklist item: %v", err)
	}

	// Connect as Modality SCU (Simulated X-Ray / CT Console)
	conn, err := net.DialTimeout("tcp", "127.0.0.1:11119", 2*time.Second)
	if err != nil {
		t.Fatalf("SCU failed to connect to DICOM SCP: %v", err)
	}
	defer conn.Close()

	// 1. Send A-ASSOCIATE-RQ
	assocRQPDU := buildTestAssociateRQ("CT_SCAN_01", "HEALTH_CONNECT")
	if _, err := conn.Write(assocRQPDU); err != nil {
		t.Fatalf("Failed to write A-ASSOCIATE-RQ: %v", err)
	}

	// Read A-ASSOCIATE-AC
	header := make([]byte, 6)
	if _, err := io.ReadFull(conn, header); err != nil {
		t.Fatalf("Failed to read A-ASSOCIATE-AC header: %v", err)
	}
	if header[0] != PDUTypeAssociateAC {
		t.Fatalf("Expected A-ASSOCIATE-AC (0x02), got 0x%02X", header[0])
	}
	acLen := binary.BigEndian.Uint32(header[2:6])
	acPayload := make([]byte, acLen)
	if _, err := io.ReadFull(conn, acPayload); err != nil {
		t.Fatalf("Failed to read A-ASSOCIATE-AC payload: %v", err)
	}

	// 2. Send C-ECHO-RQ (Verification Ping)
	echoCmd := buildTestCEchoRQ(1)
	if err := sendPDATA(conn, 1, true, echoCmd, nil); err != nil {
		t.Fatalf("Failed to send C-ECHO-RQ: %v", err)
	}

	// Read C-ECHO-RSP
	rspHeader := make([]byte, 6)
	if _, err := io.ReadFull(conn, rspHeader); err != nil {
		t.Fatalf("Failed to read C-ECHO-RSP header: %v", err)
	}
	if rspHeader[0] != PDUTypeDataTF {
		t.Fatalf("Expected P-DATA-TF (0x04) for C-ECHO-RSP, got 0x%02X", rspHeader[0])
	}
	rspLen := binary.BigEndian.Uint32(rspHeader[2:6])
	rspPayload := make([]byte, rspLen)
	if _, err := io.ReadFull(conn, rspPayload); err != nil {
		t.Fatalf("Failed to read C-ECHO-RSP payload: %v", err)
	}

	echoCmdMap := parseDicomElements(rspPayload[6:])
	cmdField := echoCmdMap.getUint16(0x0000, 0x0100)
	status := echoCmdMap.getUint16(0x0000, 0x0900)
	if cmdField != CommandCEchoRSP {
		t.Fatalf("Expected CommandCEchoRSP (0x8030), got 0x%04X", cmdField)
	}
	if status != StatusSuccess {
		t.Fatalf("Expected StatusSuccess (0x0000), got 0x%04X", status)
	}

	// 3. Send C-FIND-RQ for Modality Worklist (ctxID = 3)
	findCmd := buildTestCFindRQ(2)
	findQueryDataset := buildTestCFindQueryDataset("CT", time.Now().Format("20060102"))
	if err := sendPDATA(conn, 3, true, findCmd, findQueryDataset); err != nil {
		t.Fatalf("Failed to send C-FIND-RQ: %v", err)
	}

	// Read C-FIND-RSP stream (Pending items followed by StatusSuccess)
	expectedDesc := "CT Scan Kepala Polos Non Kontras"
	var foundTargetItem bool
	var totalMatches int

	for {
		rspHeader := make([]byte, 6)
		if _, err := io.ReadFull(conn, rspHeader); err != nil {
			t.Fatalf("Failed to read C-FIND-RSP header: %v", err)
		}
		rspLen := binary.BigEndian.Uint32(rspHeader[2:6])
		rspPayload := make([]byte, rspLen)
		if _, err := io.ReadFull(conn, rspPayload); err != nil {
			t.Fatalf("Failed to read C-FIND-RSP payload: %v", err)
		}

		// Check if it's the final response (Command with Status == 0x0000)
		if len(rspPayload) >= 6 {
			cmdMap := parseDicomElements(rspPayload[6:])
			status := cmdMap.getUint16(0x0000, 0x0900)
			if status == StatusSuccess {
				break
			}
		}

		totalMatches++
		payloadText := string(rspPayload)
		if strings.Contains(payloadText, expectedDesc) {
			count := strings.Count(payloadText, expectedDesc)
			if count >= 3 {
				foundTargetItem = true
			}
		}
	}

	if !foundTargetItem {
		t.Fatalf("Expected procedure description '%s' to appear at least 3 times (redundant copy to study_desc, req_proc_desc, series_desc) across %d matching worklist items", expectedDesc, totalMatches)
	}

	// 4. Send A-RELEASE-RQ
	releaseRQ := []byte{PDUTypeReleaseRQ, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00}
	if _, err := conn.Write(releaseRQ); err != nil {
		t.Fatalf("Failed to send A-RELEASE-RQ: %v", err)
	}

	relRspHeader := make([]byte, 10)
	if _, err := io.ReadFull(conn, relRspHeader); err != nil {
		t.Fatalf("Failed to read A-RELEASE-RP: %v", err)
	}
	if relRspHeader[0] != PDUTypeReleaseRP {
		t.Fatalf("Expected A-RELEASE-RP (0x06), got 0x%02X", relRspHeader[0])
	}

	// Verify operational stats
	stats := server.GetStats()
	if stats.EchoCount != 1 {
		t.Fatalf("Expected EchoCount = 1, got %d", stats.EchoCount)
	}
	if stats.FindCount != 1 {
		t.Fatalf("Expected FindCount = 1, got %d", stats.FindCount)
	}
	if stats.MatchesReturned < 1 {
		t.Fatalf("Expected MatchesReturned >= 1, got %d", stats.MatchesReturned)
	}
}

// Helpers for test SCU packet generation
func buildTestAssociateRQ(callingAET, calledAET string) []byte {
	var body bytes.Buffer
	binary.Write(&body, binary.BigEndian, uint16(1)) // Protocol version
	body.Write([]byte{0, 0})                        // Reserved
	body.WriteString(format16(calledAET))
	body.WriteString(format16(callingAET))
	body.Write(make([]byte, 32)) // Reserved

	// Application Context (0x10)
	writeItem(&body, 0x10, []byte(UIDApplicationContext))

	// Presentation Context 1: Verification (C-ECHO)
	var pc1 bytes.Buffer
	pc1.WriteByte(1) // Context ID 1
	pc1.Write([]byte{0, 0, 0})
	writeSubItem(&pc1, 0x30, []byte(UIDVerificationSOPClass))
	writeSubItem(&pc1, 0x40, []byte(UIDImplicitVRLittleEndian))
	writeItem(&body, 0x20, pc1.Bytes())

	// Presentation Context 3: MWL FIND (C-FIND)
	var pc2 bytes.Buffer
	pc2.WriteByte(3) // Context ID 3
	pc2.Write([]byte{0, 0, 0})
	writeSubItem(&pc2, 0x30, []byte(UIDModalityWorklistFIND))
	writeSubItem(&pc2, 0x40, []byte(UIDImplicitVRLittleEndian))
	writeItem(&body, 0x20, pc2.Bytes())

	// User Information (0x50)
	var user bytes.Buffer
	var maxPdu bytes.Buffer
	binary.Write(&maxPdu, binary.BigEndian, uint32(16384))
	writeSubItem(&user, 0x51, maxPdu.Bytes())
	writeItem(&body, 0x50, user.Bytes())

	var pdu bytes.Buffer
	pdu.WriteByte(PDUTypeAssociateRQ)
	pdu.WriteByte(0x00)
	binary.Write(&pdu, binary.BigEndian, uint32(body.Len()))
	pdu.Write(body.Bytes())
	return pdu.Bytes()
}

func buildTestCEchoRQ(msgID uint16) []byte {
	var buf bytes.Buffer
	writeImplicitElement(&buf, 0x0000, 0x0002, []byte(UIDVerificationSOPClass))
	writeImplicitUint16(&buf, 0x0000, 0x0100, CommandCEchoRQ)
	writeImplicitUint16(&buf, 0x0000, 0x0110, msgID)
	writeImplicitUint16(&buf, 0x0000, 0x0800, 0x0101) // No dataset

	fullLen := uint32(buf.Len())
	var res bytes.Buffer
	writeImplicitUint32(&res, 0x0000, 0x0000, fullLen)
	res.Write(buf.Bytes())
	return res.Bytes()
}

func buildTestCFindRQ(msgID uint16) []byte {
	var buf bytes.Buffer
	writeImplicitElement(&buf, 0x0000, 0x0002, []byte(UIDModalityWorklistFIND))
	writeImplicitUint16(&buf, 0x0000, 0x0100, CommandCFindRQ)
	writeImplicitUint16(&buf, 0x0000, 0x0110, msgID)
	writeImplicitUint16(&buf, 0x0000, 0x0800, 0x0000) // Dataset present

	fullLen := uint32(buf.Len())
	var res bytes.Buffer
	writeImplicitUint32(&res, 0x0000, 0x0000, fullLen)
	res.Write(buf.Bytes())
	return res.Bytes()
}

func buildTestCFindQueryDataset(modality, scheduledDate string) []byte {
	var buf bytes.Buffer
	writeImplicitElement(&buf, 0x0008, 0x0060, []byte(modality))
	writeImplicitElement(&buf, 0x0040, 0x0002, []byte(scheduledDate))
	return buf.Bytes()
}

func format16(s string) string {
	if len(s) > 16 {
		return s[:16]
	}
	return s + strings.Repeat(" ", 16-len(s))
}

func writeItem(buf *bytes.Buffer, itemType byte, data []byte) {
	buf.WriteByte(itemType)
	buf.WriteByte(0x00)
	binary.Write(buf, binary.BigEndian, uint16(len(data)))
	buf.Write(data)
}

func writeSubItem(buf *bytes.Buffer, itemType byte, data []byte) {
	buf.WriteByte(itemType)
	buf.WriteByte(0x00)
	binary.Write(buf, binary.BigEndian, uint16(len(data)))
	buf.Write(data)
}

func TestParseHL7ORMMessageFromMirth(t *testing.T) {
	// Exact payload from user's Mirth Connect screenshot (Message 67, ISC-TrakCare)
	hl7Payload := "MSH|^~\\&|ISC-TRAKCARE|ISC-TrakCare|APPLICATION|FACILITY|20251110110657||ORM^O01|6751914818426445136z161113|P|2.4|||AL|NE\r" +
		"PID|1||0030005502^^^^MR^^^||123^Testing Voucher^^^Ms^^L||20250929|M||||123455^ORN^CP^^^^^^P|||||||||||||||||||||||||||\r" +
		"PV1|1|E|41KUGD^^^41KUGD|||^^^41KUGD||||41KUGD||||||||||E410000002^^^^VN|||||||||||||||||||||||||||7410|||||20251103122200||||||\r" +
		"ORC|NW|562259/35||R007168^^^ISO|IP||1^^^20251110102400^^NORM||20251110102459|||EM00002169^LEONAL YUDHA PERMANA^^^^^^^^^^DN~446.4/0185/SIKP/06/11/DINKES/2015^^^^^^^^^^PRN||||\r" +
		"OBR|1|562259/35||RAD501_E^CR X-Ray Abdomen 2 Posisi^ISC-TrakCare||||||||||||EM00002169^LEONAL YUDHA PERMANA^^^^^^^^^^DN~446.4/0185/SIKP/06/11/DINKES/2015^^^^^^^^^^PRN|||||||||||\r" +
		"OBX|1|TX|EKA.QUES^Keterangan Klinis^TRAKCARE||test||||||F||||||COM"

	item, msh, err := ParseHL7ORMMessage(hl7Payload)
	if err != nil {
		t.Fatalf("Failed to parse HL7 ORM payload: %v", err)
	}

	if item.PatientID != "0030005502" {
		t.Errorf("Expected PatientID '0030005502', got '%s'", item.PatientID)
	}
	if !strings.Contains(item.PatientName, "Testing Voucher") {
		t.Errorf("Expected PatientName to contain 'Testing Voucher', got '%s'", item.PatientName)
	}
	if item.AccessionNumber != "562259/35" {
		t.Errorf("Expected AccessionNumber '562259/35', got '%s'", item.AccessionNumber)
	}
	if item.Modality != "CR" {
		t.Errorf("Expected Modality 'CR', got '%s'", item.Modality)
	}
	if item.ProcedureDesc != "CR X-Ray Abdomen 2 Posisi" {
		t.Errorf("Expected ProcedureDesc 'CR X-Ray Abdomen 2 Posisi', got '%s'", item.ProcedureDesc)
	}
	// Verify Redundant Procedure Description auto-copy
	if item.StudyDesc != item.ProcedureDesc {
		t.Errorf("Expected StudyDesc to equal ProcedureDesc, got '%s'", item.StudyDesc)
	}
	if item.ReqProcDesc != item.ProcedureDesc {
		t.Errorf("Expected ReqProcDesc to equal ProcedureDesc, got '%s'", item.ReqProcDesc)
	}
	if item.SeriesDesc != item.ProcedureDesc {
		t.Errorf("Expected SeriesDesc to equal ProcedureDesc, got '%s'", item.SeriesDesc)
	}
	if item.ScheduledDate != "20251110" {
		t.Errorf("Expected ScheduledDate '20251110', got '%s'", item.ScheduledDate)
	}
	if item.ScheduledTime != "102400" {
		t.Errorf("Expected ScheduledTime '102400', got '%s'", item.ScheduledTime)
	}

	// Verify HL7 ACK generation
	ack := BuildHL7ACK(*msh, "AA", "Order accepted")
	if !strings.Contains(ack, "MSA|AA|6751914818426445136z161113") {
		t.Errorf("Expected ACK to contain MSA|AA|6751914818426445136z161113, got:\n%s", ack)
	}
}

func TestMirthHL7IngestionToDicomServerPort(t *testing.T) {
	if err := initDb(); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	testPort := 11120
	server := &DicomServer{
		config: DicomServerConfig{
			Port:    testPort,
			AETitle: "HEALTH_CONNECT",
			Enabled: true,
		},
	}
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server on port %d: %v", testPort, err)
	}
	defer server.Stop()

	// Simulate Mirth Connect sending MLLP framed HL7 ORM to port 11120
	hl7Raw := "MSH|^~\\&|ISC-TRAKCARE|ISC-TrakCare|APPLICATION|FACILITY|20261031170000||ORM^O01|MSG-TEST-777|P|2.4\r" +
		"PID|1||0099887766^^^^MR^^^||Testing Mirth Ingestion||19900101|M\r" +
		"ORC|NW|ACC-MIRTH-777|||IP||1^^^20261031180000^^NORM\r" +
		"OBR|1|ACC-MIRTH-777||RAD101^CR Thorax PA^ISC-TrakCare"

	mllpFrame := BuildMLLPFrame(hl7Raw)

	conn, err := net.DialTimeout("tcp", "127.0.0.1:11120", 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to port 11120: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write(mllpFrame); err != nil {
		t.Fatalf("Failed to write MLLP frame: %v", err)
	}

	respBuf := make([]byte, 4096)
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(respBuf)
	if err != nil {
		t.Fatalf("Failed to read MLLP ACK from server: %v", err)
	}

	respStr := string(respBuf[:n])
	if !strings.Contains(respStr, "MSA|AA|MSG-TEST-777") {
		t.Fatalf("Expected MLLP response to contain 'MSA|AA|MSG-TEST-777', got: %s", respStr)
	}

	// Verify item was stored in database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM dicom_worklist_items WHERE accession_number = 'ACC-MIRTH-777'").Scan(&count)
	if err != nil || count == 0 {
		t.Fatalf("Worklist item was not saved in SQLite DB: %v", err)
	}
}

func TestRadiologyGatewayChannelDualJSONAndHL7(t *testing.T) {
	if err := initDb(); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}

	testEngine := NewIntegrationEngine()
	testEngine.LoadActiveRoutes()
	defer func() {
		for _, s := range testEngine.TCPServers {
			s.Close()
		}
	}()

	// 1. Ingest JSON order
	jsonOrder := `{"accession_number":"ACC-JSON-888","patient_id":"RM-JSON-01","patient_name":"Budi Santoso","modality":"CT","procedure_desc":"CT Scan Abdomen 3 Fase","physician_name":"dr. Ahmad Sp.Rad"}`
	success, resp, err := testEngine.ExecuteRoute("orm-pacs-gateway", jsonOrder)
	if !success || err != nil {
		t.Fatalf("Failed to execute route with JSON order: %v, resp: %s", err, resp)
	}

	// Verify JSON order saved in SQLite
	var jsonProcDesc string
	err = db.QueryRow("SELECT procedure_desc FROM dicom_worklist_items WHERE accession_number = 'ACC-JSON-888'").Scan(&jsonProcDesc)
	if err != nil || jsonProcDesc != "CT Scan Abdomen 3 Fase" {
		t.Fatalf("JSON order not found in DB: %v", err)
	}

	// 2. Ingest HL7 ORM order
	hl7Order := "MSH|^~\\&|SIMRS|HOSPITAL|HEALTH_CONNECT|PACS|20261031171500||ORM^O01|MSG-HL7-999|P|2.4\r" +
		"PID|1||RM-HL7-02^^^^MR||Siti Aminah||19950817|F\r" +
		"ORC|NW|ACC-HL7-999|||IP||1^^^20261031190000^^NORM\r" +
		"OBR|1|ACC-HL7-999||USG01^USG Abdomen Upper Lower^SIMRS"

	success2, resp2, err2 := testEngine.ExecuteRoute("orm-pacs-gateway", hl7Order)
	if !success2 || err2 != nil {
		t.Fatalf("Failed to execute route with HL7 order: %v, resp: %s", err2, resp2)
	}

	// Verify HL7 order saved in SQLite with auto-copied descriptions
	var hl7Modality, hl7StudyDesc string
	err = db.QueryRow("SELECT modality, study_desc FROM dicom_worklist_items WHERE accession_number = 'ACC-HL7-999'").Scan(&hl7Modality, &hl7StudyDesc)
	if err != nil || hl7Modality != "US" || hl7StudyDesc != "USG Abdomen Upper Lower" {
		t.Fatalf("HL7 order not properly saved: %v (modality=%s, study=%s)", err, hl7Modality, hl7StudyDesc)
	}
}
