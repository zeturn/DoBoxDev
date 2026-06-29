package database

import (
	"docode/internal/models"
	"fmt"
	"log"
	"strings"
	"time"

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
	if err := DB.AutoMigrate(&models.User{}); err != nil {
		return fmt.Errorf("failed to migrate users table: %w", err)
	}
	if err := archiveLegacyProjectTables(); err != nil {
		return fmt.Errorf("failed to archive legacy project tables: %w", err)
	}
	if err := migrateLegacyUserOwnership(); err != nil {
		return fmt.Errorf("failed to migrate legacy ownership columns: %w", err)
	}
	if err := dropRebuildableIndexes(); err != nil {
		return fmt.Errorf("failed to drop rebuildable indexes: %w", err)
	}

	err := DB.AutoMigrate(
		&models.Project{},
		&models.Sandbox{},
		&models.AgentSession{},
		&models.ToolCall{},
		&models.Container{},
		&models.OperationAudit{},
	)

	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	log.Println("✓ Database migrations completed")
	return nil
}

func migrateLegacyUserOwnership() error {
	legacyTables := []string{"projects", "sandboxes", "agent_sessions", "tool_calls", "containers", "operation_audits"}
	for _, table := range legacyTables {
		if err := addLegacyColumn(table, "user_id", "integer NOT NULL DEFAULT 1"); err != nil {
			return err
		}
	}
	if err := addLegacyColumn("agent_sessions", "status", "text NOT NULL DEFAULT 'active'"); err != nil {
		return err
	}
	if err := addLegacyColumn("tool_calls", "status", "text NOT NULL DEFAULT 'succeeded'"); err != nil {
		return err
	}
	if err := addLegacyColumn("tool_calls", "tool_name", "text NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := addLegacyColumn("operation_audits", "status", "text NOT NULL DEFAULT 'success'"); err != nil {
		return err
	}
	if err := addLegacyColumn("operation_audits", "action", "text NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func archiveLegacyProjectTables() error {
	if !DB.Migrator().HasTable("projects") {
		return nil
	}
	var columns []struct {
		Name string
		Type string
	}
	if err := DB.Raw("PRAGMA table_info(`projects`)").Scan(&columns).Error; err != nil {
		return err
	}
	for _, column := range columns {
		if column.Name == "id" && strings.EqualFold(column.Type, "INTEGER") {
			return nil
		}
		if column.Name == "id" {
			suffix := fmt.Sprintf("legacy_%d", time.Now().Unix())
			for _, table := range []string{"tool_calls", "agent_sessions", "sandboxes", "projects"} {
				if DB.Migrator().HasTable(table) {
					if err := DB.Migrator().RenameTable(table, table+"_"+suffix); err != nil {
						return err
					}
				}
			}
			if err := dropRebuildableIndexes(); err != nil {
				return err
			}
			log.Printf("archived legacy project tables with suffix %s because projects.id is %s", suffix, column.Type)
			return nil
		}
	}
	return nil
}

func dropRebuildableIndexes() error {
	for _, index := range []string{
		"idx_projects_user_id",
		"idx_sandboxes_user_id",
		"idx_sandboxes_project_id",
		"idx_sandboxes_container_id",
		"idx_agent_sessions_user_id",
		"idx_agent_sessions_project_id",
		"idx_agent_sessions_status",
		"idx_tool_calls_user_id",
		"idx_tool_calls_project_id",
		"idx_tool_calls_agent_session_id",
		"idx_tool_calls_tool_name",
		"idx_tool_calls_status",
		"idx_containers_user_id",
		"idx_containers_container_id",
		"idx_operation_audits_user_id",
		"idx_operation_audits_container_id",
		"idx_operation_audits_action",
		"idx_operation_audits_status",
	} {
		if err := DB.Exec("DROP INDEX IF EXISTS `" + index + "`").Error; err != nil {
			return err
		}
	}
	return nil
}

func addLegacyColumn(table, column, definition string) error {
	if !DB.Migrator().HasTable(table) || DB.Migrator().HasColumn(table, column) {
		return nil
	}
	return DB.Exec("ALTER TABLE `" + table + "` ADD COLUMN `" + column + "` " + definition).Error
}

// GetDB returns the database instance
func GetDB() *gorm.DB {
	return DB
}
