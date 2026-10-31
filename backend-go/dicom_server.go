package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DICOM SOP Class UIDs
const (
	UIDVerificationSOPClass = "1.2.840.10008.1.1"
	UIDModalityWorklistFIND = "1.2.840.10008.5.1.4.31"

	UIDImplicitVRLittleEndian = "1.2.840.10008.1.2"
	UIDExplicitVRLittleEndian = "1.2.840.10008.1.2.1"
	UIDApplicationContext     = "1.2.840.10008.3.1.1.1"
	UIDImplementationClass    = "1.2.826.0.1.3680043.9.7890.1"
	ImplementationVersionName = "HC_DICOM_1_0"
)

// DICOM PDU Types
const (
	PDUTypeAssociateRQ = 0x01
	PDUTypeAssociateAC = 0x02
	PDUTypeAssociateRJ = 0x03
	PDUTypeDataTF      = 0x04
	PDUTypeReleaseRQ   = 0x05
	PDUTypeReleaseRP   = 0x06
	PDUTypeAbort       = 0x07
)

// DIMSE Command Codes
const (
	CommandCEchoRQ  uint16 = 0x0030
	CommandCEchoRSP uint16 = 0x8030
	CommandCFindRQ  uint16 = 0x0020
	CommandCFindRSP uint16 = 0x8020
)

// DIMSE Status Codes
const (
	StatusSuccess uint16 = 0x0000
	StatusPending uint16 = 0xFF00
)

// DicomServerConfig holds server configuration
type DicomServerConfig struct {
	Port     int    `json:"port"`
	AETitle  string `json:"ae_title"`
	Enabled  bool   `json:"enabled"`
}

// DicomServerStats holds operational metrics
type DicomServerStats struct {
	Running         bool     `json:"running"`
	Port            int      `json:"port"`
	AETitle         string   `json:"ae_title"`
	ServerIPs       []string `json:"server_ips"`
	StartedAt       string   `json:"started_at"`
	EchoCount       int64    `json:"echo_count"`
	FindCount       int64    `json:"find_count"`
	MatchesReturned int64    `json:"matches_returned"`
	LastClientAET   string   `json:"last_client_ae_title"`
	LastActivityAt  string   `json:"last_activity_at"`
	ErrorCount      int64    `json:"error_count"`
}

// DicomServer represents the native Go DICOM DIMSE SCP server
type DicomServer struct {
	mu             sync.Mutex
	config         DicomServerConfig
	listener       net.Listener
	running        bool
	startedAt      time.Time
	echoCount      int64
	findCount      int64
	matchesCount   int64
	errorCount     int64
	lastClientAET  string
	lastActivityAt string
	stopChan       chan struct{}
}

// Global DICOM Server instance
var GlobalDicomServer *DicomServer
var dicomServerOnce sync.Once

// InitDicomServer initializes and optionally starts the DICOM DIMSE server
func InitDicomServer() *DicomServer {
	dicomServerOnce.Do(func() {
		port := 11112
		if portEnv := os.Getenv("DICOM_PORT"); portEnv != "" {
			if p, err := strconv.Atoi(portEnv); err == nil && p > 0 {
				port = p
			}
		}
		aet := os.Getenv("DICOM_AE_TITLE")
		if aet == "" {
			aet = "HEALTH_CONNECT"
		}

		GlobalDicomServer = &DicomServer{
			config: DicomServerConfig{
				Port:    port,
				AETitle: strings.TrimSpace(aet),
				Enabled: true,
			},
		}

		// Auto start server in background
		go func() {
			time.Sleep(500 * time.Millisecond)
			if err := GlobalDicomServer.Start(); err != nil {
				log.Printf("[DICOM Server] Start notice: %v", err)
			}
		}()
	})
	return GlobalDicomServer
}

// Start launches the DICOM TCP listener
func (s *DicomServer) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}

	addr := fmt.Sprintf("0.0.0.0:%d", s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to bind DICOM port %d: %w", s.config.Port, err)
	}

	s.listener = listener
	s.running = true
	s.startedAt = time.Now()
	s.stopChan = make(chan struct{})
	s.mu.Unlock()

	log.Printf("[DICOM Server] Listening on %s (Called AE Title: %s)", addr, s.config.AETitle)

	go s.acceptLoop()
	return nil
}

// Stop stops the DICOM listener gracefully
func (s *DicomServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false
	close(s.stopChan)
	if s.listener != nil {
		err := s.listener.Close()
		s.listener = nil
		log.Printf("[DICOM Server] Stopped listening on port %d", s.config.Port)
		return err
	}
	return nil
}

// Restart reloads configuration and restarts the server
func (s *DicomServer) Restart(newPort int, newAETitle string) error {
	s.Stop()
	s.mu.Lock()
	if newPort > 0 {
		s.config.Port = newPort
	}
	if newAETitle != "" {
		s.config.AETitle = strings.TrimSpace(newAETitle)
	}
	s.mu.Unlock()
	return s.Start()
}

// GetStats returns current operational telemetry
func (s *DicomServer) GetStats() DicomServerStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	startedStr := ""
	if !s.startedAt.IsZero() {
		startedStr = s.startedAt.Format(time.RFC3339)
	}

	return DicomServerStats{
		Running:         s.running,
		Port:            s.config.Port,
		AETitle:         s.config.AETitle,
		ServerIPs:       GetLocalIPAddresses(),
		StartedAt:       startedStr,
		EchoCount:       atomic.LoadInt64(&s.echoCount),
		FindCount:       atomic.LoadInt64(&s.findCount),
		MatchesReturned: atomic.LoadInt64(&s.matchesCount),
		LastClientAET:   s.lastClientAET,
		LastActivityAt:  s.lastActivityAt,
		ErrorCount:      atomic.LoadInt64(&s.errorCount),
	}
}

func (s *DicomServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			s.mu.Lock()
			running := s.running
			s.mu.Unlock()
			if !running {
				return
			}
			log.Printf("[DICOM Server] Accept error: %v", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// DicomAssociation represents an active DICOM Upper Layer connection session
type DicomAssociation struct {
	conn            net.Conn
	callingAET      string
	calledAET       string
	activePresCtx   map[byte]presContext // contextID -> presContext
	maxPDUSize      uint32
}

type presContext struct {
	contextID      byte
	abstractSyntax string
	transferSyntax string
}

func (s *DicomServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(60 * time.Second))

	assoc := &DicomAssociation{
		conn:          conn,
		activePresCtx: make(map[byte]presContext),
		maxPDUSize:    16384,
	}

	for {
		header := make([]byte, 6)
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}

		// Intelligent Dual-Protocol Auto-Detection:
		// If inbound connection is HL7 v2 (MLLP \x0B or raw MSH), handle ORM ingestion
		if header[0] == 0x0B || (header[0] == 'M' && header[1] == 'S' && header[2] == 'H') {
			s.handleHL7Connection(conn, header)
			return
		}

		pduType := header[0]
		pduLength := binary.BigEndian.Uint32(header[2:6])

		// Validate that pduType is a legitimate DICOM PDU Type (0x01 to 0x07)
		if pduType < PDUTypeAssociateRQ || pduType > PDUTypeAbort {
			log.Printf("[DICOM Server] Non-DICOM packet received on port %d (Type: 0x%02X), closing session", s.config.Port, pduType)
			return
		}

		// Sanity check: DICOM Upper Layer max PDU length should not exceed 16MB
		if pduLength > 16*1024*1024 {
			log.Printf("[DICOM Server] PDU length excessive (%d bytes) for type 0x%02X, closing connection", pduLength, pduType)
			return
		}

		pduPayload := make([]byte, pduLength)
		if _, err := io.ReadFull(conn, pduPayload); err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				log.Printf("[DICOM Server] Incomplete PDU payload: %v", err)
			}
			return
		}

		_ = conn.SetDeadline(time.Now().Add(60 * time.Second))

		switch pduType {
		case PDUTypeAssociateRQ:
			if err := s.handleAssociateRQ(assoc, pduPayload); err != nil {
				log.Printf("[DICOM Server] Associate RQ error: %v", err)
				atomic.AddInt64(&s.errorCount, 1)
				return
			}
		case PDUTypeDataTF:
			if err := s.handleDataTF(assoc, pduPayload); err != nil {
				log.Printf("[DICOM Server] Data TF error: %v", err)
				atomic.AddInt64(&s.errorCount, 1)
				return
			}
		case PDUTypeReleaseRQ:
			s.sendReleaseRP(assoc)
			return
		case PDUTypeAbort:
			return
		default:
			log.Printf("[DICOM Server] Unknown PDU type: 0x%02X", pduType)
			return
		}
	}
}

func (s *DicomServer) handleHL7Connection(conn net.Conn, initialBytes []byte) {
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	var rawMsg bytes.Buffer
	isMLLP := initialBytes[0] == 0x0B
	if isMLLP {
		rawMsg.Write(initialBytes[1:])
	} else {
		rawMsg.Write(initialBytes)
	}

	buf := make([]byte, 4096)
	for {
		if isMLLP && bytes.Contains(rawMsg.Bytes(), []byte{0x1C}) {
			break
		}
		n, err := conn.Read(buf)
		if n > 0 {
			rawMsg.Write(buf[:n])
			if isMLLP && bytes.Contains(rawMsg.Bytes(), []byte{0x1C}) {
				break
			}
		}
		if err != nil {
			break
		}
	}

	hl7Text := rawMsg.String()
	item, msh, err := ParseHL7ORMMessage(hl7Text)
	if err != nil {
		log.Printf("[DICOM Server] HL7 Parse Error: %v", err)
		if msh == nil {
			msh = &HL7MSHInfo{}
		}
		nack := BuildHL7ACK(*msh, "AE", "Parse error: "+err.Error())
		if isMLLP {
			_, _ = conn.Write(BuildMLLPFrame(nack))
		} else {
			_, _ = conn.Write([]byte(nack))
		}
		atomic.AddInt64(&s.errorCount, 1)
		return
	}

	if err := SaveDicomWorklistItem(item); err != nil {
		log.Printf("[DICOM Server] HL7 Database Error: %v", err)
		nack := BuildHL7ACK(*msh, "AE", "Database error: "+err.Error())
		if isMLLP {
			_, _ = conn.Write(BuildMLLPFrame(nack))
		} else {
			_, _ = conn.Write([]byte(nack))
		}
		atomic.AddInt64(&s.errorCount, 1)
		return
	}

	s.mu.Lock()
	if msh.SendingApp != "" {
		s.lastClientAET = msh.SendingApp
	}
	s.lastActivityAt = time.Now().Format(time.RFC3339)
	s.mu.Unlock()

	atomic.AddInt64(&s.findCount, 1)
	log.Printf("[DICOM Server] Ingested HL7 ORM: Accession=%s, Patient=%s, Modality=%s, Procedure=%s", item.AccessionNumber, item.PatientName, item.Modality, item.ProcedureDesc)

	ack := BuildHL7ACK(*msh, "AA", "Order accepted and saved to DICOM Worklist")
	if isMLLP {
		_, _ = conn.Write(BuildMLLPFrame(ack))
	} else {
		_, _ = conn.Write([]byte(ack))
	}
}

func (s *DicomServer) handleAssociateRQ(assoc *DicomAssociation, payload []byte) error {
	if len(payload) < 68 {
		return fmt.Errorf("associate payload too short: %d", len(payload))
	}

	called := strings.TrimSpace(string(payload[10:26]))
	calling := strings.TrimSpace(string(payload[26:42]))
	assoc.calledAET = called
	assoc.callingAET = calling

	s.mu.Lock()
	s.lastClientAET = calling
	s.lastActivityAt = time.Now().Format(time.RFC3339)
	s.mu.Unlock()

	// Parse Variable Items starting at byte 68
	var acceptedContexts []presContext
	offset := 68
	for offset+4 <= len(payload) {
		itemType := payload[offset]
		itemLen := int(binary.BigEndian.Uint16(payload[offset+2 : offset+4]))
		itemEnd := offset + 4 + itemLen
		if itemEnd > len(payload) {
			break
		}

		itemData := payload[offset+4 : itemEnd]
		if itemType == 0x20 && len(itemData) >= 4 { // Presentation Context Item
			ctxID := itemData[0]
			var abstractSyntax string
			var transferSyntaxes []string

			subOffset := 4
			for subOffset+4 <= len(itemData) {
				subType := itemData[subOffset]
				subLen := int(binary.BigEndian.Uint16(itemData[subOffset+2 : subOffset+4]))
				subEnd := subOffset + 4 + subLen
				if subEnd > len(itemData) {
					break
				}
				subPayload := string(itemData[subOffset+4 : subEnd])
				if subType == 0x30 { // Abstract Syntax UID
					abstractSyntax = strings.TrimRight(subPayload, "\x00 ")
				} else if subType == 0x40 { // Transfer Syntax UID
					transferSyntaxes = append(transferSyntaxes, strings.TrimRight(subPayload, "\x00 "))
				}
				subOffset = subEnd
			}

			// Decide if we accept this SOP Class
			if abstractSyntax == UIDVerificationSOPClass || abstractSyntax == UIDModalityWorklistFIND {
				chosenTS := UIDImplicitVRLittleEndian
				for _, ts := range transferSyntaxes {
					if ts == UIDExplicitVRLittleEndian {
						chosenTS = UIDExplicitVRLittleEndian
						break
					}
				}
				pc := presContext{
					contextID:      ctxID,
					abstractSyntax: abstractSyntax,
					transferSyntax: chosenTS,
				}
				acceptedContexts = append(acceptedContexts, pc)
				assoc.activePresCtx[ctxID] = pc
			}
		} else if itemType == 0x50 { // User Information Item
			subOffset := 0
			for subOffset+4 <= len(itemData) {
				subType := itemData[subOffset]
				subLen := int(binary.BigEndian.Uint16(itemData[subOffset+2 : subOffset+4]))
				subEnd := subOffset + 4 + subLen
				if subEnd > len(itemData) {
					break
				}
				if subType == 0x51 && subLen == 4 { // Max PDU Length
					assoc.maxPDUSize = binary.BigEndian.Uint32(itemData[subOffset+4 : subEnd])
				}
				subOffset = subEnd
			}
		}
		offset = itemEnd
	}

	return s.sendAssociateAC(assoc, acceptedContexts)
}

func (s *DicomServer) sendAssociateAC(assoc *DicomAssociation, contexts []presContext) error {
	var body bytes.Buffer

	// Protocol version (uint16 = 1)
	binary.Write(&body, binary.BigEndian, uint16(1))
	// Reserved 2 bytes
	body.Write([]byte{0, 0})
	// Called AE Title (16 bytes, space padded)
	body.WriteString(fmt.Sprintf("%-16s", s.config.AETitle))
	// Calling AE Title (16 bytes, space padded)
	body.WriteString(fmt.Sprintf("%-16s", assoc.callingAET))
	// Reserved 32 bytes
	body.Write(make([]byte, 32))

	// Application Context Item (0x10)
	var appCtx bytes.Buffer
	appCtx.WriteString(UIDApplicationContext)
	body.WriteByte(0x10)
	body.WriteByte(0x00)
	binary.Write(&body, binary.BigEndian, uint16(appCtx.Len()))
	body.Write(appCtx.Bytes())

	// Presentation Context Items (0x21 - AC)
	for _, pc := range contexts {
		var pcBuf bytes.Buffer
		pcBuf.WriteByte(pc.contextID)
		pcBuf.WriteByte(0x00) // Reserved
		pcBuf.WriteByte(0x00) // Acceptance (0x00 = acceptance)
		pcBuf.WriteByte(0x00) // Reserved

		// Sub-item 0x40: Transfer Syntax
		var tsBuf bytes.Buffer
		tsBuf.WriteString(pc.transferSyntax)
		pcBuf.WriteByte(0x40)
		pcBuf.WriteByte(0x00)
		binary.Write(&pcBuf, binary.BigEndian, uint16(tsBuf.Len()))
		pcBuf.Write(tsBuf.Bytes())

		body.WriteByte(0x21)
		body.WriteByte(0x00)
		binary.Write(&body, binary.BigEndian, uint16(pcBuf.Len()))
		body.Write(pcBuf.Bytes())
	}

	// User Information Item (0x50)
	var userBuf bytes.Buffer
	// Sub-item 0x51: Max PDU Length (16384)
	userBuf.WriteByte(0x51)
	userBuf.WriteByte(0x00)
	binary.Write(&userBuf, binary.BigEndian, uint16(4))
	binary.Write(&userBuf, binary.BigEndian, uint32(16384))

	// Sub-item 0x52: Implementation Class UID
	var implClassBuf bytes.Buffer
	implClassBuf.WriteString(UIDImplementationClass)
	userBuf.WriteByte(0x52)
	userBuf.WriteByte(0x00)
	binary.Write(&userBuf, binary.BigEndian, uint16(implClassBuf.Len()))
	userBuf.Write(implClassBuf.Bytes())

	// Sub-item 0x55: Implementation Version Name
	var implVerBuf bytes.Buffer
	implVerBuf.WriteString(ImplementationVersionName)
	userBuf.WriteByte(0x55)
	userBuf.WriteByte(0x00)
	binary.Write(&userBuf, binary.BigEndian, uint16(implVerBuf.Len()))
	userBuf.Write(implVerBuf.Bytes())

	body.WriteByte(0x50)
	body.WriteByte(0x00)
	binary.Write(&body, binary.BigEndian, uint16(userBuf.Len()))
	body.Write(userBuf.Bytes())

	// Build full PDU
	var pdu bytes.Buffer
	pdu.WriteByte(PDUTypeAssociateAC)
	pdu.WriteByte(0x00)
	binary.Write(&pdu, binary.BigEndian, uint32(body.Len()))
	pdu.Write(body.Bytes())

	_, err := assoc.conn.Write(pdu.Bytes())
	return err
}

func (s *DicomServer) sendReleaseRP(assoc *DicomAssociation) {
	pdu := []byte{PDUTypeReleaseRP, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00}
	_, _ = assoc.conn.Write(pdu)
}

func (s *DicomServer) handleDataTF(assoc *DicomAssociation, payload []byte) error {
	offset := 0
	for offset+6 <= len(payload) {
		pdvLen := binary.BigEndian.Uint32(payload[offset : offset+4])
		ctxID := payload[offset+4]
		msgHeader := payload[offset+5] // Bit 0: 1=Command, 0=Data. Bit 1: 1=Last fragment

		pdvData := payload[offset+6 : offset+4+int(pdvLen)]
		offset += 4 + int(pdvLen)

		isCommand := (msgHeader & 0x01) != 0

		if isCommand {
			cmdMap := parseDicomElements(pdvData)
			cmdField := cmdMap.getUint16(0x0000, 0x0100)
			msgID := cmdMap.getUint16(0x0000, 0x0110)

			s.mu.Lock()
			s.lastActivityAt = time.Now().Format(time.RFC3339)
			s.mu.Unlock()

			if cmdField == CommandCEchoRQ {
				atomic.AddInt64(&s.echoCount, 1)
				return s.handleCEcho(assoc, ctxID, msgID)
			} else if cmdField == CommandCFindRQ {
				atomic.AddInt64(&s.findCount, 1)
				// The identifier query dataset will arrive in subsequent PDV or in remainder of stream
				// Parse any identifier in remainder of payload or read next PDV
				var queryData []byte
				if offset < len(payload) {
					// Dataset in same PDU
					nextPDVLen := binary.BigEndian.Uint32(payload[offset : offset+4])
					queryData = payload[offset+6 : offset+4+int(nextPDVLen)]
				}
				return s.handleCFind(assoc, ctxID, msgID, queryData)
			}
		}
	}
	return nil
}

func (s *DicomServer) handleCEcho(assoc *DicomAssociation, ctxID byte, msgID uint16) error {
	cmdSet := buildCEchoResponse(msgID, StatusSuccess)
	return sendPDATA(assoc.conn, ctxID, true, cmdSet, nil)
}

func (s *DicomServer) handleCFind(assoc *DicomAssociation, ctxID byte, msgID uint16, queryData []byte) error {
	queryElements := parseDicomElements(queryData)

	// Extract query keys
	patientID := queryElements.getString(0x0010, 0x0020)
	patientName := queryElements.getString(0x0010, 0x0010)
	accessionNum := queryElements.getString(0x0008, 0x0050)
	
	// Modality & Scheduled Date from sequence or top level
	modality := queryElements.getString(0x0008, 0x0060)
	scheduledDate := queryElements.getString(0x0040, 0x0002)
	stationAET := queryElements.getString(0x0040, 0x0001)

	// Query matching items from internal SQLite / PostgreSQL database
	items, err := FindMatchingWorklistItems(scheduledDate, modality, patientID, accessionNum, patientName, stationAET)
	if err != nil {
		log.Printf("[DICOM MWL] Database query error: %v", err)
	}

	atomic.AddInt64(&s.matchesCount, int64(len(items)))

	// Send C-FIND-RSP with Status = 0xFF00 (Pending) for each matching item
	for _, item := range items {
		// Ensure procedure_desc is redundantly copied to study_desc, req_proc_desc, and series_desc
		item.ApplyProcedureDescriptionCopy()

		respCmd := buildCFindResponse(msgID, StatusPending, true)
		respDataset := encodeDicomWorklistDataset(item)

		if err := sendPDATA(assoc.conn, ctxID, true, respCmd, respDataset); err != nil {
			return err
		}
	}

	// Send final C-FIND-RSP with Status = 0x0000 (Success) and no dataset to mark completion
	finalCmd := buildCFindResponse(msgID, StatusSuccess, false)
	return sendPDATA(assoc.conn, ctxID, true, finalCmd, nil)
}

func sendPDATA(conn net.Conn, ctxID byte, isCommand bool, cmdData []byte, datasetData []byte) error {
	var body bytes.Buffer

	if len(cmdData) > 0 {
		pdvLen := uint32(len(cmdData) + 2)
		binary.Write(&body, binary.BigEndian, pdvLen)
		body.WriteByte(ctxID)
		body.WriteByte(0x03) // 0x01 (Command) | 0x02 (Last fragment)
		body.Write(cmdData)
	}

	if len(datasetData) > 0 {
		pdvLen := uint32(len(datasetData) + 2)
		binary.Write(&body, binary.BigEndian, pdvLen)
		body.WriteByte(ctxID)
		body.WriteByte(0x02) // 0x00 (Data) | 0x02 (Last fragment)
		body.Write(datasetData)
	}

	var pdu bytes.Buffer
	pdu.WriteByte(PDUTypeDataTF)
	pdu.WriteByte(0x00)
	binary.Write(&pdu, binary.BigEndian, uint32(body.Len()))
	pdu.Write(body.Bytes())

	_, err := conn.Write(pdu.Bytes())
	return err
}

func buildCEchoResponse(msgID uint16, status uint16) []byte {
	var buf bytes.Buffer
	// Group 0x0000 elements in Implicit VR Little Endian
	// (0000,0002) Affected SOP Class UID: Verification SOP Class
	writeImplicitElement(&buf, 0x0000, 0x0002, []byte(UIDVerificationSOPClass))
	// (0000,0100) Command Field: C-ECHO-RSP (0x8030)
	writeImplicitUint16(&buf, 0x0000, 0x0100, CommandCEchoRSP)
	// (0000,0120) Message ID Being Responded To
	writeImplicitUint16(&buf, 0x0000, 0x0120, msgID)
	// (0000,0800) Data Set Present (0x0101 = No Data Set)
	writeImplicitUint16(&buf, 0x0000, 0x0800, 0x0101)
	// (0000,0900) Status
	writeImplicitUint16(&buf, 0x0000, 0x0900, status)

	// Prepend (0000,0000) Command Group Length
	fullLen := uint32(buf.Len())
	var res bytes.Buffer
	writeImplicitUint32(&res, 0x0000, 0x0000, fullLen)
	res.Write(buf.Bytes())
	return res.Bytes()
}

func buildCFindResponse(msgID uint16, status uint16, hasDataset bool) []byte {
	var buf bytes.Buffer
	// (0000,0002) Affected SOP Class UID: Modality Worklist FIND
	writeImplicitElement(&buf, 0x0000, 0x0002, []byte(UIDModalityWorklistFIND))
	// (0000,0100) Command Field: C-FIND-RSP (0x8020)
	writeImplicitUint16(&buf, 0x0000, 0x0100, CommandCFindRSP)
	// (0000,0120) Message ID Being Responded To
	writeImplicitUint16(&buf, 0x0000, 0x0120, msgID)
	// (0000,0800) Data Set Type (0x0000 = Data set present, 0x0101 = None)
	if hasDataset {
		writeImplicitUint16(&buf, 0x0000, 0x0800, 0x0000)
	} else {
		writeImplicitUint16(&buf, 0x0000, 0x0800, 0x0101)
	}
	// (0000,0900) Status
	writeImplicitUint16(&buf, 0x0000, 0x0900, status)

	fullLen := uint32(buf.Len())
	var res bytes.Buffer
	writeImplicitUint32(&res, 0x0000, 0x0000, fullLen)
	res.Write(buf.Bytes())
	return res.Bytes()
}

// encodeDicomWorklistDataset serializes a DicomWorklistItem into standard DICOM MWL dataset
// strictly fulfilling the auto-copy procedure_desc requirement across all 4 description tags.
func encodeDicomWorklistDataset(item DicomWorklistItem) []byte {
	var buf bytes.Buffer

	// Specific Character Set (ISO_IR 100 for Latin-1 / UTF-8 fallback)
	writeImplicitElement(&buf, 0x0008, 0x0005, []byte("ISO_IR 100"))

	// Accession Number (0008,0050)
	writeImplicitElement(&buf, 0x0008, 0x0050, []byte(item.AccessionNumber))

	// Referring Physician's Name (0008,0090)
	writeImplicitElement(&buf, 0x0008, 0x0090, []byte(item.PhysicianName))

	// Study Description (0008,1030) -> REDUNDANT COPY from procedure_desc
	writeImplicitElement(&buf, 0x0008, 0x1030, []byte(item.StudyDesc))

	// Patient's Name (0010,0010)
	writeImplicitElement(&buf, 0x0010, 0x0010, []byte(item.PatientName))

	// Patient ID (0010,0020)
	writeImplicitElement(&buf, 0x0010, 0x0020, []byte(item.PatientID))

	// Patient's Birth Date (0010,0030)
	writeImplicitElement(&buf, 0x0010, 0x0030, []byte(item.BirthDate))

	// Patient's Sex (0010,0040)
	writeImplicitElement(&buf, 0x0010, 0x0040, []byte(item.Gender))

	// Study Instance UID (0020,000D)
	writeImplicitElement(&buf, 0x0020, 0x000D, []byte(item.StudyUID))

	// Requesting Physician (0032,1032)
	writeImplicitElement(&buf, 0x0032, 0x1032, []byte(item.PhysicianName))

	// Requested Procedure Description (0032,1060) -> REDUNDANT COPY from procedure_desc
	writeImplicitElement(&buf, 0x0032, 0x1060, []byte(item.ReqProcDesc))

	// Requested Procedure ID (0040,1001)
	writeImplicitElement(&buf, 0x0040, 0x1001, []byte(item.AccessionNumber))

	// Scheduled Procedure Step Sequence (0040,0100)
	var spsBuf bytes.Buffer
	// (0008,0060) Modality
	writeImplicitElement(&spsBuf, 0x0008, 0x0060, []byte(item.Modality))
	// (0040,0001) Scheduled Station AE Title
	writeImplicitElement(&spsBuf, 0x0040, 0x0001, []byte(item.StationAETitle))
	// (0040,0002) Scheduled Procedure Step Start Date
	writeImplicitElement(&spsBuf, 0x0040, 0x0002, []byte(item.ScheduledDate))
	// (0040,0003) Scheduled Procedure Step Start Time
	writeImplicitElement(&spsBuf, 0x0040, 0x0003, []byte(item.ScheduledTime))
	// (0040,0006) Scheduled Performing Physician's Name
	writeImplicitElement(&spsBuf, 0x0040, 0x0006, []byte(item.PhysicianName))
	// (0040,0007) Scheduled Procedure Step Description -> Source procedure_desc
	writeImplicitElement(&spsBuf, 0x0040, 0x0007, []byte(item.ProcedureDesc))
	// (0008,103E) Series Description -> REDUNDANT COPY from procedure_desc
	writeImplicitElement(&spsBuf, 0x0008, 0x103E, []byte(item.SeriesDesc))
	// (0040,0009) Scheduled Procedure Step ID
	writeImplicitElement(&spsBuf, 0x0040, 0x0009, []byte("SPS_"+item.AccessionNumber))

	// Wrap in Sequence Item (FFFE,E000)
	var seqItem bytes.Buffer
	binary.Write(&seqItem, binary.LittleEndian, uint16(0xFFFE))
	binary.Write(&seqItem, binary.LittleEndian, uint16(0xE000))
	binary.Write(&seqItem, binary.LittleEndian, uint32(spsBuf.Len()))
	seqItem.Write(spsBuf.Bytes())

	// Write (0040,0100) Sequence Header
	binary.Write(&buf, binary.LittleEndian, uint16(0x0040))
	binary.Write(&buf, binary.LittleEndian, uint16(0x0100))
	binary.Write(&buf, binary.LittleEndian, uint32(seqItem.Len()))
	buf.Write(seqItem.Bytes())

	return buf.Bytes()
}

// Low-level DICOM Implicit VR Little Endian writers
func writeImplicitElement(buf *bytes.Buffer, group uint16, elem uint16, val []byte) {
	binary.Write(buf, binary.LittleEndian, group)
	binary.Write(buf, binary.LittleEndian, elem)

	// Ensure even length padding with space (0x20)
	length := len(val)
	if length%2 != 0 {
		val = append(val, ' ')
		length++
	}

	binary.Write(buf, binary.LittleEndian, uint32(length))
	buf.Write(val)
}

func writeImplicitUint16(buf *bytes.Buffer, group uint16, elem uint16, val uint16) {
	binary.Write(buf, binary.LittleEndian, group)
	binary.Write(buf, binary.LittleEndian, elem)
	binary.Write(buf, binary.LittleEndian, uint32(2))
	binary.Write(buf, binary.LittleEndian, val)
}

func writeImplicitUint32(buf *bytes.Buffer, group uint16, elem uint16, val uint32) {
	binary.Write(buf, binary.LittleEndian, group)
	binary.Write(buf, binary.LittleEndian, elem)
	binary.Write(buf, binary.LittleEndian, uint32(4))
	binary.Write(buf, binary.LittleEndian, val)
}

// DICOM element map parser
type dicomTagKey uint32

func makeTagKey(group, elem uint16) dicomTagKey {
	return dicomTagKey((uint32(group) << 16) | uint32(elem))
}

type dicomElementMap map[dicomTagKey][]byte

func (m dicomElementMap) getString(group, elem uint16) string {
	if val, ok := m[makeTagKey(group, elem)]; ok {
		return strings.TrimRight(string(val), "\x00 ")
	}
	return ""
}

func (m dicomElementMap) getUint16(group, elem uint16) uint16 {
	if val, ok := m[makeTagKey(group, elem)]; ok && len(val) >= 2 {
		return binary.LittleEndian.Uint16(val)
	}
	return 0
}

func parseDicomElements(data []byte) dicomElementMap {
	elements := make(dicomElementMap)
	offset := 0

	for offset+8 <= len(data) {
		group := binary.LittleEndian.Uint16(data[offset : offset+2])
		elem := binary.LittleEndian.Uint16(data[offset+2 : offset+4])
		length := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		offset += 8

		if length == 0xFFFFFFFF {
			// Undefined length sequence / item, stop parsing basic elements
			break
		}

		end := offset + int(length)
		if end > len(data) {
			break
		}

		val := data[offset:end]
		elements[makeTagKey(group, elem)] = val
		offset = end
	}

	return elements
}

// PingDicomServer performs a real DICOM C-ECHO verification handshake against host:port
func PingDicomServer(host string, port int, callingAET, calledAET string) (time.Duration, error) {
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if port <= 0 {
		port = 11112
	}
	if callingAET == "" {
		callingAET = "HC_PING_CLIENT"
	}
	if calledAET == "" {
		calledAET = "HEALTH_CONNECT"
	}

	start := time.Now()
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return 0, fmt.Errorf("Koneksi TCP ke %s gagal: %w", addr, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

	// 1. Send A-ASSOCIATE-RQ
	assocRQ := buildPingAssociateRQ(callingAET, calledAET)
	if _, err := conn.Write(assocRQ); err != nil {
		return 0, fmt.Errorf("Gagal kirim A-ASSOCIATE-RQ: %w", err)
	}

	// 2. Read A-ASSOCIATE-AC
	header := make([]byte, 6)
	if _, err := io.ReadFull(conn, header); err != nil {
		return 0, fmt.Errorf("Gagal baca respon A-ASSOCIATE-AC: %w", err)
	}
	if header[0] != PDUTypeAssociateAC {
		return 0, fmt.Errorf("Asosiasi ditolak server DICOM (PDU 0x%02X)", header[0])
	}
	acLen := binary.BigEndian.Uint32(header[2:6])
	acPayload := make([]byte, acLen)
	if _, err := io.ReadFull(conn, acPayload); err != nil {
		return 0, fmt.Errorf("Payload A-ASSOCIATE-AC tidak lengkap: %w", err)
	}

	// 3. Send C-ECHO-RQ (Verification Ping)
	echoCmd := buildPingCEchoRQ(1)
	if err := sendPDATA(conn, 1, true, echoCmd, nil); err != nil {
		return 0, fmt.Errorf("Gagal kirim C-ECHO-RQ: %w", err)
	}

	// 4. Read C-ECHO-RSP
	rspHeader := make([]byte, 6)
	if _, err := io.ReadFull(conn, rspHeader); err != nil {
		return 0, fmt.Errorf("Gagal baca header C-ECHO-RSP: %w", err)
	}
	if rspHeader[0] != PDUTypeDataTF {
		return 0, fmt.Errorf("Respon bukan P-DATA-TF (0x%02X)", rspHeader[0])
	}
	rspLen := binary.BigEndian.Uint32(rspHeader[2:6])
	rspPayload := make([]byte, rspLen)
	if _, err := io.ReadFull(conn, rspPayload); err != nil {
		return 0, fmt.Errorf("Payload C-ECHO-RSP tidak lengkap: %w", err)
	}

	if len(rspPayload) < 6 {
		return 0, fmt.Errorf("Panjang respon C-ECHO terlalu pendek")
	}
	cmdMap := parseDicomElements(rspPayload[6:])
	cmdField := cmdMap.getUint16(0x0000, 0x0100)
	status := cmdMap.getUint16(0x0000, 0x0900)

	if cmdField != CommandCEchoRSP {
		return 0, fmt.Errorf("Respon bukan C-ECHO-RSP (0x%04X)", cmdField)
	}
	if status != StatusSuccess {
		return 0, fmt.Errorf("C-ECHO mengembalikan status gagal: 0x%04X", status)
	}

	// 5. Release association gracefully
	releaseRQ := []byte{PDUTypeReleaseRQ, 0x00, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00}
	_, _ = conn.Write(releaseRQ)

	return time.Since(start), nil
}

func buildPingAssociateRQ(callingAET, calledAET string) []byte {
	var body bytes.Buffer
	binary.Write(&body, binary.BigEndian, uint16(1))
	body.Write([]byte{0, 0})
	body.WriteString(fmt.Sprintf("%-16s", calledAET))
	body.WriteString(fmt.Sprintf("%-16s", callingAET))
	body.Write(make([]byte, 32))

	// Application Context (0x10)
	var appBuf bytes.Buffer
	appBuf.WriteString(UIDApplicationContext)
	body.WriteByte(0x10)
	body.WriteByte(0x00)
	binary.Write(&body, binary.BigEndian, uint16(appBuf.Len()))
	body.Write(appBuf.Bytes())

	// Presentation Context 1: Verification (C-ECHO)
	var pc1 bytes.Buffer
	pc1.WriteByte(1) // Context ID 1
	pc1.Write([]byte{0, 0, 0})
	// Abstract Syntax (0x30)
	var asBuf bytes.Buffer
	asBuf.WriteString(UIDVerificationSOPClass)
	pc1.WriteByte(0x30)
	pc1.WriteByte(0x00)
	binary.Write(&pc1, binary.BigEndian, uint16(asBuf.Len()))
	pc1.Write(asBuf.Bytes())
	// Transfer Syntax (0x40)
	var tsBuf bytes.Buffer
	tsBuf.WriteString(UIDImplicitVRLittleEndian)
	pc1.WriteByte(0x40)
	pc1.WriteByte(0x00)
	binary.Write(&pc1, binary.BigEndian, uint16(tsBuf.Len()))
	pc1.Write(tsBuf.Bytes())

	body.WriteByte(0x20)
	body.WriteByte(0x00)
	binary.Write(&body, binary.BigEndian, uint16(pc1.Len()))
	body.Write(pc1.Bytes())

	// User Information (0x50)
	var user bytes.Buffer
	user.WriteByte(0x51)
	user.WriteByte(0x00)
	binary.Write(&user, binary.BigEndian, uint16(4))
	binary.Write(&user, binary.BigEndian, uint32(16384))

	body.WriteByte(0x50)
	body.WriteByte(0x00)
	binary.Write(&body, binary.BigEndian, uint16(user.Len()))
	body.Write(user.Bytes())

	var pdu bytes.Buffer
	pdu.WriteByte(PDUTypeAssociateRQ)
	pdu.WriteByte(0x00)
	binary.Write(&pdu, binary.BigEndian, uint32(body.Len()))
	pdu.Write(body.Bytes())
	return pdu.Bytes()
}

func buildPingCEchoRQ(msgID uint16) []byte {
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

// GetLocalIPAddresses detects local IPv4 addresses assigned to this machine
func GetLocalIPAddresses() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				if ip4 := ipNet.IP.To4(); ip4 != nil {
					ips = append(ips, ip4.String())
				}
			}
		}
	}
	if len(ips) == 0 {
		ips = append(ips, "127.0.0.1")
	}
	return ips
}

