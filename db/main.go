package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"scanner-bridge/database"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

type ScanPair struct {
	Front string `json:"front"`          // Base64 string
	Back  string `json:"back,omitempty"` // Base64 string
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
	Kode       string `json:"kode"` // Added field
	ImageFront string `json:"image_front"`
	ImageBack  string `json:"image_back"`
}

// Middleware manual buat CORS (biar Next.js bisa akses)
func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS, POST")
	(*w).Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// 1. Update dulu struct di database/db.go kamu biar cuma satu kolom path
// 1. Update dulu struct di database/db.go kamu biar cuma satu kolom path
// (Struct lokal ini dihapus biar gak bingung, pakai yang di database/db.go aja)

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

	// --- CEK DUPLIKAT DULU DI SINI ---
	// (Check Logika Duplikat dihapus karena kolom sn_bapp sudah didrop)
	// Jika ingin cek duplikat, bisa pakai NPSN saja atau logika lain.
	// Untuk sekarang, kita allow multiple scan per NPSN atau client yang handle.
	// --------------------------------
	// --------------------------------

	storageDir := os.Getenv("SCAN_STORAGE_PATH")
	if storageDir == "" {
		storageDir = "./scans" // Default Linux friendly
	}
	os.MkdirAll(storageDir, 0755)

	fileNameBase := fmt.Sprintf("%s_%s", req.NPSN, req.SNBapp)
	pdfPath := filepath.Join(storageDir, fileNameBase+".pdf")

	// Cek juga secara fisik apakah filenya ada (opsional tapi bagus buat jaga-jaga)
	if _, err := os.Stat(pdfPath); err == nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "File PDF sudah ada di storage!"})
		return
	}

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

	newRecord := database.ScanRecord{
		NPSN: req.NPSN,
		Path: pdfPath,
	}

	result := database.DB.Create(&newRecord)
	if result.Error != nil {
		os.Remove(pdfPath)

		fmt.Println("Gagal simpan (Duplikat?):", result.Error)
		w.WriteHeader(http.StatusConflict) // 409 Conflict
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Data gagal disimpan! NPSN atau SN BAPP mungkin sudah terdaftar.",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Dokumen berhasil digabung jadi PDF dan disimpan!",
	})
}

type DashboardStat struct {
	Termin       string `json:"termin"`
	TotalSchools int64  `json:"total_schools"`
	Scanned      int64  `json:"scanned"`
	LogsAccepted int64  `json:"logs_accepted"`
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	// Helper structs for partial results
	type Res struct {
		Termin string
		Cnt    int64
	}

	var totalRes, scannedRes, logsRes []Res
	statsMap := make(map[string]*DashboardStat)

	// 1. Get Total Schools per Termin
	// SELECT termin, COUNT(*) FROM schools GROUP BY termin
	database.DB.Table("schools").Select("termin, count(*) as cnt").Group("termin").Scan(&totalRes)

	// 2. Get Scanned Count per Termin
	database.DB.Table("schools").
		Joins("INNER JOIN scan_records ON scan_records.npsn = schools.npsn").
		Select("schools.termin, count(distinct scan_records.npsn) as cnt").
		Group("schools.termin").
		Scan(&scannedRes)

	// 3. Get Logs Accepted Count per Termin
	database.DB.Table("schools").
		Joins("INNER JOIN logs ON logs.npsn = schools.npsn").
		Where("logs.hasil_cek = ?", "sesuai").
		Select("schools.termin, count(distinct logs.npsn) as cnt").
		Group("schools.termin").
		Scan(&logsRes)

	// Merge Results
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

	// Convert map to slice (Random order, sorted in frontend)
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

func recordsHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	searchNPSN := r.URL.Query().Get("npsn")
	var records []RecordResponse

	// Query kompleks untuk join scan_records, schools, dan log terakhir
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
		log.Println("Error fetching records:", err)
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

type IsApprovedResult struct {
	HasilCek    string `json:"hasil_cek"`
	NPSN        string `json:"npsn"`
	SNBapp      string `json:"sn_bapp" gorm:"column:sn_bapp"`
	NamaSekolah string `json:"nama_sekolah" gorm:"column:nama_sekolah"`
	Kode        string `json:"kode" gorm:"column:kode"`
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

	// Query raw SQL based on parameter
	if noBapp != "" {
		err = database.DB.Raw("SELECT hasil_cek, npsn, sn_bapp, nama_sekolah, kode FROM v_logs WHERE nomor_bapp = ? ORDER BY tanggal_pengecekan DESC, id DESC", noBapp).Scan(&results).Error
	} else {
		err = database.DB.Raw("SELECT hasil_cek, npsn, sn_bapp, nama_sekolah, kode FROM v_logs WHERE npsn = ? ORDER BY tanggal_pengecekan DESC, id DESC", npsn).Scan(&results).Error
	}

	if err != nil {
		log.Println("Error querying v_logs:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Gagal mengecek status approval.", "error": err.Error()})
		return
	}

	// Deduplication (Client side logic migration: unique NPSN, take first)
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

func main() {
	http.HandleFunc("/save", saveHandler)
	http.HandleFunc("/stats", statsHandler)
	http.HandleFunc("/records", recordsHandler)
	http.HandleFunc("/is-approved", isApprovedHandler)
	database.InitDB()

	// Serve Static Files (Scans)
	storageDir := os.Getenv("SCAN_STORAGE_PATH")
	if storageDir == "" {
		storageDir = "./scans"
	}
	// Pastikan folder ada
	os.MkdirAll(storageDir, 0755)

	fs := http.FileServer(http.Dir(storageDir))
	http.Handle("/scans/", http.StripPrefix("/scans/", fs))

	port := ":5000"
	fmt.Printf("Database API (Golang) siap di http://localhost%s\n", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Gagal menjalankan server:", err)
	}
}
