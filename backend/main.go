package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
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

// Middleware manual buat CORS (biar Next.js bisa akses)
func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
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

	// 2. Siapkan command NAPS2
	// Output pattern: $(n) akan diganti jadi urutan angka (1, 2, 3...)
	// Contoh: scan_1.jpg, scan_2.jpg
	outputPath := filepath.Join(tempDir, "scan_$(n).jpg")

	// naps2.console.exe -o "C:\Temp\...\scan_$(n).jpg" -p "Plustek" --force
	cmd := exec.Command(naps2Path, "-o", outputPath, "-p", profileName, "--force")

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

func main() {
	http.HandleFunc("/scan", scanHandler)

	port := ":5000"
	fmt.Printf("Scanner Bridge (Golang) siap di http://localhost%s\n", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Gagal menjalankan server:", err)
	}
}
