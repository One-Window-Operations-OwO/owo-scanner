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
const profileName = "Plustek" // Harus sama dengan nama profile di NAPS2

// Struktur JSON Response
type Response struct {
	Success bool   `json:"success"`
	Image   string `json:"image,omitempty"` // Base64 string
	Message string `json:"message,omitempty"`
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

	// 1. Tentukan nama file sementara
	fileName := fmt.Sprintf("scan_%d.jpg", time.Now().UnixNano())
	tempDir := os.TempDir() // Pake folder temp bawaan Windows
	outputPath := filepath.Join(tempDir, fileName)

	// 2. Siapkan command NAPS2
	// naps2.console.exe -o "C:\Temp\...\scan_123.jpg" -p "Plustek" --force
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

	// 4. Baca file gambar hasil scan
	imgBytes, err := os.ReadFile(outputPath)
	if err != nil {
		fmt.Println("Gagal baca file output:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Success: false, Message: "Gagal membaca hasil scan"})
		return
	}

	// 5. Hapus file sementara (Cleanup)
	// Kita pake 'defer' gak bisa disini karena kita mau hapus SEGERA setelah dibaca
	// biar disk gak penuh kalau request banyak.
	err = os.Remove(outputPath)
	if err != nil {
		fmt.Println("Warning: Gagal hapus file temp:", err)
	}

	// 6. Convert ke Base64
	base64Str := base64.StdEncoding.EncodeToString(imgBytes)
	finalString := "data:image/jpeg;base64," + base64Str

	// 7. Kirim Response JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Image:   finalString,
	})

	fmt.Println("Scan sukses! Mengirim data ke browser.")
}

func main() {
	http.HandleFunc("/scan", scanHandler)

	port := ":5000"
	fmt.Printf("Scanner Bridge (Golang) siap di http://localhost%s\n", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Gagal menjalankan server:", err)
	}
}
