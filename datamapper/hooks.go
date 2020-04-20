package datamapper

import (
	"log"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

// How about AOP?
// https://github.com/gogap/aop

// CreateWithHooks handles before and after DB hookpoints for create
func CreateWithHooks(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel) (models.IModel, error) {
	var err error
	var cargo models.ModelCargo

	if v, ok := modelObj.(models.IBeforeInsert); ok {
		err = v.BeforeInsertDB(db, oid, typeString, &cargo)
		if err != nil {
			return nil, err
		}
	}

	// No need to check if primary key is blank.
	// If it is it'll be created by Gorm's BeforeCreate hook
	// (defined in base model)
	if dbc := db.Create(modelObj); dbc.Error != nil {
		// create failed: UNIQUE constraint failed: user.email
		// It looks like this error may be dependent on the type of database we use
		return nil, dbc.Error
	}

	// For table with trigger which update before insert, we need to load it again
	if err = db.First(modelObj).Error; err != nil {
		// That's weird. we just inserted it.
		return nil, err
	}

	if v, ok := modelObj.(models.IAfterInsert); ok {
		err = v.AfterInsertDB(db, oid, typeString, &cargo)
		if err != nil {
			return nil, err
		}
	}

	log.Println("modelObj:", modelObj)

	return modelObj, nil
}
