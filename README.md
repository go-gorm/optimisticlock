# Optimisticlock

Optimisticlock is optimistic lock plugin for gorm.

## Usage

```go
import "gorm.io/plugin/optimisticlock"

type User struct {
  ID      int
  Name    string
  Age     uint
  Version optimisticlock.Version
}

var user User
DB.First(&user)

DB.Model(&user).Update("age", 18)
// UPDATE `users` SET `age`=18,`version`=`version`+1 WHERE `users`.`version` = 1 AND `id` = 1

// Ignoring the optimistic lock check.
DB.Unscoped().Model(&user).Update("age", 18)
// UPDATE `users` SET `age`=18,`version`=`version`+1 WHERE `id` = 1

// Ignoring the passed Version value.
DB.Model(&user).Updates(&User{Age: 18, Version: optimisticlock.Version{Int64: 1}})
// UPDATE `users` SET `age`=18,`version`=`version`+1 WHERE `users`.`version` = 3 AND `id` = 1

// If the Model's Version value is zero, Without considering optimistic lock check.
DB.Model(&User{}).Where("id = 1").Update("age", 12)
// UPDATE `users` SET `age`=12,`version`=`version`+1 WHERE id = 1

// When want to use GORM's Save method, need to call Select. Otherwise, will return primary key duplicate error.
// The Select param is the fields that you want to update or "*".
DB.Model(&user).Select("*").Updates(&User{Age: 18})
```
