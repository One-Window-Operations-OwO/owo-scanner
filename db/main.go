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
type ScanRecord struct {
	ID        uint      `gorm:"primaryKey"`
	DocName   string    `json:"doc_name"`
	NPSN      string    `json:"npsn"`
	SNBapp    string    `gorm:"uniqueIndex" json:"sn_bapp"`
	HasilCek  string    `json:"hasil_cek"`
	Path      string    `json:"path"` // <--- CUMA SATU KOLOM INI AJA
	CreatedAt time.Time `json:"created_at"`
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

	// --- CEK DUPLIKAT DULU DI SINI ---
	var count int64
	database.DB.Model(&database.ScanRecord{}).Where("sn_bapp = ?", req.SNBapp).Count(&count)
	if count > 0 {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Data gagal disimpan! SN BAPP sudah terdaftar.",
		})
		return
	}
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
		DocName:  req.DocName,
		NPSN:     req.NPSN,
		SNBapp:   req.SNBapp,
		HasilCek: req.HasilCek,
		Path:     pdfPath,
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
func main() {
	http.HandleFunc("/save", saveHandler)
	database.InitDB()

	port := ":5000"
	fmt.Printf("Database API (Golang) siap di http://localhost%s\n", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Gagal menjalankan server:", err)
	}
}
