module gorm.io/plugin/optimisticlock/tests

go 1.18

require (
	gorm.io/driver/sqlite v1.5.6
	gorm.io/gorm v1.25.11
	gorm.io/plugin/optimisticlock v1.1.3
)

require (
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace gorm.io/plugin/optimisticlock => ../
