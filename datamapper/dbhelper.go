package datamapper

import (
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// How about AOP?
// https://github.com/gogap/aop

// createOneWithHooks handles before and after DB hookpoints for create
// func createOneWithHooks(createOneCore func(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel) (models.IModel, error), db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
// 	var err error
// 	var cargo models.ModelCargo

// 	if v, ok := modelObj.(models.IBeforeCreate); ok {
// 		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
// 		err = v.BeforeInsertDB(hpdata)
// 		if err != nil {
// 			return nil, err
// 		}
// 	}

// 	modelObj, err = createOneCore(db, oid, typeString, modelObj)
// 	if err != nil {
// 		return nil, err
// 	}

// 	if v, ok := modelObj.(models.IAfterCreate); ok {
// 		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
// 		err = v.AfterInsertDB(hpdata)
// 		if err != nil {
// 			return nil, err
// 		}
// 	}

// 	return modelObj, nil
// }

type batchOpJob struct {
	// mapper       IGetOneWithIDMapper
	serv         service.IService
	db           *gorm.DB
	oid          *datatypes.UUID
	scope        *string
	typeString   string
	oldmodelObjs []models.IModel // use for update (need to load and override for pegged fields)
	modelObjs    []models.IModel // current field value from the user if update, or from the loaded field if delete
}

func batchOpCore(job batchOpJob,
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error,
	taskFunc func(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (models.IModel, error),
) ([]models.IModel, error) {

	db, oid, scope, typeString, modelObjs, oldmodelObjs := job.db, job.oid, job.scope, job.typeString, job.modelObjs, job.oldmodelObjs

	ms := make([]models.IModel, len(modelObjs))
	cargo := models.BatchHookCargo{}

	// Before batch update hookpoint
	// if before := models.ModelRegistry[typeString].BeforeUpdate; before != nil {
	if before != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := before(bhpData); err != nil {
			return nil, err
		}
	}

	// TODO: Could update all at once, then load all at once again
	for i, modelObj := range modelObjs {
		id := modelObj.GetID()

		// m, err := updateOneCore(serv, db, oid, scope, typeString, modelObj, id)
		var m models.IModel
		var err error
		if oldmodelObjs == nil {
			m, err = taskFunc(db, oid, scope, typeString, modelObj, id, nil)
		} else {
			m, err = taskFunc(db, oid, scope, typeString, modelObj, id, oldmodelObjs[i])
		}
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}

		ms[i] = m
	}

	// After batch update hookpoint
	// if after := models.ModelRegistry[typeString].AfterUpdate; after != nil {
	if after != nil {
		bhpData := models.BatchHookPointData{Ms: ms, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := after(bhpData); err != nil {
			return nil, err
		}
	}

	return ms, nil
}

type opJob struct {
	// mapper      IGetOneWithIDMapper
	serv        service.IService
	db          *gorm.DB
	oid         *datatypes.UUID
	scope       *string
	typeString  string
	oldModelObj models.IModel // use for update (need to load and override for pegged fields)
	modelObj    models.IModel // current field value from the user if update, or from the loaded field if delete
}

func opCore(before func(hpdata models.HookPointData) error,
	after func(hpdata models.HookPointData) error,
	job opJob,
	taskFun func(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (models.IModel, error),
) (models.IModel, error) {
	db, oid, scope, typeString, oldModelObj, modelObj := job.db,
		job.oid, job.scope, job.typeString, job.oldModelObj, job.modelObj

	cargo := models.ModelCargo{}

	// Before hook
	// It is now expected that the hookpoint for before expect that the patch
	// gets applied to the JSON, but not before actually updating to DB.
	if before != nil {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := before(hpdata); err != nil {
			return nil, err
		}
	}

	// Now do the task
	id := modelObj.GetID()
	modelObj2, err := taskFun(db, oid, scope, typeString, modelObj, id, oldModelObj)
	if err != nil {
		return nil, err
	}

	// After hook
	if after != nil {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = after(hpdata); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}
