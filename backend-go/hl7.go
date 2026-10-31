package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// HL7MSHInfo stores essential MSH header metadata needed for ACK replies
type HL7MSHInfo struct {
	FieldSeparator  string `json:"field_separator"`
	ComponentSep    string `json:"component_separator"`
	SendingApp      string `json:"sending_app"`
	SendingFacility string `json:"sending_facility"`
	ReceivingApp    string `json:"receiving_app"`
	ReceivingFac    string `json:"receiving_facility"`
	MsgDateTime     string `json:"msg_date_time"`
	MsgType         string `json:"msg_type"`
	MsgEvent        string `json:"msg_event"`
	ControlID       string `json:"control_id"`
	ProcessingID    string `json:"processing_id"`
	VersionID       string `json:"version_id"`
}

// HL7Message represents a parsed HL7 v2 message
type HL7Message struct {
	Raw        string
	Segments   []HL7Segment
	MSH        HL7MSHInfo
	FieldSep   string
	CompSep    string
	RepSep     string
	EscapeChar string
	SubCompSep string
}

// HL7Segment represents a single segment with its fields
type HL7Segment struct {
	Name   string
	Fields []string
}

// ParseHL7 parses an HL7 v2 pipe-delimited message string
func ParseHL7(raw string) *HL7Message {
	trimmed := strings.TrimSpace(raw)
	// Strip MLLP wrapping characters if present
	trimmed = strings.Trim(trimmed, "\x0B\x1C\x0D\n")

	msg := &HL7Message{
		Raw:        trimmed,
		FieldSep:   "|",
		CompSep:    "^",
		RepSep:     "~",
		EscapeChar: "\\",
		SubCompSep: "&",
	}

	if len(trimmed) < 4 || !strings.HasPrefix(trimmed, "MSH") {
		return msg
	}

	msg.FieldSep = string(trimmed[3])
	if len(trimmed) > 4 {
		endEnc := strings.Index(trimmed[4:], msg.FieldSep)
		if endEnc != -1 {
			encChars := trimmed[4 : 4+endEnc]
			if len(encChars) >= 1 {
				msg.CompSep = string(encChars[0])
			}
			if len(encChars) >= 2 {
				msg.RepSep = string(encChars[1])
			}
			if len(encChars) >= 3 {
				msg.EscapeChar = string(encChars[2])
			}
			if len(encChars) >= 4 {
				msg.SubCompSep = string(encChars[3])
			}
		}
	}

	// Split by carriage return or newline
	normalized := strings.ReplaceAll(trimmed, "\r\n", "\r")
	normalized = strings.ReplaceAll(normalized, "\n", "\r")
	lines := strings.Split(normalized, "\r")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, msg.FieldSep)
		segName := fields[0]

		// For MSH, fields[1] is the encoding characters, so adjust standard 1-based indexing
		if segName == "MSH" {
			// Insert fieldSep at index 1 to match 1-based HL7 MSH index specification
			mshFields := make([]string, len(fields)+1)
			mshFields[0] = "MSH"
			mshFields[1] = msg.FieldSep
			copy(mshFields[2:], fields[1:])
			fields = mshFields

			msg.MSH.FieldSeparator = msg.FieldSep
			msg.MSH.ComponentSep = msg.CompSep
			if len(fields) > 3 {
				msg.MSH.SendingApp = fields[3]
			}
			if len(fields) > 4 {
				msg.MSH.SendingFacility = fields[4]
			}
			if len(fields) > 5 {
				msg.MSH.ReceivingApp = fields[5]
			}
			if len(fields) > 6 {
				msg.MSH.ReceivingFac = fields[6]
			}
			if len(fields) > 7 {
				msg.MSH.MsgDateTime = fields[7]
			}
			if len(fields) > 9 {
				typeParts := strings.Split(fields[9], msg.CompSep)
				if len(typeParts) > 0 {
					msg.MSH.MsgType = typeParts[0]
				}
				if len(typeParts) > 1 {
					msg.MSH.MsgEvent = typeParts[1]
				}
			}
			if len(fields) > 10 {
				msg.MSH.ControlID = fields[10]
			}
			if len(fields) > 11 {
				msg.MSH.ProcessingID = fields[11]
			}
			if len(fields) > 12 {
				msg.MSH.VersionID = fields[12]
			}
		}

		msg.Segments = append(msg.Segments, HL7Segment{
			Name:   segName,
			Fields: fields,
		})
	}

	return msg
}

// GetSegment returns the first segment matching the given name
func (m *HL7Message) GetSegment(name string) *HL7Segment {
	for i := range m.Segments {
		if m.Segments[i].Name == name {
			return &m.Segments[i]
		}
	}
	return nil
}

// GetSegments returns all segments matching the given name
func (m *HL7Message) GetSegments(name string) []*HL7Segment {
	var list []*HL7Segment
	for i := range m.Segments {
		if m.Segments[i].Name == name {
			list = append(list, &m.Segments[i])
		}
	}
	return list
}

// GetField returns a field by 1-based index from a segment
func (s *HL7Segment) GetField(idx int) string {
	if s == nil || idx <= 0 || idx >= len(s.Fields) {
		return ""
	}
	return s.Fields[idx]
}

// GetComponent returns a sub-component by 1-based index
func (s *HL7Segment) GetComponent(fieldIdx int, compIdx int, compSep string) string {
	f := s.GetField(fieldIdx)
	if f == "" || compIdx <= 0 {
		return ""
	}
	parts := strings.Split(f, compSep)
	if compIdx > len(parts) {
		return ""
	}
	return parts[compIdx-1]
}

// IsHL7Payload checks if a string appears to be an HL7 v2 message
func IsHL7Payload(payload string) bool {
	trimmed := strings.TrimSpace(payload)
	trimmed = strings.Trim(trimmed, "\x0B\x1C\x0D\n")
	return strings.HasPrefix(trimmed, "MSH|") || strings.HasPrefix(trimmed, "MSH^")
}

// ParseHL7ORMMessage parses an HL7 ORM^O01 (or OMG^O19) message into a DicomWorklistItem
func ParseHL7ORMMessage(raw string) (*DicomWorklistItem, *HL7MSHInfo, error) {
	msg := ParseHL7(raw)
	if len(msg.Segments) == 0 || msg.GetSegment("MSH") == nil {
		return nil, nil, fmt.Errorf("invalid HL7: missing MSH segment")
	}

	mshSeg := msg.GetSegment("MSH")
	pidSeg := msg.GetSegment("PID")
	orcSeg := msg.GetSegment("ORC")
	obrSeg := msg.GetSegment("OBR")

	item := &DicomWorklistItem{
		RawPayload: raw,
		Status:     "SCHEDULED",
	}

	// 1. Patient Details from PID
	if pidSeg != nil {
		// PID-3: Patient ID / MRN (0030005502^^^^MR^^^)
		patIDRaw := pidSeg.GetField(3)
		if patIDRaw != "" {
			parts := strings.Split(patIDRaw, msg.CompSep)
			item.PatientID = strings.TrimSpace(parts[0])
		}

		// PID-5: Patient Name (123^Testing Voucher^^^Ms^^L)
		patNameRaw := pidSeg.GetField(5)
		if patNameRaw != "" {
			nameParts := strings.Split(patNameRaw, msg.CompSep)
			// HL7 format: Family^Given^Middle^Suffix^Prefix
			family := ""
			given := ""
			if len(nameParts) > 0 {
				family = strings.TrimSpace(nameParts[0])
			}
			if len(nameParts) > 1 {
				given = strings.TrimSpace(nameParts[1])
			}

			if given != "" && family != "" {
				// If family is numeric prefix (like MRN or counter), prioritize given name
				if isNumeric(family) {
					item.PatientName = given
				} else {
					item.PatientName = fmt.Sprintf("%s^%s", family, given)
				}
			} else if given != "" {
				item.PatientName = given
			} else if family != "" {
				item.PatientName = family
			} else {
				item.PatientName = patNameRaw
			}
		}

		// PID-7: Birth Date (YYYYMMDD or YYYYMMDDHHMMSS)
		birthRaw := pidSeg.GetField(7)
		if len(birthRaw) >= 8 {
			item.BirthDate = birthRaw[:8]
		}

		// PID-8: Gender (M, F, O)
		genderRaw := strings.ToUpper(strings.TrimSpace(pidSeg.GetField(8)))
		if strings.HasPrefix(genderRaw, "M") || strings.HasPrefix(genderRaw, "L") {
			item.Gender = "M"
		} else if strings.HasPrefix(genderRaw, "F") || strings.HasPrefix(genderRaw, "P") || strings.HasPrefix(genderRaw, "W") {
			item.Gender = "F"
		} else {
			item.Gender = "O"
		}
	}

	// 2. Order & Accession Details from ORC & OBR
	accession := ""
	scheduledDate := ""
	scheduledTime := ""
	physician := ""

	if orcSeg != nil {
		// ORC-1: Order Control (NW=New, CA=Cancel, DC=Discontinue, XO=Change)
		orderCtrl := strings.ToUpper(strings.TrimSpace(orcSeg.GetField(1)))
		if orderCtrl == "CA" || orderCtrl == "DC" || orderCtrl == "OC" {
			item.Status = "CANCELLED"
		}

		// ORC-2: Placer Order Number (562259/35)
		orcPlacer := orcSeg.GetField(2)
		if orcPlacer != "" {
			parts := strings.Split(orcPlacer, msg.CompSep)
			accession = strings.TrimSpace(parts[0])
		}

		// ORC-7: Quantity/Timing (1^^^20251110102400^^NORM)
		timing := orcSeg.GetField(7)
		if timing != "" {
			tParts := strings.Split(timing, msg.CompSep)
			// Subfield 4 (index 3) is scheduled date/time in YYYYMMDDHHMMSS format
			for _, part := range tParts {
				cleanPart := strings.TrimSpace(part)
				if len(cleanPart) >= 8 && isNumeric(cleanPart[:8]) {
					scheduledDate = cleanPart[:8]
					if len(cleanPart) >= 14 {
						scheduledTime = cleanPart[8:14]
					}
					break
				}
			}
		}

		// ORC-12: Ordering Provider (EM00002169^LEONAL YUDHA PERMANA^^^^^^^^^^DN...)
		provider := orcSeg.GetField(12)
		if provider != "" {
			pParts := strings.Split(provider, msg.CompSep)
			if len(pParts) > 1 && strings.TrimSpace(pParts[1]) != "" {
				physician = strings.TrimSpace(pParts[1])
			} else if len(pParts) > 0 {
				physician = strings.TrimSpace(pParts[0])
			}
		}
	}

	if obrSeg != nil {
		// OBR-2: Placer Order Number (fallback for accession)
		if accession == "" {
			obrPlacer := obrSeg.GetField(2)
			if obrPlacer != "" {
				parts := strings.Split(obrPlacer, msg.CompSep)
				accession = strings.TrimSpace(parts[0])
			}
		}

		// OBR-4: Universal Service Identifier (RAD501_E^CR X-Ray Abdomen 2 Posisi^ISC-TrakCare)
		serviceID := obrSeg.GetField(4)
		if serviceID != "" {
			sParts := strings.Split(serviceID, msg.CompSep)
			code := ""
			desc := ""
			if len(sParts) > 0 {
				code = strings.TrimSpace(sParts[0])
			}
			if len(sParts) > 1 {
				desc = strings.TrimSpace(sParts[1])
			}
			if desc == "" {
				desc = code
			}

			item.ProcedureDesc = desc

			// Infer Modality from procedure description or service code
			item.Modality = detectModality(desc, code)
		}

		// OBR-7: Observation Date/Time (fallback)
		if scheduledDate == "" {
			obrDate := obrSeg.GetField(7)
			if len(obrDate) >= 8 && isNumeric(obrDate[:8]) {
				scheduledDate = obrDate[:8]
				if len(obrDate) >= 14 {
					scheduledTime = obrDate[8:14]
				}
			}
		}

		// OBR-16: Ordering Provider (fallback)
		if physician == "" {
			obrDoc := obrSeg.GetField(16)
			if obrDoc != "" {
				dParts := strings.Split(obrDoc, msg.CompSep)
				if len(dParts) > 1 && strings.TrimSpace(dParts[1]) != "" {
					physician = strings.TrimSpace(dParts[1])
				} else if len(dParts) > 0 {
					physician = strings.TrimSpace(dParts[0])
				}
			}
		}
	}

	// Accession fallback
	if accession == "" {
		accession = "ACC" + time.Now().Format("20060102150405")
	}
	item.AccessionNumber = accession

	// Modality fallback
	if item.Modality == "" {
		item.Modality = "DX"
	}

	// Procedure description fallback
	if item.ProcedureDesc == "" {
		item.ProcedureDesc = "Pemeriksaan Radiologi"
	}

	// Scheduled date and time fallbacks
	if scheduledDate == "" {
		scheduledDate = time.Now().Format("20060102")
	}
	if scheduledTime == "" {
		scheduledTime = time.Now().Format("150405")
	}
	item.ScheduledDate = FormatDicomDate(scheduledDate)
	item.ScheduledTime = FormatDicomTime(scheduledTime)

	item.PhysicianName = physician
	item.StationAETitle = "*"

	// Strictly apply procedure description auto-copy rule across all 4 DICOM description tags
	item.ApplyProcedureDescriptionCopy()

	_ = mshSeg
	return item, &msg.MSH, nil
}

// BuildHL7ACK creates an RFC/HL7 v2 compliant ACK message for Mirth Connect / SIMRS
func BuildHL7ACK(msh HL7MSHInfo, ackCode string, text string) string {
	if ackCode == "" {
		ackCode = "AA" // Application Accept
	}
	if text == "" {
		if ackCode == "AA" {
			text = "Message accepted successfully"
		} else {
			text = "Error processing message"
		}
	}

	now := time.Now().Format("20060102150405")
	sendingApp := "HEALTH_CONNECT"
	sendingFac := "HEALTH_CONNECT"
	recApp := msh.SendingApp
	if recApp == "" {
		recApp = "SIMRS"
	}
	recFac := msh.SendingFacility
	if recFac == "" {
		recFac = "HOSPITAL"
	}
	controlID := msh.ControlID
	if controlID == "" {
		controlID = fmt.Sprintf("ACK%d", time.Now().UnixNano())
	}
	ver := msh.VersionID
	if ver == "" {
		ver = "2.4"
	}

	msgType := "ACK"
	if msh.MsgEvent != "" {
		msgType = fmt.Sprintf("ACK^%s", msh.MsgEvent)
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("MSH|^~\\&|%s|%s|%s|%s|%s||%s|%s|P|%s\r", sendingApp, sendingFac, recApp, recFac, now, msgType, controlID, ver))
	buf.WriteString(fmt.Sprintf("MSA|%s|%s|%s\r", ackCode, controlID, text))

	return buf.String()
}

// BuildMLLPFrame wraps an HL7 string inside MLLP delimiters (\x0B ... \x1C\x0D)
func BuildMLLPFrame(hl7Msg string) []byte {
	return fmt.Appendf(nil, "\x0B%s\x1C\x0D", hl7Msg)
}

// detectModality inspects procedure description and service code to determine DICOM Modality
func detectModality(desc, code string) string {
	descUpper := strings.ToUpper(desc)
	codeUpper := strings.ToUpper(code)

	// 1. Check specific modality prefixes first
	if strings.HasPrefix(descUpper, "US ") || strings.HasPrefix(descUpper, "USG ") || strings.Contains(descUpper, "ULTRASOUND") || strings.Contains(descUpper, "SONOGRAFI") || strings.Contains(codeUpper, "US") {
		return "US"
	}
	if strings.HasPrefix(descUpper, "CT ") || strings.Contains(descUpper, "CT SCAN") || strings.Contains(descUpper, "CT-SCAN") || strings.Contains(codeUpper, "CT") {
		return "CT"
	}
	if strings.HasPrefix(descUpper, "MR ") || strings.HasPrefix(descUpper, "MRI ") || strings.Contains(descUpper, "MAGNETIC RESONANCE") || strings.Contains(codeUpper, "MR") {
		return "MR"
	}
	if strings.HasPrefix(descUpper, "MG ") || strings.Contains(descUpper, "MAMMOGRA") || strings.Contains(codeUpper, "MG") {
		return "MG"
	}
	if strings.HasPrefix(descUpper, "XA ") || strings.Contains(descUpper, "ANGIOGRA") || strings.Contains(codeUpper, "XA") {
		return "XA"
	}
	if strings.HasPrefix(descUpper, "CR ") || strings.HasPrefix(descUpper, "CR_") || strings.Contains(descUpper, "CR X-RAY") {
		return "CR"
	}

	// 2. Check general radiography / X-Ray / Anatomy tokens
	if strings.HasPrefix(descUpper, "DX ") || strings.HasPrefix(descUpper, "DR ") || strings.Contains(descUpper, "RONTGEN") || strings.Contains(descUpper, "X-RAY") || strings.Contains(descUpper, "THORAX") || strings.Contains(descUpper, "ABDOMEN") {
		return "DX"
	}

	// 3. Check code tokens fallback
	if strings.HasPrefix(codeUpper, "RAD") || strings.Contains(codeUpper, "CR") {
		return "CR"
	}

	return "DX"
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// ConvertHL7ToOrderJSON converts an HL7 ORM message to standardized JSON string for pipelines
func ConvertHL7ToOrderJSON(raw string) (string, error) {
	item, _, err := ParseHL7ORMMessage(raw)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(item)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
