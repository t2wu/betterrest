package db

import (
	"sync"

	"github.com/jinzhu/gorm"
	// _ "github.com/jinzhu/gorm/dialects/sqlite"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

var once sync.Once
var db *gorm.DB

var sqlDbName = "BetterRESTDB"

// Shared is a singleton call
func Shared() *gorm.DB {
	// For thread safety
	// Does not allow repeating (FIXME: what if database goes down?)
	once.Do(func() {
		// Somehow I have to set _db first, otherwise return db will have nil
		// _db, err := gorm.Open("sqlite3", "./test.db")
		_db, err := gorm.Open("mysql", "root:12345678@/"+sqlDbName+"?charset=utf8mb4&parseTime=True&loc=Local")

		_db.LogMode(true)

		if err != nil {
			panic("failed to connect database:" + err.Error())
		}
		// defer db.Close()
		_db.SingularTable(true)
		db = _db
	})

	return db
}
