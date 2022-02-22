# optimisticlock
gorm-optimisticlock is optimistic lock plugin for gorm.

## require
- gorm ≤ v1.22.4

```go
import "github.com/whoisix/gorm-optimisticlock"

type User struct {
    ID      int
    Name    string
    Age     uint
    Version optimisticlock.Version
}

var user User
DB.First(&user)

DB.Model(&user).Update("age", 18)
UPDATE `users` SET `age`=18,`version`=`version`+1 WHERE `users`.`version` = 1 AND `id` = 1

DB.Model(&user).Updates(&User{Age: 18, Version: optimisticlock.Version{Int64: 1}})
UPDATE `users` SET `age`=18,`version`=`version`+1 WHERE `users`.`version` = 2 AND `id` = 1

DB.Model(&User{}).Where("id = 1").Update("age", 12)
UPDATE `users` SET `age`=12,`version`=`version`+1 WHERE id = 1
```
