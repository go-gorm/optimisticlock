package optimisticlock

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	mysqlDSN     = "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8&parseTime=True&loc=Local"
	postgresDSN  = "user=gorm password=gorm dbname=gorm host=localhost port=9920 sslmode=disable TimeZone=Asia/Shanghai"
	sqlserverDSN = "sqlserver://gorm:LoremIpsum86@localhost:9930?database=gorm"
)

type User struct {
	gorm.Model

	Name    string
	Age     uint
	Version Version
}

func TestVersion(t *testing.T) {
	DB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.Nil(t, err)

	user := User{Name: "bob", Age: 20}
	_ = DB.Migrator().DropTable(&User{})
	_ = DB.AutoMigrate(&User{})
	DB.Save(&user)
	require.Equal(t, "bob", user.Name)
	require.Equal(t, uint(20), user.Age)
	require.Equal(t, int64(1), user.Version.Int64)

	date := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	DB.NowFunc = func() time.Time {
		return date
	}
	rows := DB.Model(&user).Update("age", 18).RowsAffected
	require.Equal(t, int64(1), rows)
	require.Nil(t, DB.First(&user).Error)
	require.Equal(t, int64(2), user.Version.Int64)
	require.Equal(t, uint(18), user.Age)
	require.Equal(t, date, user.UpdatedAt)
	DB.NowFunc = time.Now

	rows = DB.Model(&user).Update("age", 16).RowsAffected
	require.Equal(t, int64(1), rows)
	require.Equal(t, uint(16), user.Age)

	rows = DB.Model(&user).Update("age", 14).RowsAffected
	require.Equal(t, int64(0), rows)

	rows = DB.Model(&User{}).Where("id = 1").Update("age", 12).RowsAffected
	require.Equal(t, int64(1), rows)
	require.Nil(t, DB.First(&user).Error)
	require.Equal(t, int64(4), user.Version.Int64)
	require.Equal(t, uint(12), user.Age)

	rows = DB.Model(&user).Select("name", "age", "version").Updates(&User{Name: "lewis"}).RowsAffected
	require.Equal(t, int64(1), rows)
	require.Nil(t, DB.First(&user).Error)
	require.Equal(t, int64(5), user.Version.Int64)
	require.Equal(t, "lewis", user.Name)
	require.Equal(t, uint(0), user.Age)

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

	sql = DB.Session(&gorm.Session{DryRun: true}).Model(&user).UpdateColumns(User{
		Age: 18,
	}).Statement.SQL.String()
	require.Contains(t, sql, "`version`=`version`+1")
	require.Contains(t, sql, "`version` = ?")

	sql = DB.Session(&gorm.Session{DryRun: true}).Model(&user).UpdateColumns(map[string]interface{}{
		"age": 18,
	}).Statement.SQL.String()
	require.Contains(t, sql, "`version`=`version`+1")
	require.Contains(t, sql, "`version` = ?")

	DB.NowFunc = func() time.Time {
		return date
	}
	user.Name = "micky"
	rows = DB.Updates(&user).RowsAffected
	require.Equal(t, int64(1), rows)
	require.Equal(t, date, user.UpdatedAt)
	DB.NowFunc = time.Now

	// support create
	users := []User{{Name: "foo", Age: 30}, {Name: "bar", Age: 40, Version: Version{Int64: 100}}}
	DB.Create(&users)
	require.Equal(t, "foo", users[0].Name)
	require.Equal(t, uint(30), users[0].Age)
	require.Equal(t, int64(1), users[0].Version.Int64)
	require.Equal(t, "bar", users[1].Name)
	require.Equal(t, uint(40), users[1].Age)
	require.Equal(t, int64(100), users[1].Version.Int64)
}

func TestJsonMarshal(t *testing.T) {
	user := User{
		Model: gorm.Model{
			ID: 1,
		},
		Name:    "lewis",
		Age:     18,
		Version: Version{Int64: 12, Valid: true},
	}
	b, err := json.Marshal(user)
	require.Nil(t, err)
	require.Equal(t, `{"ID":1,"CreatedAt":"0001-01-01T00:00:00Z","UpdatedAt":"0001-01-01T00:00:00Z","DeletedAt":null,"Name":"lewis","Age":18,"Version":12}`, string(b))

	user.Version.Valid = false
	b, err = json.Marshal(user)
	require.Nil(t, err)
	require.Equal(t, `{"ID":1,"CreatedAt":"0001-01-01T00:00:00Z","UpdatedAt":"0001-01-01T00:00:00Z","DeletedAt":null,"Name":"lewis","Age":18,"Version":null}`, string(b))
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
	require.Contains(t, sql, "`updated_at`=?")
	require.Contains(t, sql, "`version`=`version`+1")
	require.Contains(t, sql, "`user_id`=?")
	require.Contains(t, sql, "`ext`=?")
	require.Contains(t, sql, "`amount`=?")

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

// use gorm.io/gorm/tests docker compose file
func TestPostgres(t *testing.T) {
	DB, err := gorm.Open(postgres.Open(postgresDSN), &gorm.Config{})
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
}
