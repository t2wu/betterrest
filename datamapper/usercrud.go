package datamapper

import (
	"errors"
	"log"
	"sync"

	"betterrest/models"

	"github.com/jinzhu/gorm"
)

var onceUser sync.Once
var usercrud *UserMapper

// UserMapper is a User CRUD manager
type UserMapper struct {
}

// SharedUserCrud creats a singleton of Crud object
func SharedUserMapper() *UserMapper {
	onceUser.Do(func() {
		usercrud = &UserMapper{}
	})

	return usercrud
}

//------------------------
// User specific CRUD
// Cuz user is spcial, need to create ownership and no need to check for owner
// ------------------------------------

// CreateOne creates an user model based on json and store it in db
// Also creates a ownership with admin access
func (c *UserMapper) CreateOne(db *gorm.DB, oid uint, typeString string, modelObj models.IModel) (models.IModel, error) {
	// Special case, there is really no oid in this case, user doesn't exist yet

	// basicMapper := SharedBasicMapper()

	// FIXME: better be in a transaction
	// http://gorm.io/docs/transactions.html
	// Better have transaction, but transaction requires
	// keeping the tx as the database handle. And currently
	// that isn't injected
	// // tx := db.Begin()
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		tx.Rollback()
	// 	}
	// }()

	if userObj, ok := modelObj.(*models.User); ok { // it has to be OK...
		userObj.Ownerships = make([]models.Ownership, 1)
		g := models.Ownership{}
		g.Role = models.Admin
		userObj.Ownerships[0] = g

		if db.NewRecord(modelObj) { // FIXME: new record is not what I think. it will still return
			if dbc := db.Create(modelObj); dbc.Error != nil { // FIXME: Create doesn't return error, why?
				// create failed: UNIQUE constraint failed: user.email
				// It looks like this error may be dependent on the type of database we use
				log.Println("create failed:", dbc.Error)
				return nil, dbc.Error
			}

			return userObj, nil
		} else {
			return nil, errors.New("record exists")
		}
	} else {
		return nil, errors.New("User model expected when creating a user")
	}
}
