package datamapper

import (
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

// How about AOP?
// https://github.com/gogap/aop

// createOneWithHooks handles before and after DB hookpoints for create
func createOneWithHooks(createOneCore func(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel) (models.IModel, error), db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	var err error
	var cargo models.ModelCargo

	if v, ok := modelObj.(models.IBeforeInsert); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		err = v.BeforeInsertDB(hpdata)
		if err != nil {
			return nil, err
		}
	}

	modelObj, err = createOneCore(db, oid, typeString, modelObj)
	if err != nil {
		return nil, err
	}

	if v, ok := modelObj.(models.IAfterInsert); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		err = v.AfterInsertDB(hpdata)
		if err != nil {
			return nil, err
		}
	}

	return modelObj, nil
}
