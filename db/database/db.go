package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

type ScanRecord struct {
	ID        uint      `gorm:"primaryKey"`
	DocName   string    `json:"doc_name"`
	NPSN      string    `json:"npsn"`
	SNBapp    string    `gorm:"type:varchar(191);uniqueIndex" json:"sn_bapp"`
	HasilCek  string    `json:"hasil_cek"`
	Kode      string    `json:"kode"` // Added Kode field
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

	err = DB.AutoMigrate(&ScanRecord{})
	if err != nil {
		log.Fatal("Gagal migrasi tabel:", err)
	}

	fmt.Println("Database & Tabel siap dipake, Bos Xeyla! ✨")
}
