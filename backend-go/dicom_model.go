package main

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DicomWorklistItem represents a patient examination order scheduled for DICOM Modality Worklist (MWL)
type DicomWorklistItem struct {
	ID              string `json:"id"`
	AccessionNumber string `json:"accession_number"`
	PatientID       string `json:"patient_id"`
	PatientName     string `json:"patient_name"`
	BirthDate       string `json:"birth_date"` // YYYYMMDD
	Gender          string `json:"gender"`     // M, F, O
	Modality        string `json:"modality"`   // DX, CR, CT, MR, US, MG, etc.
	ProcedureDesc   string `json:"procedure_desc"`
	StudyDesc       string `json:"study_desc"`
	ReqProcDesc     string `json:"req_proc_desc"`
	SeriesDesc      string `json:"series_desc"`
	StudyUID        string `json:"study_uid"`
	StationAETitle  string `json:"station_ae_title"`
	PhysicianName   string `json:"physician_name"`
	ScheduledDate   string `json:"scheduled_date"` // YYYYMMDD
	ScheduledTime   string `json:"scheduled_time"` // HHMMSS
	Status          string `json:"status"`         // SCHEDULED, IN_PROGRESS, COMPLETED, CANCELLED
	RawPayload      string `json:"raw_payload"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// GenerateDicomUID generates a standard, globally unique DICOM UID using the ISO/IEC 9834-8
// ITU-T Rec. X.667 root "2.25.<decimal-representation-of-uuid>". This format is universally
// accepted by all DICOM standard compliant PACS and modalities without requiring an assigned enterprise root.
func GenerateDicomUID() string {
	u := uuid.New()
	var bi big.Int
	bi.SetBytes(u[:])
	return "2.25." + bi.String()
}

// FormatDicomDate standardizes date string to DICOM DA VR format (YYYYMMDD)
func FormatDicomDate(input string) string {
	cleaned := strings.ReplaceAll(strings.ReplaceAll(input, "-", ""), "/", "")
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) == 8 {
		return cleaned
	}
	// Try parsing common formats
	for _, layout := range []string{"2006-01-02", "02/01/2006", "2006/01/02", "02-01-2006", time.RFC3339} {
		if t, err := time.Parse(layout, strings.TrimSpace(input)); err == nil {
			return t.Format("20060102")
		}
	}
	if len(cleaned) > 8 {
		return cleaned[:8]
	}
	return time.Now().Format("20060102")
}

// FormatDicomTime standardizes time string to DICOM TM VR format (HHMMSS)
func FormatDicomTime(input string) string {
	cleaned := strings.ReplaceAll(input, ":", "")
	cleaned = strings.TrimSpace(cleaned)
	if len(cleaned) == 6 {
		return cleaned
	}
	for _, layout := range []string{"15:04:05", "15:04", time.TimeOnly} {
		if t, err := time.Parse(layout, strings.TrimSpace(input)); err == nil {
			return t.Format("150405")
		}
	}
	if len(cleaned) >= 6 {
		return cleaned[:6]
	}
	if len(cleaned) == 4 {
		return cleaned + "00"
	}
	return time.Now().Format("150405")
}

// ApplyProcedureDescriptionCopy enforces the requirement that procedure_desc is automatically
// copied to study_desc, req_proc_desc, and series_desc for complete vendor compatibility.
func (item *DicomWorklistItem) ApplyProcedureDescriptionCopy() {
	item.ProcedureDesc = strings.TrimSpace(item.ProcedureDesc)
	item.StudyDesc = strings.TrimSpace(item.StudyDesc)
	item.ReqProcDesc = strings.TrimSpace(item.ReqProcDesc)
	item.SeriesDesc = strings.TrimSpace(item.SeriesDesc)

	// If procedure_desc is empty, fallback to available descriptions
	if item.ProcedureDesc == "" {
		if item.StudyDesc != "" {
			item.ProcedureDesc = item.StudyDesc
		} else if item.ReqProcDesc != "" {
			item.ProcedureDesc = item.ReqProcDesc
		} else if item.SeriesDesc != "" {
			item.ProcedureDesc = item.SeriesDesc
		} else {
			item.ProcedureDesc = "Pemeriksaan Radiologi"
		}
	}

	// Redundancy rule: copy procedure_desc to study_desc, req_proc_desc, and series_desc if empty or default
	if item.StudyDesc == "" {
		item.StudyDesc = item.ProcedureDesc
	}
	if item.ReqProcDesc == "" {
		item.ReqProcDesc = item.ProcedureDesc
	}
	if item.SeriesDesc == "" {
		item.SeriesDesc = item.ProcedureDesc
	}

	// Normalize UID
	if strings.TrimSpace(item.StudyUID) == "" {
		item.StudyUID = GenerateDicomUID()
	}

	// Normalize Date & Time
	if item.ScheduledDate == "" {
		item.ScheduledDate = time.Now().Format("20060102")
	} else {
		item.ScheduledDate = FormatDicomDate(item.ScheduledDate)
	}

	if item.ScheduledTime == "" {
		item.ScheduledTime = time.Now().Format("150405")
	} else {
		item.ScheduledTime = FormatDicomTime(item.ScheduledTime)
	}

	if item.StationAETitle == "" {
		item.StationAETitle = "*"
	}
	if item.Status == "" {
		item.Status = "SCHEDULED"
	}
}

// SaveDicomWorklistItem inserts or updates an item in the database using parameterized queries
func SaveDicomWorklistItem(item *DicomWorklistItem) error {
	item.ApplyProcedureDescriptionCopy()
	now := time.Now().Format(time.RFC3339)

	if item.ID == "" && item.AccessionNumber != "" {
		var existingID string
		err := db.QueryRow("SELECT id FROM dicom_worklist_items WHERE accession_number = ?", item.AccessionNumber).Scan(&existingID)
		if err == nil && existingID != "" {
			item.ID = existingID
		}
	}

	if item.ID == "" {
		item.ID = uuid.New().String()
		item.CreatedAt = now
		item.UpdatedAt = now

		query := `INSERT INTO dicom_worklist_items (
			id, accession_number, patient_id, patient_name, birth_date, gender, modality,
			procedure_desc, study_desc, req_proc_desc, series_desc, study_uid,
			station_ae_title, physician_name, scheduled_date, scheduled_time, status,
			raw_payload, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

		_, err := db.Exec(query,
			item.ID, item.AccessionNumber, item.PatientID, item.PatientName, item.BirthDate, item.Gender, item.Modality,
			item.ProcedureDesc, item.StudyDesc, item.ReqProcDesc, item.SeriesDesc, item.StudyUID,
			item.StationAETitle, item.PhysicianName, item.ScheduledDate, item.ScheduledTime, item.Status,
			item.RawPayload, item.CreatedAt, item.UpdatedAt,
		)
		return err
	}

	item.UpdatedAt = now
	query := `UPDATE dicom_worklist_items SET
		accession_number = ?, patient_id = ?, patient_name = ?, birth_date = ?, gender = ?, modality = ?,
		procedure_desc = ?, study_desc = ?, req_proc_desc = ?, series_desc = ?, study_uid = ?,
		station_ae_title = ?, physician_name = ?, scheduled_date = ?, scheduled_time = ?, status = ?,
		raw_payload = ?, updated_at = ?
		WHERE id = ?`

	_, err := db.Exec(query,
		item.AccessionNumber, item.PatientID, item.PatientName, item.BirthDate, item.Gender, item.Modality,
		item.ProcedureDesc, item.StudyDesc, item.ReqProcDesc, item.SeriesDesc, item.StudyUID,
		item.StationAETitle, item.PhysicianName, item.ScheduledDate, item.ScheduledTime, item.Status,
		item.RawPayload, item.UpdatedAt, item.ID,
	)
	return err
}

// GetDicomWorklistItems retrieves worklist items with optional filters
func GetDicomWorklistItems(dateFilter, modalityFilter, statusFilter, search string, limit int) ([]DicomWorklistItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var conditions []string
	var args []interface{}

	if dateFilter != "" {
		conditions = append(conditions, "scheduled_date = ?")
		args = append(args, FormatDicomDate(dateFilter))
	}
	if modalityFilter != "" && modalityFilter != "ALL" {
		conditions = append(conditions, "UPPER(modality) = ?")
		args = append(args, strings.ToUpper(strings.TrimSpace(modalityFilter)))
	}
	if statusFilter != "" && statusFilter != "ALL" {
		conditions = append(conditions, "UPPER(status) = ?")
		args = append(args, strings.ToUpper(strings.TrimSpace(statusFilter)))
	}
	if search != "" {
		pattern := "%" + strings.TrimSpace(search) + "%"
		conditions = append(conditions, "(patient_id LIKE ? OR patient_name LIKE ? OR accession_number LIKE ? OR procedure_desc LIKE ?)")
		args = append(args, pattern, pattern, pattern, pattern)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`SELECT
		id, accession_number, patient_id, patient_name, COALESCE(birth_date, ''), COALESCE(gender, ''), modality,
		procedure_desc, study_desc, req_proc_desc, COALESCE(series_desc, ''), study_uid,
		COALESCE(station_ae_title, '*'), COALESCE(physician_name, ''), scheduled_date, COALESCE(scheduled_time, ''), status,
		COALESCE(raw_payload, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM dicom_worklist_items
		%s
		ORDER BY scheduled_date DESC, scheduled_time DESC
		LIMIT ?`, whereClause)

	args = append(args, limit)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DicomWorklistItem
	for rows.Next() {
		var item DicomWorklistItem
		err := rows.Scan(
			&item.ID, &item.AccessionNumber, &item.PatientID, &item.PatientName, &item.BirthDate, &item.Gender, &item.Modality,
			&item.ProcedureDesc, &item.StudyDesc, &item.ReqProcDesc, &item.SeriesDesc, &item.StudyUID,
			&item.StationAETitle, &item.PhysicianName, &item.ScheduledDate, &item.ScheduledTime, &item.Status,
			&item.RawPayload, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// FindMatchingWorklistItems performs matching query for DICOM C-FIND SCP queries
func FindMatchingWorklistItems(scheduledDate, modality, patientID, accessionNum, patientName, stationAET string) ([]DicomWorklistItem, error) {
	var conditions []string
	var args []interface{}

	// By default for C-FIND MWL, only return active SCHEDULED or IN_PROGRESS items
	conditions = append(conditions, "status IN ('SCHEDULED', 'IN_PROGRESS')")

	if scheduledDate != "" && scheduledDate != "*" {
		// Can be date range "YYYYMMDD-YYYYMMDD" or single date "YYYYMMDD"
		if strings.Contains(scheduledDate, "-") {
			parts := strings.Split(scheduledDate, "-")
			if len(parts) == 2 {
				startDate := strings.TrimSpace(parts[0])
				endDate := strings.TrimSpace(parts[1])
				if startDate != "" && endDate != "" {
					conditions = append(conditions, "scheduled_date BETWEEN ? AND ?")
					args = append(args, startDate, endDate)
				} else if startDate != "" {
					conditions = append(conditions, "scheduled_date >= ?")
					args = append(args, startDate)
				} else if endDate != "" {
					conditions = append(conditions, "scheduled_date <= ?")
					args = append(args, endDate)
				}
			}
		} else {
			conditions = append(conditions, "scheduled_date = ?")
			args = append(args, scheduledDate)
		}
	}

	if modality != "" && modality != "*" {
		conditions = append(conditions, "UPPER(modality) = ?")
		args = append(args, strings.ToUpper(strings.TrimSpace(modality)))
	}

	if patientID != "" && patientID != "*" {
		cleanID := strings.ReplaceAll(patientID, "*", "%")
		conditions = append(conditions, "patient_id LIKE ?")
		args = append(args, cleanID)
	}

	if accessionNum != "" && accessionNum != "*" {
		cleanAcc := strings.ReplaceAll(accessionNum, "*", "%")
		conditions = append(conditions, "accession_number LIKE ?")
		args = append(args, cleanAcc)
	}

	if patientName != "" && patientName != "*" {
		cleanName := strings.ReplaceAll(patientName, "*", "%")
		cleanName = strings.ReplaceAll(cleanName, "^", "%")
		conditions = append(conditions, "patient_name LIKE ?")
		args = append(args, "%"+cleanName+"%")
	}

	if stationAET != "" && stationAET != "*" {
		conditions = append(conditions, "(station_ae_title = ? OR station_ae_title = '*')")
		args = append(args, stationAET)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`SELECT
		id, accession_number, patient_id, patient_name, COALESCE(birth_date, ''), COALESCE(gender, ''), modality,
		procedure_desc, study_desc, req_proc_desc, COALESCE(series_desc, ''), study_uid,
		COALESCE(station_ae_title, '*'), COALESCE(physician_name, ''), scheduled_date, COALESCE(scheduled_time, ''), status,
		COALESCE(raw_payload, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM dicom_worklist_items
		%s
		ORDER BY scheduled_date ASC, scheduled_time ASC
		LIMIT 200`, whereClause)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []DicomWorklistItem
	for rows.Next() {
		var item DicomWorklistItem
		err := rows.Scan(
			&item.ID, &item.AccessionNumber, &item.PatientID, &item.PatientName, &item.BirthDate, &item.Gender, &item.Modality,
			&item.ProcedureDesc, &item.StudyDesc, &item.ReqProcDesc, &item.SeriesDesc, &item.StudyUID,
			&item.StationAETitle, &item.PhysicianName, &item.ScheduledDate, &item.ScheduledTime, &item.Status,
			&item.RawPayload, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
