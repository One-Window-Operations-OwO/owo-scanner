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
	SNBapp    string    `gorm:"uniqueIndex" json:"sn_bapp"`
	HasilCek  string    `json:"hasil_cek"`
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

	dsn := fmt.Sprintf("%s:%s@tcp(%s:3306)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPass, dbHost, dbName)

	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})

	if err != nil {
		log.Fatal("Aduh, gagal nyambung ke MariaDB:", err)
	}

	DB.AutoMigrate(&ScanRecord{})
	fmt.Println("Database MariaDB siap dipake, Bos!")
}
