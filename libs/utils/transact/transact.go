package transact

import (
	"log"
	"sync"
	"time"

	"github.com/go-chi/render"
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/settings"
	"github.com/t2wu/betterrest/libs/webrender"
)

var transactDebugCount = 0
var transactDebugCountLock *sync.RWMutex = &sync.RWMutex{}

// Transact wraps in a trasaction
// https://stackoverflow.com/questions/16184238/database-sql-tx-detecting-commit-or-rollback
func Transact(db *gorm.DB, txFunc func(*gorm.DB) error, labels ...string) (err error) {
	debug := false
	if len(labels) != 0 && settings.TransactDebug {
		debug = true
	}

	var now time.Time
	var transactID string
	var label string
	if debug {
		if len(labels) > 0 {
			label = labels[0]
		}

		transactID = datatypes.NewUUID().String()
		if len(labels) > 1 {
			transactID = labels[1]
		}

		now = time.Now()

		transactDebugCountLock.Lock()
		transactDebugCount += 1
		log.Printf("Transact ID %s %s begin to begin, existing transactions: %d\n", transactID, label, transactDebugCount)
		transactDebugCountLock.Unlock()
	}

	tx := db.Begin()
	if err = tx.Error; err != nil {
		return
	}

	defer func() {
		if p := recover(); p != nil {
			if debug {
				transactDebugCountLock.Lock()
				transactDebugCount -= 1
				log.Printf("Transact ID %s %s end (rollback panic), existing transactions: %d, takes %f second\n", transactID, label, transactDebugCount, float64(time.Now().Sub(now))/float64(time.Second))
				transactDebugCountLock.Unlock()
			}

			tx.Rollback()
			panic(p) // re-throw panic after Rollback
		} else if err != nil {
			if debug {
				transactDebugCountLock.Lock()
				transactDebugCount -= 1
				log.Printf("Transact ID %s %s end (rollback), existing transactions: %d, takes %f second\n", transactID, label, transactDebugCount, float64(time.Now().Sub(now))/float64(time.Second))
				transactDebugCountLock.Unlock()
			}

			tx.Rollback() // err is non-nil; don't change it
		} else {
			err = tx.Commit().Error // err is nil; if Commit returns error update err

			if debug {
				transactDebugCountLock.Lock()
				transactDebugCount -= 1
				log.Printf("Transact ID %s %s end (commit), existing transactions: %d, takes %f second\n", transactID, label, transactDebugCount, float64(time.Now().Sub(now))/float64(time.Second))
				transactDebugCountLock.Unlock()
			}
		}
	}()

	if debug {
		log.Printf("Transact ID %s %s begin exec\n", transactID, label)
	}

	err = txFunc(tx)
	return err
}

func TransactCustomError(db *gorm.DB, txFunc func(*gorm.DB) *webrender.RetVal, labels ...string) (retval *webrender.RetVal) {
	debug := false
	if len(labels) != 0 && settings.TransactDebug {
		debug = true
	}

	var now time.Time
	var transactID string
	var label string
	if debug {
		if len(labels) > 0 {
			label = labels[0]
		}

		transactID = datatypes.NewUUID().String()
		if len(labels) > 1 {
			transactID = labels[1]
		}

		now = time.Now()

		transactDebugCountLock.Lock()
		transactDebugCount += 1
		log.Printf("Transact ID %s %s begin to begin, existing transactions: %d\n", transactID, label, transactDebugCount)
		transactDebugCountLock.Unlock()
	}

	tx := db.Begin()
	if err := tx.Error; err != nil {
		renderer := webrender.NewErrInternalServerError(err)
		retval = &webrender.RetVal{CustomRenderer: &renderer}
	}

	defer func() {
		if p := recover(); p != nil {
			if debug {
				transactDebugCountLock.Lock()
				transactDebugCount -= 1
				log.Printf("Transact ID %s %s end (rollback panic), existing transactions: %d, takes %f second\n", transactID, label, transactDebugCount, float64(time.Now().Sub(now))/float64(time.Second))
				transactDebugCountLock.Unlock()
			}

			tx.Rollback()
			panic(p) // re-throw panic after Rollback
		} else if retval != nil {
			if debug {
				transactDebugCountLock.Lock()
				transactDebugCount -= 1
				log.Printf("Transact ID %s %s end (rollback), existing transactions: %d, takes %f second\n", transactID, label, transactDebugCount, float64(time.Now().Sub(now))/float64(time.Second))
				transactDebugCountLock.Unlock()
			}

			tx.Rollback() // err is non-nil; don't change it
		} else {
			if err := tx.Commit().Error; err != nil { // err is nil; if Commit returns error update err
				renderer := webrender.NewErrInternalServerError(err)
				retval = &webrender.RetVal{CustomRenderer: &renderer}
			}

			if debug {
				transactDebugCountLock.Lock()
				transactDebugCount -= 1
				log.Printf("Transact ID %s %s end (commit), existing transactions: %d, takes %f second\n", transactID, label, transactDebugCount, float64(time.Now().Sub(now))/float64(time.Second))
				transactDebugCountLock.Unlock()
			}
		}
	}()

	if debug {
		log.Printf("Transact ID %s %s begin exec\n", transactID, label)
	}

	retval = txFunc(tx)
	return retval
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
