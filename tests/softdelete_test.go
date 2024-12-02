package tests

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	. "gorm.io/plugin/optimisticlock"
	"gorm.io/plugin/soft_delete"
)

type Model struct {
	ID        int64 `gorm:"column:id;primaryKey;autoIncrement"`
	Version   Version
	DeletedAt soft_delete.DeletedAt
}

func TestSoftDelete(t *testing.T) {
	DB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	DB = DB.Debug()

	_ = DB.Migrator().DropTable(&Model{})
	_ = DB.AutoMigrate(&Model{})

	data := Model{ID: 1}
	DB.Save(&data)

	if err = DB.Where("id = ?", 1).Delete(&Model{}).Error; err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	DB.Delete(&data)
}
