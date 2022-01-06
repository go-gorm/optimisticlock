package optimisticlock

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	ID      int
	Name    string
	Age     uint
	Version Version
}

func TestVersion(t *testing.T) {
	DB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	DB = DB.Debug()
	require.Nil(t, err)

	user := User{Name: "bob", Age: 20}
	_ = DB.Migrator().DropTable(&User{})
	_ = DB.AutoMigrate(&User{})
	DB.Save(&user)

	require.Equal(t, int64(1), user.Version.Int64)

	rows := DB.Model(&user).Update("age", 18).RowsAffected
	require.Equal(t, int64(1), rows)

	err = DB.First(&user, "id = 1").Error
	require.Nil(t, err)
	require.Equal(t, int64(2), user.Version.Int64)
	t.Logf("%+v", user)

	rows = DB.Model(&user).Update("age", 16).RowsAffected
	require.Equal(t, int64(1), rows)

	rows = DB.Model(&user).Update("age", 14).RowsAffected
	require.Equal(t, int64(0), rows)

	// not version
	rows = DB.Model(&User{}).Where("id = 1").UpdateColumn("age", 12).RowsAffected
	require.Equal(t, int64(1), rows)

	err = DB.First(&user, "id = 1").Error
	require.Nil(t, err)
	require.Equal(t, int64(4), user.Version.Int64)
	t.Logf("%+v", user)

	rows = DB.Model(&user).Updates(&User{Name: "lewis"}).RowsAffected
	require.Equal(t, int64(1), rows)

	err = DB.First(&user, "id = 1").Error
	require.Nil(t, err)

	rows = DB.Model(&user).Updates(map[string]interface{}{
		"age": 18,
	}).RowsAffected
	require.Equal(t, int64(1), rows)

	sql := DB.Session(&gorm.Session{DryRun: true}).Model(&user).Updates(nil).Statement.SQL.String()
	require.Contains(t, sql, "`version`=`version`+1")

	sql = DB.Session(&gorm.Session{DryRun: true}).Model(&user).Updates(&User{
		Age:     19,
		Version: Version{Int64: 1},
	}).Statement.SQL.String()
	require.Contains(t, sql, "`version`=`version`+1")

	sql = DB.Session(&gorm.Session{DryRun: true}).Model(&user).Updates(map[string]interface{}{
		"age":     18,
		"version": 1,
	}).Statement.SQL.String()
	require.Contains(t, sql, "`version`=`version`+1")
}
