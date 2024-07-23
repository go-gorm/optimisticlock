package optimisticlock

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model

	Name    string
	Age     uint
	Version Version
}

func TestVersion(t *testing.T) {
	DB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	user := User{Name: "bob", Age: 20}
	_ = DB.Migrator().DropTable(&User{})
	_ = DB.AutoMigrate(&User{})
	DB.Save(&user)

	date := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	DB.NowFunc = func() time.Time {
		return date
	}

	rows := DB.Model(&user).Update("age", 18).RowsAffected
	if rows != int64(1) {
		t.Fatalf("expect RowsAffected 1, got %d", rows)
	}

	DB.First(&user)
	if user.Version.Int64 != int64(2) || user.Age != uint(18) || user.UpdatedAt != date {
		t.Fatalf("data does not match expectations, got %v", user)
	}

	DB.NowFunc = time.Now

	rows = DB.Model(&user).Update("age", 16).RowsAffected
	if rows != int64(1) || user.Age != uint(16) {
		t.Fatalf("expect RowsAffected 1 and age 16, got RowsAffected %d and age %d", rows, user.Age)
	}

	// user version value not equal to database value
	rows = DB.Model(&user).Update("age", 14).RowsAffected
	if rows != int64(0) {
		t.Fatalf("expect RowsAffected 0, got %d", rows)
	}

	rows = DB.Model(&User{}).Where("id = 1").Update("age", 12).RowsAffected
	if rows != int64(1) {
		t.Fatalf("expect RowsAffected 1, got %d", rows)
	}

	DB.First(&user)
	if user.Version.Int64 != int64(4) || user.Age != uint(12) {
		t.Fatalf("expect version 4 and age 12, got version %v and age %d", user.Version.Int64, user.Age)
	}

	DB.Model(&user).Select("name", "age", "version").Updates(&User{Name: "lewis"})
	DB.First(&user)
	if user.Version.Int64 != int64(5) || user.Name != "lewis" || user.Age != uint(0) {
		t.Fatalf("expect version 5, name lewis and age 0, got version %d, name %s and age %d", user.Version.Int64, user.Name, user.Age)
	}

	if rows = DB.Model(&user).Updates(map[string]interface{}{
		"age": 18,
	}).RowsAffected; rows != int64(1) {
		t.Fatalf("expect RowsAffected == 1, got %d", rows)
	}

	DB.First(&user)
	if user.Version.Int64 != int64(6) || user.Age != uint(18) {
		t.Fatalf("expect version 6 and age 18, got version %d and age %d", user.Version.Int64, user.Age)
	}

	sql := DB.Session(&gorm.Session{DryRun: true}).Model(&user).Updates(nil).Statement.SQL.String()
	if !strings.Contains(sql, "`version`=`version`+1") {
		t.Fatalf("invalid sql generated, got %v", sql)
	}

	sql = DB.Session(&gorm.Session{DryRun: true}).Model(&user).UpdateColumns(User{
		Age: 18,
	}).Statement.SQL.String()
	if !strings.Contains(sql, "`version`=`version`+1") || !strings.Contains(sql, "`version` = ?") {
		t.Fatalf("invalid sql generated, got %v", sql)
	}

	sql = DB.Session(&gorm.Session{DryRun: true}).Model(&user).UpdateColumns(map[string]interface{}{
		"age": 18,
	}).Statement.SQL.String()
	if !strings.Contains(sql, "`version`=`version`+1") || !strings.Contains(sql, "`version` = ?") {
		t.Fatalf("invalid sql generated, got %v", sql)
	}

	sql = DB.Session(&gorm.Session{DryRun: true}).Model(&user).Updates(&User{
		Age:     19,
		Version: Version{Int64: 10000000},
	}).Statement.SQL.String()
	if !strings.Contains(sql, "`version`=`version`+1") {
		t.Fatalf("invalid sql generated, got %v", sql)
	}

	DB.NowFunc = func() time.Time {
		return date
	}
	user.Name = "micky"
	rows = DB.Updates(&user).RowsAffected
	if rows != int64(1) || user.UpdatedAt != date {
		t.Fatalf("expect RowsAffected 1 and date %s, got %d and %s", date, rows, user.UpdatedAt)
	}
	DB.NowFunc = time.Now

	// Call Select method. Otherwise, will return primary key duplicate error.
	user.Name = "lucky"
	tx := DB.Select("*").Save(&user)
	if tx.Error != nil || tx.RowsAffected != int64(0) {
		t.Fatalf("expect nil error and RowsAffected 0, got error %v and RowsAffected %d", tx.Error, tx.RowsAffected)
	}

	// support create
	users := []User{{Name: "foo", Age: 30}, {Name: "bar", Age: 40, Version: Version{Int64: 100}}}
	DB.Create(&users)
	user0 := users[0]
	user1 := users[1]
	if user0.Name != "foo" || user0.Age != uint(30) || user0.Version.Int64 != int64(1) ||
		user1.Name != "bar" || user1.Age != uint(40) || user1.Version.Int64 != int64(100) {
		t.Fatalf("batch create failed, got %v", users)
	}

	// unscoped
	user.Name = "foo"
	r := DB.Unscoped().Updates(&user)
	if r.RowsAffected != int64(1) {
		t.Fatalf("unscoped update failed")
	}
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
	if err != nil {
		t.Fatalf("failed to marshal json: %v", err)
	}
	if string(b) != `{"ID":1,"CreatedAt":"0001-01-01T00:00:00Z","UpdatedAt":"0001-01-01T00:00:00Z","DeletedAt":null,"Name":"lewis","Age":18,"Version":12}` {
		t.Fatalf("failed to marshal json, got %s", string(b))
	}

	user.Version.Valid = false
	b, err = json.Marshal(user)
	if err != nil {
		t.Fatalf("failed to marshal json: %v", err)
	}
	if string(b) != `{"ID":1,"CreatedAt":"0001-01-01T00:00:00Z","UpdatedAt":"0001-01-01T00:00:00Z","DeletedAt":null,"Name":"lewis","Age":18,"Version":null}` {
		t.Fatalf("failed to marshal json, got %s", string(b))
	}
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
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	user := User{Name: "bob", Age: 20}
	_ = DB.Migrator().DropTable(&User{})
	_ = DB.AutoMigrate(&User{})
	DB.Save(&user)

	account := Account{
		UserID: 1,
		User:   &user,
		Amount: 1000,
		Ext:    Ext{CreditCard: []string{"123456", "456123"}},
	}
	_ = DB.Migrator().DropTable(&Account{})
	_ = DB.AutoMigrate(&Account{})
	if err = DB.Save(&account).Error; err != nil {
		t.Fatalf("failed to save account: %v", err)
	}

	sql := DB.Session(&gorm.Session{DryRun: true}).Updates(&account).Statement.SQL.String()
	if !strings.Contains(sql, "`updated_at`=?") ||
		!strings.Contains(sql, "`version`=`version`+1") ||
		!strings.Contains(sql, "`user_id`=?") ||
		!strings.Contains(sql, "`ext`=?") ||
		!strings.Contains(sql, "`amount`=?") {
		t.Fatalf("invalid sql generated, got %v", sql)
	}

	var a Account
	DB.First(&a)

	a.Amount = 233
	rows := DB.Updates(&a).RowsAffected
	if rows != int64(1) {
		t.Fatalf("expect RowsAffected 1, got %d", rows)
	}

	a.Amount = 556
	rows = DB.Updates(&a).RowsAffected
	if rows != int64(0) {
		t.Fatalf("expect RowsAffected 0, got %d", rows)
	}
	DB.First(&a)
	if a.Amount != 233 {
		t.Fatalf("expect 233, got %d", a.Amount)
	}

	a.Amount = 9999
	if rows = DB.Updates(&a).RowsAffected; rows != int64(1) {
		t.Fatalf("expect RowsAffected 1, got %d", rows)
	}

	var a1 Account
	DB.First(&a1)
	if a.Amount != a1.Amount || a.Version.Int64 != int64(2) || a1.Version.Int64 != int64(3) {
		t.Fatalf("got a.Amount %v, a1.Amount %v, a.Version %v, a1.Version %v", a.Amount, a1.Amount,
			a.Version.Int64, a1.Version.Int64)
	}
}
