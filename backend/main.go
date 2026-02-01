package main

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"scanner-bridge/database"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// --- CONFIGURATION ---
// Sesuaikan path ini dengan lokasi install NAPS2 di PC lu
const naps2Path = "C:\\Program Files\\NAPS2\\NAPS2.console.exe"
const profileName = "Duplex ADF Scanner(K76)" // Harus sama dengan nama profile di NAPS2

// Struktur JSON Response
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

func scanHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)

	// Kalau browser kirim preflight check (OPTIONS), langsung OK in aja
	if r.Method == "OPTIONS" {
		return
	}

	fmt.Println("Menerima request scan...")

	// 1. Buat folder sementara khusus untuk request ini
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("scan_session_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		fmt.Println("Gagal buat temp dir:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Success: false, Message: "Gagal membuat temporary directory"})
		return
	}
	defer os.RemoveAll(tempDir) // Hapus folder temp setelah selesai

	// Ambil nama profile dari Query Param, kalau kosong pake default
	selectedProfile := r.URL.Query().Get("profile")
	if selectedProfile == "" {
		selectedProfile = profileName // Default value dari konstanta
	}

	// 2. Siapkan command NAPS2
	// Output pattern: $(n) akan diganti jadi urutan angka (1, 2, 3...)
	// Contoh: scan_1.jpg, scan_2.jpg
	outputPath := filepath.Join(tempDir, "scan_$(n).jpg")

	// naps2.console.exe -o "C:\Temp\...\scan_$(n).jpg" -p "Plustek" --force
	fmt.Printf("Scanning dengan profile: %s\n", selectedProfile)
	cmd := exec.Command(naps2Path, "-o", outputPath, "-p", selectedProfile, "--force")

	// 3. Eksekusi Command
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("Gagal scan: %v | Output NAPS2: %s", err, string(output))
		fmt.Println(errMsg)

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Success: false, Message: errMsg})
		return
	}

	// 4. Baca semua file hasil scan di folder temp
	files, err := filepath.Glob(filepath.Join(tempDir, "scan_*.jpg"))
	if err != nil {
		fmt.Println("Gagal glob files:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Success: false, Message: "Gagal mencari file hasil scan"})
		return
	}

	if len(files) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Success: false, Message: "Tidak ada gambar yang dihasilkan"})
		return
	}

	// Helper function baca file jadi Base64
	fileToBase64 := func(path string) (string, error) {
		imgBytes, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(imgBytes), nil
	}

	var scanResults []ScanPair

	// 5. Loop file dengan step 2 (0, 2, 4...)
	// Asumsi n urutannya benar berdasarkan nama glob
	// Kalau mau lebih aman harus sort dulu, tapi biasanya glob return sesuai OS (seringkali urut nama)
	// Kita percayakan naps2 exportnya scan_1.jpg, scan_2.jpg dsb dan glob sortnya workable.
	for i := 0; i < len(files); i += 2 {
		pair := ScanPair{}

		// Proses Front (i)
		frontB64, err := fileToBase64(files[i])
		if err != nil {
			fmt.Printf("Error baca file %s: %v\n", files[i], err)
			continue
		}
		pair.Front = frontB64

		// Proses Back (i+1) jika ada
		if i+1 < len(files) {
			backB64, err := fileToBase64(files[i+1])
			if err != nil {
				fmt.Printf("Error baca file %s: %v\n", files[i+1], err)
				// Kalau back gagal, tetep masukin front? atau error?
				// Kita keep front aja, back kosong.
			} else {
				pair.Back = backB64
			}
		}

		scanResults = append(scanResults, pair)
	}

	// 6. Kirim Response JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data:    scanResults,
	})

	fmt.Printf("Scan sukses! Mengirim %d pasang gambar ke browser.\n", len(scanResults))
}

func profilesHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	// Cari lokasi profiles.xml di AppData
	appData, err := os.UserConfigDir() // Usually C:\Users\Username\AppData\Roaming
	if err != nil {
		fmt.Printf("Gagal get AppData: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Success: false, Message: "Gagal mendeteksi folder AppData"})
		return
	}

	profilesPath := filepath.Join(appData, "NAPS2", "profiles.xml")
	xmlFile, err := os.Open(profilesPath)
	if err != nil {
		fmt.Printf("Gagal buka profiles.xml: %v\n", err)
		// Fallback: coba cari di local AppData kalau roaming gagal, atau return empty
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Success: false, Message: "Gagal membuka file profil NAPS2"})
		return
	}
	defer xmlFile.Close()

	// Simple XML parsing
	// Kita cuma butuh ambil isi tag <DisplayName>...</DisplayName>
	// Cara paling gampang tanpa bikin struct kompleks adalah baca file sebagai text dan regex,
	// atau decoding XML decodernya.
	// Kita coba pake standard xml decoder dengan struct minimalis
	type ScanProfile struct {
		DisplayName string `xml:"DisplayName"`
	}
	type ArrayOfScanProfile struct {
		Profiles []ScanProfile `xml:"ScanProfile"`
	}

	var data ArrayOfScanProfile
	byteValue, _ := io.ReadAll(xmlFile)

	// Perlu import "encoding/xml" dan "io"
	if err := xml.Unmarshal(byteValue, &data); err != nil {
		fmt.Printf("Gagal parsing XML: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Success: false, Message: "Gagal memparsing file profil"})
		return
	}

	var profiles []string
	for _, p := range data.Profiles {
		if p.DisplayName != "" {
			profiles = append(profiles, p.DisplayName)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"profiles": profiles,
	})
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

	storageDir := os.Getenv("SCAN_STORAGE_PATH")
	if storageDir == "" {
		storageDir = "C:\\bapp_storage\\scans"
	}
	os.MkdirAll(storageDir, 0755)

	fileNameBase := fmt.Sprintf("%s_%s", req.NPSN, req.SNBapp)
	pdfPath := filepath.Join(storageDir, fileNameBase+".pdf")

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
	http.HandleFunc("/scan", scanHandler)
	http.HandleFunc("/profiles", profilesHandler)
	http.HandleFunc("/save", saveHandler)
	database.InitDB()

	port := ":5000"
	fmt.Printf("Scanner Bridge (Golang) siap di http://localhost%s\n", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Gagal menjalankan server:", err)
	}
}
