package optimisticlock

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
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
	require.Equal(t, "bob", user.Name)
	require.Equal(t, uint(20), user.Age)
	require.Equal(t, int64(1), user.Version.Int64)

	rows := DB.Model(&user).Update("age", 18).RowsAffected
	require.Equal(t, int64(1), rows)
	require.Nil(t, DB.First(&user).Error)
	require.Equal(t, int64(2), user.Version.Int64)
	require.Equal(t, uint(18), user.Age)

	rows = DB.Model(&user).Update("age", 16).RowsAffected
	require.Equal(t, int64(1), rows)
	require.Equal(t, uint(16), user.Age)

	rows = DB.Model(&user).Update("age", 14).RowsAffected
	require.Equal(t, int64(0), rows)

	// not version
	rows = DB.Model(&User{}).Where("id = 1").UpdateColumn("age", 12).RowsAffected
	require.Equal(t, int64(1), rows)
	require.Nil(t, DB.First(&user).Error)
	require.Equal(t, int64(4), user.Version.Int64)
	require.Equal(t, uint(12), user.Age)

	rows = DB.Model(&user).Updates(&User{Name: "lewis"}).RowsAffected
	require.Equal(t, int64(1), rows)
	require.Nil(t, DB.First(&user).Error)
	require.Equal(t, int64(5), user.Version.Int64)
	require.Equal(t, "lewis", user.Name)

	rows = DB.Model(&user).Updates(map[string]interface{}{
		"age": 18,
	}).RowsAffected
	require.Equal(t, int64(1), rows)
	require.Nil(t, DB.First(&user).Error)
	require.Equal(t, int64(6), user.Version.Int64)
	require.Equal(t, uint(18), user.Age)

	sql := DB.Session(&gorm.Session{DryRun: true}).Model(&user).Updates(nil).Statement.SQL.String()
	require.Contains(t, sql, "`version`=`version`+1")

	sql = DB.Session(&gorm.Session{DryRun: true}).Model(&user).Updates(&User{
		Age:     19,
		Version: Version{Int64: 10000000},
	}).Statement.SQL.String()
	require.Contains(t, sql, "`version`=`version`+1")

	sql = DB.Session(&gorm.Session{DryRun: true}).Model(&user).Updates(map[string]interface{}{
		"age":     18,
		"version": 1,
	}).Statement.SQL.String()
	require.Contains(t, sql, "`version`=`version`+1")

	b, err := json.Marshal(user)
	require.Nil(t, err)
	require.Equal(t, `{"ID":1,"Name":"lewis","Age":18,"Version":6}`, string(b))

	user.Version.Valid = false
	b, err = json.Marshal(user)
	require.Nil(t, err)
	require.Equal(t, `{"ID":1,"Name":"lewis","Age":18,"Version":null}`, string(b))
}

type Ext struct {
	CreditCard []string
}

type Account struct {
	gorm.Model

	UserID uint
	User   *User

	Amount int `gorm:"column:amount"`
	Ext    Ext `gorm:"column:ext"`

	Version Version
}

func (e *Ext) Scan(value interface{}) error {
	bs, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("expected []byte, got %T", bs)
	}

	if len(bs) == 0 {
		return nil
	}

	if err := json.Unmarshal(bs, e); nil != err {
		return err
	}

	return nil
}

func (e Ext) Value() (driver.Value, error) {
	bs, err := json.Marshal(e)
	if nil != err {
		return nil, fmt.Errorf("json Marshal err: %w", err)
	}

	return bs, nil
}

func TestEmbed(t *testing.T) {
	DB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	DB = DB.Debug()
	require.Nil(t, err)

	user := User{Name: "bob", Age: 20}
	_ = DB.Migrator().DropTable(&User{})
	_ = DB.AutoMigrate(&User{})
	DB.Save(&user)

	account := Account{
		UserID: 1,
		Amount: 1000,
		Ext:    Ext{CreditCard: []string{"123456", "456123"}},
	}
	_ = DB.Migrator().DropTable(&Account{})
	_ = DB.AutoMigrate(&Account{})
	DB.Save(&account)

	sql := DB.Session(&gorm.Session{DryRun: true}).Updates(&account).Statement.SQL.String()
	require.Equal(t, "UPDATE `accounts` SET `amount`=?,`created_at`=?,`ext`=?,`id`=?,`updated_at`=?,`user_id`=?,`version`=`version`+1 WHERE `accounts`.`deleted_at` IS NULL AND `accounts`.`version` = ? AND `id` = ?", sql)

	var a Account
	DB.First(&a)

	a.Amount = 233
	err = DB.Updates(&a).Error
	require.Nil(t, err)

	a.Amount = 556
	rows := DB.Updates(&a).RowsAffected
	require.Equal(t, int64(0), rows)
	require.Nil(t, DB.First(&a).Error)
	require.Equal(t, 233, a.Amount)

	a.Amount = 9999
	rows = DB.Updates(&a).RowsAffected
	require.Equal(t, int64(1), rows)

	var a1 Account
	require.Nil(t, DB.First(&a1).Error)
	require.Equal(t, a.Amount, a1.Amount)
	require.Equal(t, int64(2), a.Version.Int64)
	require.Equal(t, int64(3), a1.Version.Int64)
}
