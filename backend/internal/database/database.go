package database

import (
	"docode/internal/models"
	"fmt"
	"log"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	// Use modernc.org/sqlite (pure Go, no CGO required)
	sqlite "github.com/glebarez/sqlite"
)

var DB *gorm.DB

// Initialize database connection
func Connect(dbPath string) error {
	var err error

	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	log.Println("✓ Database connected successfully")

	// Auto migrate the schema
	return Migrate()
}

// Migrate runs database migrations
func Migrate() error {
	err := DB.AutoMigrate(
		&models.User{},
		&models.Container{},
		&models.OperationAudit{},
	)

	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("✓ Database migrations completed")
	return nil
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}
