package db

import (
	"os"
	"sync"

	"github.com/jinzhu/gorm"
	// _ "github.com/jinzhu/gorm/dialects/sqlite"
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

var once sync.Once
var db *gorm.DB

var sqlDbName = os.Getenv("BETTER_REST_DB")

// Shared is a singleton call
func Shared() *gorm.DB {
	// For thread safety
	// Does not allow repeating (FIXME: what if database goes down?)
	once.Do(func() {
		username := os.Getenv("BETTER_REST_DB_USER")
		passwd := os.Getenv("BETTER_REST_DB_PASSWD")

		// Somehow I have to set _db first, otherwise return db will have nil
		// _db, err := gorm.Open("sqlite3", "./test.db")
		_db, err := gorm.Open("mysql", username+":"+passwd+"@/"+sqlDbName+"?charset=utf8mb4&parseTime=True&loc=Local")
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
