package datamapper

import (
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/models"
)

// How about AOP?
// https://github.com/gogap/aop

type batchOpJob struct {
	// mapper       IGetOneWithIDMapper
	serv         service.IService
	db           *gorm.DB
	who          models.Who
	typeString   string
	oldmodelObjs []models.IModel // use for update (need to load and override for pegged fields)
	modelObjs    []models.IModel // current field value from the user if update, or from the loaded field if delete
	cargo        *models.BatchHookCargo
	crupdOp      models.CRUPDOp
	options      *map[urlparam.Param]interface{}
}

func batchOpCore(job batchOpJob,
	before func(bhpData models.BatchHookPointData) error,
	after func(bhpData models.BatchHookPointData) error,
	taskFunc func(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (models.IModel, error),
) ([]models.IModel, error) {

	db, who, typeString, modelObjs, oldmodelObjs, cargo, crupdOp, options := job.db, job.who, job.typeString,
		job.modelObjs, job.oldmodelObjs, job.cargo, job.crupdOp, job.options

	ms := make([]models.IModel, len(modelObjs))

	if cargo == nil {
		cargo = &models.BatchHookCargo{}
	}

	// After CUPD hook
	if before := models.ModelRegistry[typeString].BeforeCUPD; before != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, Who: who, TypeString: typeString, Cargo: cargo, URLParams: options}
		if err := before(bhpData, crupdOp); err != nil {
			return nil, err
		}
	}

	// Before batch update hookpoint
	// if before := models.ModelRegistry[typeString].BeforeUpdate; before != nil {
	if before != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, Who: who, TypeString: typeString, Cargo: cargo, URLParams: options}
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
			m, err = taskFunc(db, who, typeString, modelObj, id, nil)
		} else {
			m, err = taskFunc(db, who, typeString, modelObj, id, oldmodelObjs[i])
		}
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}

		ms[i] = m
	}

	// After CRUPD hook
	if after := models.ModelRegistry[typeString].AfterCRUPD; after != nil {
		bhpData := models.BatchHookPointData{Ms: ms, DB: db, Who: who, TypeString: typeString, Cargo: cargo, URLParams: options}
		if err := after(bhpData, crupdOp); err != nil {
			return nil, err
		}
	}

	// After batch update hookpoint
	// if after := models.ModelRegistry[typeString].AfterUpdate; after != nil {
	if after != nil {
		bhpData := models.BatchHookPointData{Ms: ms, DB: db, Who: who, TypeString: typeString, Cargo: cargo, URLParams: options}
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
	who         models.Who
	typeString  string
	crupdOp     models.CRUPDOp
	oldModelObj models.IModel      // use for update (need to load and override for pegged fields)
	modelObj    models.IModel      // current field value from the user if update, or from the loaded field if delete
	cargo       *models.ModelCargo // This only is used because we may have an even earlier hookpoint for PatchApply
	options     *map[urlparam.Param]interface{}
}

// before and after are strings, because once we load it from the DB the hooks
// should be the new one. (at least for after)
func opCore(
	beforeFuncName *string,
	afterFuncName *string,
	// before func(hpdata models.HookPointData) error,
	// after func(hpdata models.HookPointData) error,
	job opJob,
	taskFun func(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (models.IModel, error),
) (models.IModel, error) {
	db, who, typeString, oldModelObj, modelObj, crupdOp, cargo, options := job.db, job.who,
		job.typeString, job.oldModelObj, job.modelObj, job.crupdOp, job.cargo, job.options

	if cargo == nil {
		cargo = &models.ModelCargo{}
	}

	// Before CRUPD hook
	if m, ok := modelObj.(models.IBeforeCUPD); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: cargo, URLParams: options}
		if err := m.BeforeCUPDDB(hpdata, crupdOp); err != nil {
			return nil, err
		}
	}

	// Before hook
	// It is now expected that the hookpoint for before expect that the patch
	// gets applied to the JSON, but not before actually updating to DB.
	if beforeFuncName != nil {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: cargo, URLParams: options}
		result := reflect.ValueOf(modelObj).MethodByName(*beforeFuncName).Call([]reflect.Value{reflect.ValueOf(hpdata)})
		if err, ok := result[0].Interface().(error); ok {
			return nil, err
		}
	}

	// Now do the task
	id := modelObj.GetID()
	modelObjReloaded, err := taskFun(db, who, typeString, modelObj, id, oldModelObj)
	if err != nil {
		return nil, err
	}

	// After CRUPD hook
	if m, ok := modelObjReloaded.(models.IAfterCRUPD); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: cargo, URLParams: options}
		if err := m.AfterCRUPDDB(hpdata, crupdOp); err != nil {
			return nil, err
		}
		modelObjReloaded = m.(models.IModel)
	}

	// After hook
	if afterFuncName != nil {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: cargo, URLParams: options}
		result := reflect.ValueOf(modelObjReloaded).MethodByName(*afterFuncName).Call([]reflect.Value{reflect.ValueOf(hpdata)})
		if err, ok := result[0].Interface().(error); ok {
			return nil, err
		}
	}

	return modelObjReloaded, nil
}
