package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RegisterDicomRoutes wires the DICOM REST API routes into Gin router
func RegisterDicomRoutes(r *gin.Engine) {
	api := r.Group("/api")
	{
		api.GET("/dicom-worklist", handleGetDicomWorklist)
		api.POST("/dicom-worklist", handleCreateDicomWorklist)
		api.GET("/dicom-worklist/:id", handleGetDicomWorklistById)
		api.PUT("/dicom-worklist/:id", handleUpdateDicomWorklist)
		api.DELETE("/dicom-worklist/:id", handleDeleteDicomWorklist)
		api.POST("/dicom-worklist/:id/status", handleUpdateDicomWorklistStatus)

		api.GET("/dicom-server/status", handleGetDicomServerStatus)
		api.POST("/dicom-server/control", handleControlDicomServer)
	}
}

// IngressOrderPayload provides flexible field mapping for various SIMRS JSON structures
type IngressOrderPayload struct {
	AccessionNumber string `json:"accession_number"`
	AccessionNo     string `json:"accession_no"`
	NoOrder         string `json:"no_order"`
	OrderNo         string `json:"order_no"`

	PatientID       string `json:"patient_id"`
	NoRM            string `json:"no_rm"`
	Norm            string `json:"norm"`
	MRN             string `json:"mrn"`

	PatientName     string `json:"patient_name"`
	NamaPasien      string `json:"nama_pasien"`
	Nama            string `json:"nama"`

	BirthDate       string `json:"birth_date"`
	TglLahir        string `json:"tgl_lahir"`
	TanggalLahir    string `json:"tanggal_lahir"`

	Gender          string `json:"gender"`
	JenisKelamin    string `json:"jenis_kelamin"`
	Sex             string `json:"sex"`

	Modality        string `json:"modality"`
	Modalitas       string `json:"modalitas"`

	ProcedureDesc   string `json:"procedure_desc"`
	ProcedureName   string `json:"procedure_name"`
	NamaTindakan    string `json:"nama_tindakan"`
	Tindakan        string `json:"tindakan"`
	StudyDesc       string `json:"study_desc"`
	ReqProcDesc     string `json:"req_proc_desc"`
	SeriesDesc      string `json:"series_desc"`

	StationAETitle  string `json:"station_ae_title"`
	AETitle         string `json:"ae_title"`

	PhysicianName   string `json:"physician_name"`
	DokterPengirim  string `json:"dokter_pengirim"`
	Dokter          string `json:"dokter"`

	ScheduledDate   string `json:"scheduled_date"`
	TglPeriksa      string `json:"tgl_periksa"`
	TanggalPeriksa  string `json:"tanggal_periksa"`

	ScheduledTime   string `json:"scheduled_time"`
	JamPeriksa      string `json:"jam_periksa"`

	Status          string `json:"status"`
}

func handleGetDicomWorklist(c *gin.Context) {
	dateFilter := c.Query("date")
	modalityFilter := c.Query("modality")
	statusFilter := c.Query("status")
	search := c.Query("search")
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)

	items, err := GetDicomWorklistItems(dateFilter, modalityFilter, statusFilter, search, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query worklist items"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"count": len(items),
	})
}

func handleCreateDicomWorklist(c *gin.Context) {
	var payload IngressOrderPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON payload: " + err.Error()})
		return
	}

	// Resolve fields using priority mapping
	acc := firstNonEmpty(payload.AccessionNumber, payload.AccessionNo, payload.NoOrder, payload.OrderNo)
	if acc == "" {
		acc = "ACC" + time.Now().Format("20060102150405")
	}

	patID := firstNonEmpty(payload.PatientID, payload.NoRM, payload.Norm, payload.MRN)
	if patID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing patient identifier (patient_id, no_rm, norm, or mrn)"})
		return
	}

	patName := firstNonEmpty(payload.PatientName, payload.NamaPasien, payload.Nama)
	if patName == "" {
		patName = "ANONYMOUS"
	}

	modality := strings.ToUpper(strings.TrimSpace(firstNonEmpty(payload.Modality, payload.Modalitas, "DX")))
	procDesc := firstNonEmpty(payload.ProcedureDesc, payload.ProcedureName, payload.NamaTindakan, payload.Tindakan, payload.StudyDesc, payload.ReqProcDesc, "Rontgen")
	
	birth := firstNonEmpty(payload.BirthDate, payload.TglLahir, payload.TanggalLahir)
	if birth != "" {
		birth = FormatDicomDate(birth)
	}

	gender := strings.ToUpper(strings.TrimSpace(firstNonEmpty(payload.Gender, payload.JenisKelamin, payload.Sex, "O")))
	if strings.HasPrefix(gender, "L") || gender == "M" || strings.HasPrefix(gender, "PRIA") {
		gender = "M"
	} else if strings.HasPrefix(gender, "P") || gender == "F" || strings.HasPrefix(gender, "WANITA") {
		gender = "F"
	} else {
		gender = "O"
	}

	schDate := firstNonEmpty(payload.ScheduledDate, payload.TglPeriksa, payload.TanggalPeriksa, time.Now().Format("20060102"))
	schTime := firstNonEmpty(payload.ScheduledTime, payload.JamPeriksa, time.Now().Format("150405"))
	station := firstNonEmpty(payload.StationAETitle, payload.AETitle, "*")
	physician := firstNonEmpty(payload.PhysicianName, payload.DokterPengirim, payload.Dokter, "")

	rawBytes, _ := json.Marshal(payload)

	item := &DicomWorklistItem{
		AccessionNumber: acc,
		PatientID:       patID,
		PatientName:     patName,
		BirthDate:       birth,
		Gender:          gender,
		Modality:        modality,
		ProcedureDesc:   procDesc,
		StudyDesc:       payload.StudyDesc,
		ReqProcDesc:     payload.ReqProcDesc,
		SeriesDesc:      payload.SeriesDesc,
		StationAETitle:  station,
		PhysicianName:   physician,
		ScheduledDate:   schDate,
		ScheduledTime:   schTime,
		Status:          firstNonEmpty(payload.Status, "SCHEDULED"),
		RawPayload:      string(rawBytes),
	}

	// Enforce auto-copy procedure_desc to study_desc, req_proc_desc, and series_desc
	item.ApplyProcedureDescriptionCopy()

	if err := SaveDicomWorklistItem(item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save worklist item: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "DICOM Worklist item created successfully",
		"item":    item,
	})
}

func handleGetDicomWorklistById(c *gin.Context) {
	id := c.Param("id")
	var item DicomWorklistItem

	query := `SELECT id, accession_number, patient_id, patient_name, COALESCE(birth_date, ''), COALESCE(gender, ''), modality,
		procedure_desc, study_desc, req_proc_desc, COALESCE(series_desc, ''), study_uid,
		COALESCE(station_ae_title, '*'), COALESCE(physician_name, ''), scheduled_date, COALESCE(scheduled_time, ''), status,
		COALESCE(raw_payload, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
		FROM dicom_worklist_items WHERE id = ?`

	err := db.QueryRow(query, id).Scan(
		&item.ID, &item.AccessionNumber, &item.PatientID, &item.PatientName, &item.BirthDate, &item.Gender, &item.Modality,
		&item.ProcedureDesc, &item.StudyDesc, &item.ReqProcDesc, &item.SeriesDesc, &item.StudyUID,
		&item.StationAETitle, &item.PhysicianName, &item.ScheduledDate, &item.ScheduledTime, &item.Status,
		&item.RawPayload, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worklist item not found"})
		return
	}

	c.JSON(http.StatusOK, item)
}

func handleUpdateDicomWorklist(c *gin.Context) {
	id := c.Param("id")
	var item DicomWorklistItem
	if err := c.ShouldBindJSON(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
		return
	}

	item.ID = id
	item.ApplyProcedureDescriptionCopy()

	if err := SaveDicomWorklistItem(&item); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Worklist item updated successfully",
		"item":    item,
	})
}

func handleDeleteDicomWorklist(c *gin.Context) {
	id := c.Param("id")
	res, err := db.Exec("DELETE FROM dicom_worklist_items WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete item: " + err.Error()})
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worklist item not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Worklist item deleted successfully"})
}

func handleUpdateDicomWorklistStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status field is required (e.g. SCHEDULED, IN_PROGRESS, COMPLETED, CANCELLED)"})
		return
	}

	now := time.Now().Format(time.RFC3339)
	res, err := db.Exec("UPDATE dicom_worklist_items SET status = ?, updated_at = ? WHERE id = ?", strings.ToUpper(req.Status), now, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Worklist item not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Status updated to " + strings.ToUpper(req.Status)})
}

func handleGetDicomServerStatus(c *gin.Context) {
	server := InitDicomServer()
	stats := server.GetStats()
	c.JSON(http.StatusOK, stats)
}

func handleControlDicomServer(c *gin.Context) {
	var req struct {
		Action  string `json:"action" binding:"required"` // start, stop, restart
		Port    int    `json:"port"`
		AETitle string `json:"ae_title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "action is required ('start', 'stop', 'restart')"})
		return
	}

	server := InitDicomServer()
	var err error

	switch strings.ToLower(req.Action) {
	case "start":
		err = server.Start()
	case "stop":
		err = server.Stop()
	case "restart":
		err = server.Restart(req.Port, req.AETitle)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action. Supported: 'start', 'stop', 'restart'"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "stats": server.GetStats()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "DICOM server action '" + req.Action + "' completed successfully",
		"stats":   server.GetStats(),
	})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		trimmed := strings.TrimSpace(v)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
