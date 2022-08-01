package datamapper

import (
	"fmt"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/hfetcher"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

// How about AOP?
// https://github.com/gogap/aop

type batchOpJobV1 struct {
	serv service.IServiceV1

	oldmodelObjs []mdl.IModel // use for update (need to load and override for pegged fields)
	modelObjs    []mdl.IModel // current field value from the user if update, or from the loaded field if delete

	fetcher *hfetcher.HandlerFetcher
	data    *hook.Data
	ep      *hook.EndPoint
}

func batchOpCoreV1(job batchOpJobV1,
	taskFunc func(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, oldModelObj mdl.IModel) (mdl.IModel, error),
) (*MapperRet, *webrender.RetError) {
	modelObjs, oldmodelObjs := job.modelObjs, job.oldmodelObjs
	fetcher, data, ep := job.fetcher, job.data, job.ep

	ms := make([]mdl.IModel, len(modelObjs))

	if data.Cargo == nil {
		return nil, &webrender.RetError{Error: fmt.Errorf("cargo shouldn't be nil")}
	}

	// fetch all handlers with before hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "B") { // FetchHandlersForOpAndHook is stateful, cannot be repeated called
		if renderer := hdlr.(hook.IBefore).Before(data, ep); renderer != nil {
			return nil, renderer
		}
	}

	// TODO: Could update all at once, then load all at once again
	for i, modelObj := range modelObjs {
		id := modelObj.GetID()

		// m, err := updateOneCore(serv, db, oid, scope, typeString, modelObj, id)
		var m mdl.IModel
		var err error
		if oldmodelObjs == nil {
			m, err = taskFunc(data.DB, ep.Who, ep.TypeString, modelObj, id, nil)
		} else {
			m, err = taskFunc(data.DB, ep.Who, ep.TypeString, modelObj, id, oldmodelObjs[i])
		}
		if err != nil { // Error is "record not found" when not found
			return nil, &webrender.RetError{Error: err}
		}

		ms[i] = m
	}

	// fetch all handlers with after hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "A") {
		if renderer := hdlr.(hook.IAfter).After(data, ep); renderer != nil {
			return nil, renderer
		}
	}

	return &MapperRet{
		Ms:      ms,
		Fetcher: fetcher,
	}, nil
}

type opJobV1 struct {
	serv service.IServiceV1

	oldModelObj mdl.IModel // use for update (need to load and override for pegged fields)
	modelObj    mdl.IModel // current field value from the user if update, or from the loaded field if delete

	fetcher *hfetcher.HandlerFetcher
	data    *hook.Data
	ep      *hook.EndPoint
}

func opCoreV1(
	job opJobV1,
	taskFun func(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, oldModelObj mdl.IModel) (mdl.IModel, error),
) (*MapperRet, *webrender.RetError) {
	oldModelObj, modelObj := job.oldModelObj, job.modelObj
	fetcher, data, ep := job.fetcher, job.data, job.ep

	if data.Cargo == nil {
		return nil, &webrender.RetError{Error: fmt.Errorf("cargo shouldn't be nil")}
	}

	// fetch all handlers with before hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "B") {
		if renderer := hdlr.(hook.IBefore).Before(data, ep); renderer != nil {
			return nil, renderer
		}
	}

	// Now do the task
	id := modelObj.GetID()
	modelObjReloaded, err := taskFun(data.DB, ep.Who, ep.TypeString, modelObj, id, oldModelObj)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	data.Ms[0] = modelObjReloaded

	// fetch all handlers with after hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "A") {
		if renderer := hdlr.(hook.IAfter).After(data, ep); renderer != nil {
			return nil, renderer
		}
	}

	return &MapperRet{
		Ms:      []mdl.IModel{modelObjReloaded},
		Fetcher: fetcher,
	}, nil
}

// -------------------------------------------------------------------------------------------

type batchOpJobV2 struct {
	serv service.IServiceV2

	oldmodelObjs []mdl.IModel // use for update (need to load and override for pegged fields)
	modelObjs    []mdl.IModel // current field value from the user if update, or from the loaded field if delete

	fetcher *hfetcher.HandlerFetcher
	data    *hook.Data
	ep      *hook.EndPoint
}

func batchOpCoreV2(job batchOpJobV2,
	taskFunc func(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, oldModelObj mdl.IModel) (mdl.IModel, error),
) (*MapperRet, *webrender.RetError) {
	modelObjs, oldmodelObjs := job.modelObjs, job.oldmodelObjs
	fetcher, data, ep := job.fetcher, job.data, job.ep

	ms := make([]mdl.IModel, len(modelObjs))

	if data.Cargo == nil {
		return nil, &webrender.RetError{Error: fmt.Errorf("cargo shouldn't be nil")}
	}

	// fetch all handlers with before hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "B") { // FetchHandlersForOpAndHook is stateful, cannot be repeated called
		if renderer := hdlr.(hook.IBefore).Before(data, ep); renderer != nil {
			return nil, renderer
		}
	}

	// TODO: Could update all at once, then load all at once again
	for i, modelObj := range modelObjs {
		id := modelObj.GetID()

		// m, err := updateOneCore(serv, db, oid, scope, typeString, modelObj, id)
		var m mdl.IModel
		var err error
		if oldmodelObjs == nil {
			m, err = taskFunc(data.DB, ep.Who, ep.TypeString, modelObj, id, nil)
		} else {
			m, err = taskFunc(data.DB, ep.Who, ep.TypeString, modelObj, id, oldmodelObjs[i])
		}
		if err != nil { // Error is "record not found" when not found
			return nil, &webrender.RetError{Error: err}
		}

		ms[i] = m
	}

	// fetch all handlers with after hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "A") {
		if renderer := hdlr.(hook.IAfter).After(data, ep); renderer != nil {
			return nil, renderer
		}
	}

	return &MapperRet{
		Ms:      ms,
		Fetcher: fetcher,
	}, nil
}

type opJobV2 struct {
	serv service.IServiceV2

	oldModelObj mdl.IModel // use for update (need to load and override for pegged fields)
	modelObj    mdl.IModel // current field value from the user if update, or from the loaded field if delete

	fetcher *hfetcher.HandlerFetcher
	data    *hook.Data
	ep      *hook.EndPoint
}

func opCoreV2(
	job opJobV2,
	taskFun func(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, oldModelObj mdl.IModel) (mdl.IModel, error),
) (*MapperRet, *webrender.RetError) {
	oldModelObj, modelObj := job.oldModelObj, job.modelObj
	fetcher, data, ep := job.fetcher, job.data, job.ep

	if data.Cargo == nil {
		return nil, &webrender.RetError{Error: fmt.Errorf("cargo shouldn't be nil")}
	}

	// fetch all handlers with before hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "B") {
		if renderer := hdlr.(hook.IBefore).Before(data, ep); renderer != nil {
			return nil, renderer
		}
	}

	// Now do the task
	id := modelObj.GetID()
	modelObjReloaded, err := taskFun(data.DB, ep.Who, ep.TypeString, modelObj, id, oldModelObj)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	data.Ms[0] = modelObjReloaded

	// fetch all handlers with after hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "A") {
		if renderer := hdlr.(hook.IAfter).After(data, ep); renderer != nil {
			return nil, renderer
		}
	}

	return &MapperRet{
		Ms:      []mdl.IModel{modelObjReloaded},
		Fetcher: fetcher,
	}, nil
}
