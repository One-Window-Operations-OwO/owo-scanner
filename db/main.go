package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"scanner-bridge/database"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
)

const baseApiUrl = "https://s3.pnj-digit.site"

type ScanPair struct {
	Front string `json:"front"`
	Back  string `json:"back,omitempty"`
}

type DeleteRequest struct {
	NPSN string `json:"npsn"`
}

type Response struct {
	Success bool       `json:"success"`
	Data    []ScanPair `json:"data,omitempty"`
	Message string     `json:"message,omitempty"`
}

type SaveRequest struct {
	DocName    string `json:"doc_name"`
	NPSN       string `json:"npsn"`
	SNBapp     string `json:"sn_bapp"`
	HasilCek   string `json:"hasil_cek"`
	Kode       string `json:"kode"`
	ImageFront string `json:"image_front"`
	ImageBack  string `json:"image_back"`
}

type DashboardStat struct {
	Termin       string `json:"termin"`
	TotalSchools int64  `json:"total_schools"`
	Scanned      int64  `json:"scanned"`
	LogsAccepted int64  `json:"logs_accepted"`
}

type RecordResponse struct {
	ID          uint      `json:"id"`
	NPSN        string    `json:"npsn"`
	NamaSekolah string    `json:"nama_sekolah"`
	SNBapp      string    `json:"sn_bapp"`
	HasilCek    string    `json:"hasil_cek"`
	Kode        string    `json:"kode"`
	Path        string    `json:"path"`
	CreatedAt   time.Time `json:"created_at"`
}

type IsApprovedResult struct {
	HasilCek    string `json:"hasil_cek"`
	NPSN        string `json:"npsn"`
	SNBapp      string `json:"sn_bapp" gorm:"column:sn_bapp"`
	NamaSekolah string `json:"nama_sekolah" gorm:"column:nama_sekolah"`
	Kode        string `json:"kode" gorm:"column:kode"`
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS, POST")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func uploadToS3(filePath string, folder string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("folder", folder)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}
	io.Copy(part, file)
	writer.Close()

	req, err := http.NewRequest("POST", baseApiUrl+"/send", body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gagal upload s3 status %d", resp.StatusCode)
	}

	return nil
}

func deleteFromS3(fileName string, folder string) error {
	s3ApiUrl := fmt.Sprintf("%s/delete?folder=%s&file=%s", baseApiUrl, folder, fileName)
	req, err := http.NewRequest("DELETE", s3ApiUrl, nil)
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gagal hapus file s3 status %d", resp.StatusCode)
	}

	return nil
}

func home(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "API is running",
		"time":    time.Now(),
	})
}

func saveHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	var req SaveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Request tidak valid"})
		return
	}

	var count int64
	database.DB.Model(&database.ScanRecord{}).Where("npsn = ?", req.NPSN).Count(&count)
	if count > 0 {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Data dengan NPSN ini sudah ada! NPSN atau SN BAPP mungkin sudah terdaftar.",
		})
		return
	}

	fileNameBase := fmt.Sprintf("%s_%s", req.NPSN, req.SNBapp)
	pdfName := fileNameBase + ".pdf"
	pdfPath := filepath.Join(os.TempDir(), pdfName)

	pdf := gofpdf.New("P", "mm", "A4", "")

	processImg := func(b64Str, label string) string {
		if b64Str == "" {
			return ""
		}
		i := strings.Index(b64Str, ",")
		if i != -1 {
			b64Str = b64Str[i+1:]
		}
		data, _ := base64.StdEncoding.DecodeString(b64Str)

		tmpPath := filepath.Join(os.TempDir(), fmt.Sprintf("tmp_%s_%s.jpg", fileNameBase, label))
		os.WriteFile(tmpPath, data, 0644)

		pdf.AddPage()
		pdf.SetFont("Arial", "B", 12)
		pdf.Cell(0, 10, "Halaman "+label)
		pdf.ImageOptions(tmpPath, 10, 20, 190, 0, false, gofpdf.ImageOptions{ImageType: "JPG"}, 0, "")

		return tmpPath
	}

	tmpF := processImg(req.ImageFront, "Depan")
	tmpB := processImg(req.ImageBack, "Belakang")

	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Gagal membuat PDF"})
		return
	}

	if tmpF != "" {
		os.Remove(tmpF)
	}
	if tmpB != "" {
		os.Remove(tmpB)
	}

	errUpload := uploadToS3(pdfPath, "scan-bapp-ifp")
	os.Remove(pdfPath)

	if errUpload != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Gagal upload PDF ke S3"})
		return
	}

	newRecord := database.ScanRecord{
		NPSN: req.NPSN,
		Path: pdfName,
	}

	result := database.DB.Create(&newRecord)
	if result.Error != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Data gagal disimpan! NPSN atau SN BAPP mungkin sudah terdaftar.",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Dokumen berhasil digabung jadi PDF dan disimpan ke S3!",
	})
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	type Res struct {
		Termin string
		Cnt    int64
	}

	var totalRes, scannedRes, logsRes []Res
	statsMap := make(map[string]*DashboardStat)

	database.DB.Table("schools").Select("termin, count(*) as cnt").Group("termin").Scan(&totalRes)

	database.DB.Table("schools").
		Joins("INNER JOIN scan_records ON scan_records.npsn = schools.npsn").
		Select("schools.termin, count(distinct scan_records.npsn) as cnt").
		Group("schools.termin").
		Scan(&scannedRes)

	database.DB.Table("schools").
		Joins("INNER JOIN logs ON logs.npsn = schools.npsn").
		Where("logs.hasil_cek = ?", "sesuai").
		Select("schools.termin, count(distinct logs.npsn) as cnt").
		Group("schools.termin").
		Scan(&logsRes)

	getStat := func(termin string) *DashboardStat {
		if _, ok := statsMap[termin]; !ok {
			statsMap[termin] = &DashboardStat{Termin: termin}
		}
		return statsMap[termin]
	}

	for _, r := range totalRes {
		getStat(r.Termin).TotalSchools = r.Cnt
	}
	for _, r := range scannedRes {
		getStat(r.Termin).Scanned = r.Cnt
	}
	for _, r := range logsRes {
		getStat(r.Termin).LogsAccepted = r.Cnt
	}

	var finalStats []DashboardStat
	for _, s := range statsMap {
		if s.Termin != "" {
			finalStats = append(finalStats, *s)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    finalStats,
	})
}

func workStatsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")

	if startDate == "" || endDate == "" {
		now := time.Now().Format("2006-01-02")
		startDate = now
		endDate = now
	}

	var scannedCount int64

	database.DB.Table("scan_records").
		Where("DATE(created_at) BETWEEN ? AND ?", startDate, endDate).
		Count(&scannedCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]int64{
			"scanned": scannedCount,
		},
		"period": map[string]string{
			"start": startDate,
			"end":   endDate,
		},
	})
}

func recordsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	searchNPSN := r.URL.Query().Get("npsn")
	var records []RecordResponse

	query := `
		SELECT 
			sr.id, sr.npsn, sr.path, sr.created_at, 
			s.nama_sekolah, s.kode,
			l.sn_bapp, l.hasil_cek
		FROM scan_records sr
		LEFT JOIN schools s ON sr.npsn = s.npsn
		LEFT JOIN (
			SELECT l1.npsn, l1.sn_bapp, l1.hasil_cek
			FROM logs l1
			JOIN (
				SELECT MAX(id) as id FROM logs GROUP BY npsn
			) l2 ON l1.id = l2.id
		) l ON sr.npsn = l.npsn
		WHERE 1=1
	`

	args := []interface{}{}

	if searchNPSN != "" {
		query += " AND sr.npsn LIKE ?"
		args = append(args, "%"+searchNPSN+"%")
	}

	query += " ORDER BY sr.created_at DESC LIMIT 50"

	if err := database.DB.Raw(query, args...).Scan(&records).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Gagal ambil data records"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    records,
	})
}

func isApprovedHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	noBapp := r.URL.Query().Get("no_bapp")
	npsn := r.URL.Query().Get("npsn")

	if noBapp == "" && npsn == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Parameter no_bapp atau npsn wajib diisi."})
		return
	}

	var results []IsApprovedResult
	var err error

	if noBapp != "" {
		err = database.DB.Raw("SELECT hasil_cek, npsn, sn_bapp, nama_sekolah, kode FROM v_logs WHERE nomor_bapp = ? ORDER BY tanggal_pengecekan DESC, id DESC", noBapp).Scan(&results).Error
	} else {
		err = database.DB.Raw("SELECT hasil_cek, npsn, sn_bapp, nama_sekolah, kode FROM v_logs WHERE npsn = ? ORDER BY tanggal_pengecekan DESC, id DESC", npsn).Scan(&results).Error
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Gagal mengecek status approval.", "error": err.Error()})
		return
	}

	uniqueMap := make(map[string]IsApprovedResult)
	var uniqueLogs []IsApprovedResult

	for _, row := range results {
		if _, exists := uniqueMap[row.NPSN]; !exists {
			uniqueMap[row.NPSN] = row
			uniqueLogs = append(uniqueLogs, row)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Success",
		"data":    uniqueLogs,
	})
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Invalid request payload"})
		return
	}

	if req.NPSN == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "NPSN is required"})
		return
	}

	var records []database.ScanRecord
	if err := database.DB.Where("npsn = ?", req.NPSN).Find(&records).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Gagal mencari data di database"})
		return
	}

	deletedFilesCount := 0
	for _, record := range records {
		err := deleteFromS3(record.Path, "scan-bapp-ifp")
		if err == nil {
			deletedFilesCount++
		}
	}

	dbResult := database.DB.Unscoped().Where("npsn = ?", req.NPSN).Delete(&database.ScanRecord{})

	if dbResult.Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": dbResult.Error.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Berhasil menghapus %d data dari database dan %d file dari S3.", dbResult.RowsAffected, deletedFilesCount),
	})
}

func exportHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	type ExportRow struct {
		NPSN        string
		NamaSekolah string
		Termin      string
		CreatedAt   string
		Path        string
	}

	var rows []ExportRow

	query := `
		SELECT 
			sr.npsn, 
			s.nama_sekolah, 
			s.termin, 
			DATE_FORMAT(sr.created_at, '%Y-%m-%d %H:%i:%s') as created_at, 
			sr.path
		FROM scan_records sr
		LEFT JOIN schools s ON sr.npsn = s.npsn
		ORDER BY sr.created_at DESC
	`

	if err := database.DB.Raw(query).Scan(&rows).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	f := excelize.NewFile()
	defer func() {
		_ = f.Close()
	}()

	headers := []string{"NPSN", "Nama Sekolah", "Termin", "Tanggal Scan", "Link Path Scan"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Sheet1", cell, h)
	}

	for i, row := range rows {
		f.SetCellValue("Sheet1", fmt.Sprintf("A%d", i+2), row.NPSN)
		f.SetCellValue("Sheet1", fmt.Sprintf("B%d", i+2), row.NamaSekolah)
		f.SetCellValue("Sheet1", fmt.Sprintf("C%d", i+2), row.Termin)
		f.SetCellValue("Sheet1", fmt.Sprintf("D%d", i+2), row.CreatedAt)
		f.SetCellValue("Sheet1", fmt.Sprintf("E%d", i+2), row.Path)
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=export_records.xlsx")

	_ = f.Write(w)
}

func serveFromS3Handler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	fileName := strings.TrimPrefix(r.URL.Path, "/scans/")
	if fileName == "" {
		http.Error(w, "File tidak spesifik", http.StatusBadRequest)
		return
	}

	encodedFileName := url.QueryEscape(fileName)

	s3ApiUrl := fmt.Sprintf("%s/get?folder=scan-bapp-ifp&file=%s", baseApiUrl, encodedFileName)

	resp, err := http.Get(s3ApiUrl)
	if err != nil || resp.StatusCode != http.StatusOK {
		http.Error(w, "File tidak ditemukan di S3", http.StatusNotFound)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Content-Disposition", resp.Header.Get("Content-Disposition"))

	io.Copy(w, resp.Body)
}

func main() {
	http.HandleFunc("/delete", deleteHandler)
	http.HandleFunc("/save", saveHandler)
	http.HandleFunc("/stats", statsHandler)
	http.HandleFunc("/work-stats", workStatsHandler)
	http.HandleFunc("/records", recordsHandler)
	http.HandleFunc("/is-approved", isApprovedHandler)
	http.HandleFunc("/export", exportHandler)
	http.HandleFunc("/scans/", serveFromS3Handler)
	http.HandleFunc("/", home)

	database.InitDB()

	port := ":5000"
	fmt.Printf("Database API (Golang) siap di http://localhost%s\n", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Gagal menjalankan server:", err)
	}
}
