package datamapper

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/security"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

var onceUser sync.Once
var usercrud *UserMapper

// UserMapper is a User CRUD manager
type UserMapper struct {
}

// SharedUserMapper creats a singleton of Crud object
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
func (mapper *UserMapper) CreateOne(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel) (models.IModel, error) {
	// Special case, there is really no oid in this case, user doesn't exist yet

	if userObj, ok := modelObj.(*models.User); ok { // it has to be OK...
		userObj.Ownerships = make([]models.Ownership, 1)
		g := models.Ownership{}
		g.Role = models.Admin
		userObj.Ownerships[0] = g

		// Need to encrypt the password
		if userObj.Password != "" {
			hash, err := security.HashAndSalt(userObj.Password)
			if err != nil {
				return nil, err
			}

			userObj.PasswordHash = hash
		}

		if dbc := db.Create(modelObj); dbc.Error != nil {
			// create failed: UNIQUE constraint failed: user.email
			// It looks like this error may be dependent on the type of database we use
			log.Println("create failed:", dbc.Error)
			return nil, dbc.Error
		}

		return userObj, nil
	}

	return nil, errors.New("User model expected when creating a user")
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *UserMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, id datatypes.UUID) (models.IModel, error) {
	// TODO: Currently can only read ID from your own (not others in the admin group either)
	db = db.Set("gorm:auto_preload", true)

	if id.String() != oid.String() {
		return nil, errPermission
	}

	modelObj := models.NewFromTypeString(typeString)
	modelObj.SetID(oid)

	if err := db.First(modelObj).Error; err != nil {
		return nil, err
	}

	return modelObj, nil
}

// UpdateOneWithID updates model based on this json
func (mapper *UserMapper) UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel, id datatypes.UUID) (models.IModel, error) {
	log.Println("userMapper's UpdateOneWithID called")
	if err := checkErrorBeforeUpdate(mapper, db, oid, typeString, modelObj, id); err != nil {
		return nil, err
	}

	userObj := modelObj.(*models.User) // has to be OK otherwise a programming errr

	// Additional checking because password should not be blank with update
	if userObj.Password == "" {
		log.Println("password should not be blank!!!")
		return nil, fmt.Errorf("password should not be blank")
	}

	hash, err := security.HashAndSalt(userObj.Password)
	if err != nil {
		return nil, err
	}
	userObj.PasswordHash = hash

	cargo := models.ModelCargo{}

	// Before hook
	if v, ok := modelObj.(models.IBeforeUpdate); ok {
		if err := v.BeforeUpdateDB(db, oid, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	modelObj2, err := updateOneCore(mapper, db, oid, typeString, modelObj, id)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterUpdate); ok {
		if err = v.AfterUpdateDB(db, oid, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}

// DeleteOneWithID deletes the user with the ID
func (mapper *UserMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, id datatypes.UUID) (models.IModel, error) {
	if id.UUID.String() == "" {
		return nil, errIDEmpty
	}

	// Pull out entire modelObj
	modelObj, err := mapper.GetOneWithID(db, oid, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return nil, err
	}

	cargo := models.ModelCargo{}

	// Before delete hookpoint
	if v, ok := modelObj.(models.IBeforeDelete); ok {
		err = v.BeforeDeleteDB(db, oid, typeString, &cargo)
		if err != nil {
			return nil, err
		}
	}

	// Unscoped() for REAL delete!
	// Otherwise my constraint won't work...
	// Soft delete will take more work, have to verify myself manually
	// db.Unscoped().Delete(modelObj).Error
	err = db.Delete(modelObj).Error
	if err != nil {
		return nil, err
	}

	// After delete hookpoint
	if v, ok := modelObj.(models.IAfterDelete); ok {
		err = v.AfterDeleteDB(db, oid, typeString, &cargo)
		if err != nil {
			return nil, err
		}
	}

	return modelObj, nil
}
