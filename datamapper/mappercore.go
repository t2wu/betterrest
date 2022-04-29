package datamapper

import (
	"fmt"
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/controller"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry"
)

func callOldBatch(
	data *controller.Data,
	info *controller.EndPointInfo,
	oldGeneric func(bhpData models.BatchHookPointData, op models.CRUPDOp) error, // before or after
	oldSpecific func(bhpData models.BatchHookPointData) error, // before or after
) error {
	oldBatchCargo := models.BatchHookCargo{Payload: data.Cargo.Payload}
	bhpData := models.BatchHookPointData{Ms: data.Ms, DB: data.DB, Who: data.Who,
		TypeString: data.TypeString, Roles: data.Roles, URLParams: data.URLParams, Cargo: &oldBatchCargo}

	var op models.CRUPDOp
	switch info.Op {
	case controller.RESTOpRead:
		op = models.CRUPDOpRead
	case controller.RESTOpCreate:
		op = models.CRUPDOpCreate
	case controller.RESTOpUpdate:
		op = models.CRUPDOpUpdate
	case controller.RESTOpPatch:
		op = models.CRUPDOpPatch
	case controller.RESTOpDelete:
		op = models.CRUPDOpDelete
	}

	// After CUPD hook
	if oldGeneric != nil {
		if err := oldGeneric(bhpData, op); err != nil {
			return err
		}
	}

	// Before batch update hookpoint
	// if before := registry.ModelRegistry[typeString].BeforeUpdate; before != nil {
	if oldSpecific != nil {
		if err := oldSpecific(bhpData); err != nil {
			return err
		}
	}
	data.Cargo.Payload = bhpData.Cargo.Payload

	return nil
}

// old specific (before and after) is in string, because once we load it from the DB the hooks
// should be the new one. (at least for after)
func callOldSingle(
	data *controller.Data,
	info *controller.EndPointInfo,
	oldGeneric func(hpdata models.HookPointData, op models.CRUPDOp) error,
	oldSpecific *string, // before or after
) error {
	oldSingleCargo := models.ModelCargo{Payload: data.Cargo.Payload}
	hpdata := models.HookPointData{DB: data.DB, Who: data.Who, TypeString: data.TypeString,
		URLParams: data.URLParams, Role: &data.Roles[0], Cargo: &oldSingleCargo}

	var op models.CRUPDOp
	switch info.Op {
	case controller.RESTOpRead:
		op = models.CRUPDOpRead
	case controller.RESTOpCreate:
		op = models.CRUPDOpCreate
	case controller.RESTOpUpdate:
		op = models.CRUPDOpUpdate
	case controller.RESTOpPatch:
		op = models.CRUPDOpPatch
	case controller.RESTOpDelete:
		op = models.CRUPDOpDelete
	}

	// Before CRUPD hook
	if oldGeneric != nil {
		if err := oldGeneric(hpdata, op); err != nil {
			return err
		}
	}

	// Before hook
	// It is now expected that the hookpoint for before expect that the patch
	// gets applied to the JSON, but not before actually updating to DB.
	if oldSpecific != nil {
		result := reflect.ValueOf(data.Ms[0]).MethodByName(*oldSpecific).Call([]reflect.Value{reflect.ValueOf(hpdata)})
		if err, ok := result[0].Interface().(error); ok {
			return err
		}
	}

	data.Cargo.Payload = hpdata.Cargo.Payload

	return nil
}

// How about AOP?
// https://github.com/gogap/aop

type batchOpJob struct {
	serv service.IService
	// db           *gorm.DB
	// who          models.UserIDFetchable
	// typeString   string
	oldmodelObjs []models.IModel // use for update (need to load and override for pegged fields)
	modelObjs    []models.IModel // current field value from the user if update, or from the loaded field if delete
	// cargo        *controller.Cargo

	// crupdOp      models.CRUPDOp
	// options      map[urlparam.Param]interface{}

	oldBefore func(bhpData models.BatchHookPointData) error
	oldAfter  func(bhpData models.BatchHookPointData) error

	fetcher *CtrlFetcher
	data    *controller.Data
	info    *controller.EndPointInfo
}

func batchOpCore(job batchOpJob,
	taskFunc func(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (models.IModel, error),
) (*MapperRet, *webrender.RetError) {
	modelObjs, oldmodelObjs, oldBefore, oldAfter := job.modelObjs, job.oldmodelObjs, job.oldBefore, job.oldAfter
	fetcher, data, info := job.fetcher, job.data, job.info

	ms := make([]models.IModel, len(modelObjs))

	if data.Cargo == nil {
		return nil, &webrender.RetError{Error: fmt.Errorf("cargo shouldn't be nil")}
	}

	// deprecated, only try to call when no controlelr exists
	if !fetcher.HasRegisteredController() {
		if err := callOldBatch(data, info, registry.ModelRegistry[data.TypeString].BeforeCUPD, oldBefore); err != nil {
			return nil, &webrender.RetError{Error: err}
		}
	}

	// fetch all controllers with before hooks
	for _, ctrl := range fetcher.FetchControllersForOpAndHook(info.Op, "B") {
		if renderer := ctrl.(controller.IBefore).Before(data, info); renderer != nil {
			return nil, renderer
		}
	}

	// TODO: Could update all at once, then load all at once again
	for i, modelObj := range modelObjs {
		id := modelObj.GetID()

		// m, err := updateOneCore(serv, db, oid, scope, typeString, modelObj, id)
		var m models.IModel
		var err error
		if oldmodelObjs == nil {
			m, err = taskFunc(data.DB, data.Who, data.TypeString, modelObj, id, nil)
		} else {
			m, err = taskFunc(data.DB, data.Who, data.TypeString, modelObj, id, oldmodelObjs[i])
		}
		if err != nil { // Error is "record not found" when not found
			return nil, &webrender.RetError{Error: err}
		}

		ms[i] = m
	}

	if !fetcher.HasRegisteredController() { // deprecated, only try to call when no controlelr exists
		if err := callOldBatch(data, info, registry.ModelRegistry[data.TypeString].AfterCRUPD, oldAfter); err != nil {
			return nil, &webrender.RetError{Error: err}
		}
	}

	// fetch all controllers with after hooks
	for _, ctrl := range fetcher.FetchControllersForOpAndHook(info.Op, "A") {
		if renderer := ctrl.(controller.IAfter).After(data, info); renderer != nil {
			return nil, renderer
		}
	}

	return &MapperRet{
		Ms:      ms,
		Fetcher: fetcher,
	}, nil
}

type opJob struct {
	serv service.IService
	// db          *gorm.DB
	// who         models.UserIDFetchable
	// typeString  string
	// crupdOp     models.CRUPDOp
	oldModelObj models.IModel // use for update (need to load and override for pegged fields)
	modelObj    models.IModel // current field value from the user if update, or from the loaded field if delete
	// cargo       *controller.Cargo // This only is used because we may have an even earlier hookpoint for PatchApply
	// options     map[urlparam.Param]interface{}

	// before and after are strings, because once we load it from the DB the hooks
	// should be the new one. (at least for after)
	beforeFuncName *string
	afterFuncName  *string

	fetcher *CtrlFetcher
	data    *controller.Data
	info    *controller.EndPointInfo
}

func opCore(
	job opJob,
	taskFun func(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (models.IModel, error),
) (*MapperRet, *webrender.RetError) {
	oldModelObj, modelObj, beforeFuncName, afterFuncName := job.oldModelObj, job.modelObj, job.beforeFuncName, job.afterFuncName
	fetcher, data, info := job.fetcher, job.data, job.info

	if data.Cargo == nil {
		return nil, &webrender.RetError{Error: fmt.Errorf("cargo shouldn't be nil")}
	}

	// Deprecated
	if !fetcher.HasRegisteredController() { // deprecated, only try to call when no controller exists
		if m, ok := data.Ms[0].(models.IBeforeCUPD); ok {
			if err := callOldSingle(data, info, m.BeforeCUPDDB, beforeFuncName); err != nil {
				return nil, &webrender.RetError{Error: err}
			}
		}
	}
	// End deprecated

	// fetch all controllers with before hooks
	for _, ctrl := range fetcher.FetchControllersForOpAndHook(info.Op, "B") {
		if renderer := ctrl.(controller.IBefore).Before(data, info); renderer != nil {
			return nil, renderer
		}
	}

	// Now do the task
	id := modelObj.GetID()
	modelObjReloaded, err := taskFun(data.DB, data.Who, data.TypeString, modelObj, id, oldModelObj)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	data.Ms[0] = modelObjReloaded

	// Deprecated
	if !fetcher.HasRegisteredController() { // deprecated, only try to call when no controlelr exists
		if m, ok := data.Ms[0].(models.IAfterCRUPD); ok {
			if err := callOldSingle(data, info, m.AfterCRUPDDB, afterFuncName); err != nil {
				return nil, &webrender.RetError{Error: err}
			}
		}
	}
	// End deprecated

	// fetch all controllers with after hooks
	for _, ctrl := range fetcher.FetchControllersForOpAndHook(info.Op, "A") {
		if renderer := ctrl.(controller.IAfter).After(data, info); renderer != nil {
			return nil, renderer
		}
	}

	return &MapperRet{
		Ms:      []models.IModel{modelObjReloaded},
		Fetcher: fetcher,
	}, nil
}
