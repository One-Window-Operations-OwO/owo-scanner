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
	"regexp"
	"strings"
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

// --- HELPER UNTUK PROFIL PORTABLE & AUTO-PATCHING ---

type DeviceConfig struct {
	ID   string `xml:"ID"`
	Name string `xml:"Name"`
}

type ScanProfileStruct struct { // Nama beda biar ga conflict sama handler
	DisplayName string       `xml:"DisplayName"`
	Device      DeviceConfig `xml:"Device"`
	DriverName  string       `xml:"DriverName"`
	Version     int          `xml:"Version"`
	// Field lain ignore aja, XML decoder akan skip yg ga didefinisikan kalau pake tag yg spesifik
	// Tapi kalau mau write balik *lengkap*, kita butuh struct yang *lengkap* atau cara manipulasi string/bytes.
	// KARENA kita mau overwrite file, jika kita decode ke struct partial, saat encode field lain HILANG.
	// SOLUSI: Kita baca file sebagai string/byte, lalu replace string ID/Name nya pakai regex/string replace.
	// Ini lebih aman daripada mendefinisikan seluruh XML schema NAPS2 yang kompleks.
}

// Cari device yang terhubung via CLI
func findFirstDevice() (id, name, driver string, err error) {
	drivers := []string{"wia", "twain"}

	for _, d := range drivers {
		// naps2.console.exe --listdevices --driver wia
		cmd := exec.Command(naps2Path, "--listdevices", "--driver", d)
		output, err := cmd.CombinedOutput()
		if err != nil {
			continue // Coba driver berikutnya
		}

		outStr := string(output)
		lines := strings.Split(outStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Format WIA: "Device Name (ID)" atau "Device Name" tergantung versi
			// Kita ambil asumsi baris pertama yg valid adalah device.
			// NAPS2 biasanya output: "Friendly Name (ID)"
			// Kita coba parse sederhana.
			return line, line, d, nil // ID dan Name kita samakan dulu kalau bingung parsingnya
		}
	}
	return "", "", "", fmt.Errorf("no devices found")
}

// Fungsi Helper untuk sinkronisasi profiles.xml dari backend ke AppData NAPS2
// Supaya NAPS2 mengenali profile yang kita taruh di folder backend
// DAN update device ID sesuai mesin local
func syncProfilesToAppData() error {
	// 1. Lokasi Source (Local Backend)
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("gagal get working dir: %v", err)
	}
	srcPath := filepath.Join(wd, "profiles.xml")
	srcBytes, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("gagal baca source profiles.xml: %v", err)
	}
	srcContent := string(srcBytes)

	// 2. Logic Patching Device
	// Kita cari device lokal
	deviceID, deviceName, driver, err := findFirstDevice()
	if err == nil {
		// Device ditemukan! Kita patch content XML.
		// Asumsi XML backend punya placeholder atau device lama.
		// Kita ganti blok <Device>...</Device> atau ganti isinya.
		// Cara simple regex replace.
		// <ID>...</ID> dan <Name>...</Name> dan <DriverName>...</DriverName>

		fmt.Printf("Auto-Patching Profile ke Device: %s (%s - %s)\n", deviceName, deviceID, driver)

		// Ganti DriverName
		// Regex: <DriverName>.*?</DriverName>
		// Note: Hati-hati replace global kalau ada banyak profile.
		// Untuk sekarang kita assume "global replace" karena user mungkin mau SEMUA profile pake scanner yg kecolok.
		reDriver := regexp.MustCompile(`<DriverName>.*?</DriverName>`)
		srcContent = reDriver.ReplaceAllString(srcContent, fmt.Sprintf("<DriverName>%s</DriverName>", driver))

		// Ganti ID (Complex karena di dalam <Device>)
		// Kita coba replace string spesifik di dalam <Device> block.
		// Karena regex parsing XML beresiko, kita coba pendekatan "Best Effort"
		// Ganti semua ID di dalam profiles?
		// Lebih aman replace per tag globally, asumsi 1 jenis scanner.

		// Replace ID
		reID := regexp.MustCompile(`<ID>.*?</ID>`)
		// WARNING: Ini mereplace SEMUA ID, termasuk IconID {uid}. IconID biasanya angka/uuid pendek. Device ID panjang.
		// Device ID biasanya ada backslash atau kurung kurawal.
		// Lebih aman hanya replace <Device><ID>...</ID></Device> pattern, tapi regex multiline susah di Go stdlib (ga support (?s)).

		// Fallback: Kita decode XML -> Update Struct -> Encode XML? Resiko struct ga lengkap -> Data hilang.
		// Fallback 2: String replacement "Cerdas".
		// Kita cari "<Device>" lalu cari "<ID>...</ID>" setelahnya.

		devIndex := strings.Index(srcContent, "<Device>")
		if devIndex != -1 {
			// Kita potong string jadi beforeDevice, deviceBlock, afterDevice
			endDevIndex := strings.Index(srcContent[devIndex:], "</Device>")
			if endDevIndex != -1 {
				fullEndIndex := devIndex + endDevIndex + 9 // + len("</Device>")
				deviceBlock := srcContent[devIndex:fullEndIndex]

				// Replace ID dan Name di blok ini saja
				deviceBlock = reID.ReplaceAllString(deviceBlock, fmt.Sprintf("<ID>%s</ID>", deviceID))

				reName := regexp.MustCompile(`<Name>.*?</Name>`)
				deviceBlock = reName.ReplaceAllString(deviceBlock, fmt.Sprintf("<Name>%s</Name>", deviceName))

				// Reconstruct
				srcContent = srcContent[:devIndex] + deviceBlock + srcContent[fullEndIndex:]
			}
		}
	} else {
		fmt.Printf("Warning: Tidak ada scanner terdeteksi (%v). Skip patching, copy as-is.\n", err)
	}

	// 3. Lokasi Destination (AppData NAPS2)
	appData, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("gagal get AppData: %v", err)
	}
	destDir := filepath.Join(appData, "NAPS2")
	destPath := filepath.Join(destDir, "profiles.xml")

	// Pastikan folder NAPS2 ada
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("gagal buat folder NAPS2: %v", err)
	}

	// 4. Tulis ke Destination (Overwrite)
	if err := os.WriteFile(destPath, []byte(srcContent), 0644); err != nil {
		return fmt.Errorf("gagal tulis destination profiles.xml: %v", err)
	}

	fmt.Println("Berhasil sinkronisasi & patching profiles.xml.")
	return nil
}

func scanHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)

	// Kalau browser kirim preflight check (OPTIONS), langsung OK in aja
	if r.Method == "OPTIONS" {
		return
	}

	// ... (Rest of sync logic called below)
	fmt.Println("Menerima request scan...")

	// --- STEP SINKRONISASI PROFILE ---
	// Sebelum scan, pastikan NAPS2 pake profile lokal kita
	if err := syncProfilesToAppData(); err != nil {
		fmt.Printf("Warning: Gagal sync profiles: %v\n", err)
	}
	// ---------------------------------

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
	outputPath := filepath.Join(tempDir, "scan_$(n).jpg")
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

	// 4. Baca semua file hasil scan
	files, err := filepath.Glob(filepath.Join(tempDir, "scan_*.jpg"))
	if err != nil {
		fmt.Println("Gagal glob files:", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Success: false, Message: "Gagal mencari file hasil scan"})
		return
	}

	if len(files) == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Success: false, Message: "Tidak ada gambar yang dihasilkan. Cek koneksi scanner."})
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

	// 5. Loop file dengan step 2
	for i := 0; i < len(files); i += 2 {
		pair := ScanPair{}
		frontB64, err := fileToBase64(files[i])
		if err != nil {
			fmt.Printf("Error baca file %s: %v\n", files[i], err)
			continue
		}
		pair.Front = frontB64

		if i+1 < len(files) {
			backB64, err := fileToBase64(files[i+1])
			if err == nil {
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

	// Baca profiles.xml dari folder lokal
	dir, err := os.Getwd()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Success: false, Message: "Gagal mendeteksi folder aplikasi"})
		return
	}

	profilesPath := filepath.Join(dir, "profiles.xml")
	xmlFile, err := os.Open(profilesPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(Response{Success: false, Message: "Gagal membuka file profiles.xml di backend"})
		return
	}
	defer xmlFile.Close()

	// Struct minimalis untuk XML parsing
	type ScanProfile struct {
		DisplayName string `xml:"DisplayName"`
	}
	type ArrayOfScanProfile struct {
		Profiles []ScanProfile `xml:"ScanProfile"`
	}

	var data ArrayOfScanProfile
	byteValue, _ := io.ReadAll(xmlFile)

	if err := xml.Unmarshal(byteValue, &data); err != nil {
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

func main() {
	http.HandleFunc("/scan", scanHandler)
	http.HandleFunc("/profiles", profilesHandler)

	port := ":5000"
	fmt.Printf("Scanner Bridge (Golang) siap di http://localhost%s\n", port)
	fmt.Println("Hint: Pastikan NAPS2 terinstall di path default atau update variable 'naps2Path'.")

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Gagal menjalankan server:", err)
	}
}
