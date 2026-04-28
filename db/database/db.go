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
	NPSN      string    `json:"npsn" gorm:"type:varchar(50);index"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

type ScanRecordHistory struct {
	ID           uint      `gorm:"primaryKey"`
	ScanRecordID uint      `json:"scan_record_id" gorm:"index"`
	Action       string    `json:"action" gorm:"type:varchar(20)"`
	Catatan      string    `json:"catatan" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at"`
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

	err = DB.AutoMigrate(&ScanRecord{}, &ScanRecordHistory{})
	if err != nil {
		log.Fatal("Gagal migrasi tabel:", err)
	}

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

	fkName := "fk_scan_records_schools"
	if !migrator.HasConstraint(&ScanRecord{}, fkName) {
		fmt.Println("Menambahkan Foreign Key ke schools(npsn)...")
		err := DB.Exec(fmt.Sprintf(
			"ALTER TABLE scan_records ADD CONSTRAINT %s FOREIGN KEY (npsn) REFERENCES schools(npsn) ON DELETE CASCADE ON UPDATE CASCADE",
			fkName,
		)).Error

		if err != nil {
			log.Println("WARNING: Gagal nambah Foreign Key (mungkin tabel schools belum ada atau tipe data beda):", err)
		} else {
			fmt.Println("Foreign Key berhasil ditambahkan!")
		}
	}

	indexName := "idx_schools_termin"
	createIndexQuery := fmt.Sprintf("CREATE INDEX %s ON schools(termin)", indexName)
	if err := DB.Exec(createIndexQuery).Error; err != nil {
		if err.Error() != "" && (strings.Contains(err.Error(), "1061") || strings.Contains(err.Error(), "Duplicate key name")) {
		} else {
			log.Printf("WARNING: Gagal membuat index %s: %v\n", indexName, err)
		}
	} else {
		fmt.Printf("Index %s berhasil dibuat untuk optimasi!\n", indexName)
	}

	fmt.Println("Database & Table ready!")
}
