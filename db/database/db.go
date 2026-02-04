package database

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

type ScanRecord struct {
	ID        uint      `gorm:"primaryKey"`
	NPSN      string    `json:"npsn" gorm:"type:varchar(50);index"` // Update type agar bisa di-index/FK
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

func InitDB() {
	err := godotenv.Load()
	if err != nil {
		log.Println("Gak nemu file .env, lanjut pake env variable sistem ya bos")
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")

	dsnServer := fmt.Sprintf("%s:%s@tcp(%s:3306)/?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPass, dbHost)

	serverDB, err := gorm.Open(mysql.Open(dsnServer), &gorm.Config{})
	if err != nil {
		log.Fatal("Gagal konek ke server database:", err)
	}

	createDbQuery := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", dbName)
	if err := serverDB.Exec(createDbQuery).Error; err != nil {
		log.Fatal("Gagal bikin database:", err)
	}
	fmt.Printf("Database %s aman (sudah ada/baru dibuat)\n", dbName)

	dsnApp := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPass, dbHost, dbName)

	DB, err = gorm.Open(mysql.Open(dsnApp), &gorm.Config{})
	if err != nil {
		log.Fatal("Aduh, gagal nyambung ke database aplikasi:", err)
	}

	// 1. Auto Migrate (Update Kolom yang ada, Create Table)
	err = DB.AutoMigrate(&ScanRecord{})
	if err != nil {
		log.Fatal("Gagal migrasi tabel:", err)
	}

	// 2. Drop Kolom yang tidak diinginkan (Manual karena GORM gak otomatis drop)
	migrator := DB.Migrator()
	unusedCols := []string{"doc_name", "sn_bapp", "hasil_cek", "kode"}

	fmt.Println("Mengecek kolom yang harus dihapus...")
	for _, col := range unusedCols {
		if migrator.HasColumn(&ScanRecord{}, col) {
			fmt.Printf("Menghapus kolom: %s\n", col)
			if err := migrator.DropColumn(&ScanRecord{}, col); err != nil {
				log.Printf("Gagal hapus kolom %s: %v\n", col, err)
			}
		}
	}

	// 3. Tambah Foreign Key ke tabel schools (npsn)
	// Pastikan tabel schools ada dan npsn tipe datanya cocok (biasanya varchar)
	// Kita pakai Raw SQL biar aman kalau constraint belum ada

	// Cek constraint
	fkName := "fk_scan_records_schools"
	if !migrator.HasConstraint(&ScanRecord{}, fkName) {
		fmt.Println("Menambahkan Foreign Key ke schools(npsn)...")
		// Gunakan ALTER TABLE standard
		err := DB.Exec(fmt.Sprintf(
			"ALTER TABLE scan_records ADD CONSTRAINT %s FOREIGN KEY (npsn) REFERENCES schools(npsn) ON DELETE CASCADE ON UPDATE CASCADE",
			fkName,
		)).Error

		if err != nil {
			// Jangan fatal, print warning aja takutnya tabel schools belum ada atau tipe beda
			log.Println("WARNING: Gagal nambah Foreign Key (mungkin tabel schools belum ada atau tipe data beda):", err)
		} else {
			fmt.Println("Foreign Key berhasil ditambahkan!")
		}
	}

	// 4. Tambah Index pada schools(termin) untuk optimasi query stats
	// Cek apakah index sudah ada dengan query manual ke information_schema atau coba create & ignore error
	// Kita coba Create Index dan handle error jika sudah ada
	indexName := "idx_schools_termin"
	createIndexQuery := fmt.Sprintf("CREATE INDEX %s ON schools(termin)", indexName)
	if err := DB.Exec(createIndexQuery).Error; err != nil {
		// Error code 1061 = Duplicate key name (artinya index sudah ada)
		// Kita ignore error ini, tapi log error lain
		// Menggunakan basic string check karena error handling MySQL driver spesifik agak ribet di sini
		if err.Error() != "" && (strings.Contains(err.Error(), "1061") || strings.Contains(err.Error(), "Duplicate key name")) {
			// Sudah ada, gapapa
		} else {
			log.Printf("WARNING: Gagal membuat index %s: %v\n", indexName, err)
		}
	} else {
		fmt.Printf("Index %s berhasil dibuat untuk optimasi!\n", indexName)
	}

	fmt.Println("Database & Tabel siap dipake, Bos Xeyla! ✨")
}
