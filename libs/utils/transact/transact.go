package transact

import (
	"github.com/go-chi/render"
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/webrender"
)

// Transact wraps in a trasaction
// https://stackoverflow.com/questions/16184238/database-sql-tx-detecting-commit-or-rollback
func Transact(db *gorm.DB, txFunc func(*gorm.DB) error) (err error) {
	tx := db.Begin()
	if err = tx.Error; err != nil {
		return
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic after Rollback
		} else if err != nil {
			tx.Rollback() // err is non-nil; don't change it
		} else {
			err = tx.Commit().Error // err is nil; if Commit returns error update err
		}
	}()
	err = txFunc(tx)
	return err
}

// THIS IS WRONG Still. Don't use this yet, because having renderer doesn't necessarily mean it's an error
// Transact wraps in a trasaction
// https://stackoverflow.com/questions/16184238/database-sql-tx-detecting-commit-or-rollback
// Probably want to get rid of chi altogether. And use something more Gin-spaced and rethink this all over
func _TransactRenderer(db *gorm.DB, txFunc func(*gorm.DB) render.Renderer) (renderer render.Renderer) {
	var err error
	tx := db.Begin()
	if err = tx.Error; err != nil {
		return webrender.NewErrInternalServerError(err)
	}
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // re-throw panic after Rollback
		} else if renderer != nil {
			tx.Rollback() // err is non-nil; don't change it
		} else {
			err = tx.Commit().Error // err is nil; if Commit returns error update err
		}
	}()
	renderer = txFunc(tx)
	return renderer
}
