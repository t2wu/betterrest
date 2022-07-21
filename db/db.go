package db

import (
	"sync"

	"github.com/jinzhu/gorm"
	// _ "github.com/jinzhu/gorm/dialects/sqlite"
	// _ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

var once sync.Once
var db *gorm.DB

// SetUpDB set the db instance
func SetUpDB(_db *gorm.DB) {
	once.Do(func() {
		// if toLog == "true" {
		// 	_db.LogMode(true)
		// }

		// defer db.Close()
		_db.SingularTable(true)

		db = _db
	})
}

// Shared is a singleton call
func Shared() *gorm.DB {
	return db
}
