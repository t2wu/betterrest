package lifecycle

import (
	"strings"

	"github.com/go-chi/render"
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper"
	"github.com/t2wu/betterrest/datamapper/hfetcher"
	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/utils/transact"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry"
)

type Logger interface {
	Log(tx *gorm.DB, method, url, cardinality string)
}

func callOldBatchTransact(data *hookhandler.Data, ep *hookhandler.EndPointInfo) {
	oldBatchCargo := models.BatchHookCargo{Payload: data.Cargo.Payload}
	bhpData := models.BatchHookPointData{Ms: data.Ms, DB: nil, Who: ep.Who,
		TypeString: ep.TypeString, Roles: data.Roles, URLParams: ep.URLParams, Cargo: &oldBatchCargo}

	var op models.CRUPDOp
	switch ep.Op {
	case hookhandler.RESTOpRead:
		op = models.CRUPDOpRead
	case hookhandler.RESTOpCreate:
		op = models.CRUPDOpCreate
	case hookhandler.RESTOpUpdate:
		op = models.CRUPDOpUpdate
	case hookhandler.RESTOpPatch:
		op = models.CRUPDOpPatch
	case hookhandler.RESTOpDelete:
		op = models.CRUPDOpDelete
	}

	// the batch afterTransact hookpoint
	if afterTransact := registry.ModelRegistry[ep.TypeString].AfterTransact; afterTransact != nil {
		afterTransact(bhpData, op)
	}

	data.Cargo.Payload = oldBatchCargo.Payload
}

func callOldOneTransact(data *hookhandler.Data, ep *hookhandler.EndPointInfo) {
	oldSingleCargo := models.ModelCargo{Payload: data.Cargo.Payload}
	hpdata := models.HookPointData{DB: nil, Who: ep.Who, TypeString: ep.TypeString,
		URLParams: ep.URLParams, Role: &data.Roles[0], Cargo: &oldSingleCargo}

	var op models.CRUPDOp
	switch ep.Op {
	case hookhandler.RESTOpRead:
		op = models.CRUPDOpRead
	case hookhandler.RESTOpCreate:
		op = models.CRUPDOpCreate
	case hookhandler.RESTOpUpdate:
		op = models.CRUPDOpUpdate
	case hookhandler.RESTOpPatch:
		op = models.CRUPDOpPatch
	case hookhandler.RESTOpDelete:
		op = models.CRUPDOpDelete
	}

	// the single afterTransact hookpoint
	if v, ok := data.Ms[0].(models.IAfterTransact); ok {
		v.AfterTransact(hpdata, op)
	}
	data.Cargo.Payload = hpdata.Cargo.Payload
}

func CreateMany(mapper datamapper.IDataMapper, modelObjs []models.IModel,
	ep *hookhandler.EndPointInfo, logger Logger) (*hookhandler.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	cargo := hookhandler.Cargo{}

	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "POST", strings.ToLower(ep.TypeString), "n")
		}

		if retVal, retErr = mapper.CreateMany(tx, modelObjs, ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.CreateMany")

	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, retVal.Fetcher, webrender.NewErrCreate(retErr.Error)
		}
		return nil, retVal.Fetcher, retErr.Renderer
	}

	modelObjs = retVal.Ms

	roles := make([]models.UserRole, len(modelObjs))
	// admin is 0 so it's ok
	for i := 0; i < len(modelObjs); i++ {
		roles[i] = models.UserRoleAdmin
	}

	data := hookhandler.Data{Ms: modelObjs, DB: nil, Roles: roles, Cargo: &cargo}

	// Handes transaction
	if !retVal.Fetcher.HasAttemptRegisteringHandler() {
		// It's possible that the user has no hookhandler, even if it's new code
		callOldBatchTransact(&data, ep) // for backward compatibility, for now
		return &data, retVal.Fetcher, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func CreateOne(mapper datamapper.IDataMapper, modelObj models.IModel,
	ep *hookhandler.EndPointInfo, logger Logger) (*hookhandler.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	cargo := hookhandler.Cargo{}

	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "POST", strings.ToLower(ep.TypeString), "1")
		}

		if retVal, retErr = mapper.CreateOne(tx, modelObj, ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.CreateOne")

	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, retVal.Fetcher, webrender.NewErrCreate(retErr.Error)
		}
		return nil, retVal.Fetcher, retErr.Renderer
	}

	modelObj = retVal.Ms[0]

	roles := []models.UserRole{models.UserRoleAdmin} // just one item
	ms := []models.IModel{modelObj}

	data := hookhandler.Data{Ms: ms, DB: nil, Roles: roles, Cargo: &cargo}

	// Handes transaction
	if !retVal.Fetcher.HasAttemptRegisteringHandler() {
		// It's possible that the user has no hookhandler, even if it's new code
		callOldOneTransact(&data, ep) // for backward compatibility, for now
		return &data, retVal.Fetcher, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

// ReadMany
func ReadMany(mapper datamapper.IDataMapper, ep *hookhandler.EndPointInfo, logger Logger) (*hookhandler.Data, *int, *hfetcher.HandlerFetcher, render.Renderer) {
	if logger != nil {
		logger.Log(nil, "GET", strings.ToLower(ep.TypeString), "n")
	}

	cargo := hookhandler.Cargo{}
	var retVal *datamapper.MapperRet
	retVal, roles, no, retErr := mapper.ReadMany(db.Shared(), ep, &cargo)
	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, no, retVal.Fetcher, webrender.NewErrInternalServerError(retErr.Error) // TODO, probably should have a READ error
		}
		return nil, no, retVal.Fetcher, retErr.Renderer
	}

	modelObjs := retVal.Ms

	data := hookhandler.Data{Ms: modelObjs, DB: nil, Roles: roles, Cargo: &cargo}

	// Handes transaction
	if !retVal.Fetcher.HasAttemptRegisteringHandler() {
		// It's possible that the user has no hookhandler, even if it's new code
		callOldBatchTransact(&data, ep) // for backward compatibility, for now
		return &data, no, retVal.Fetcher, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, no, retVal.Fetcher, nil
}

func ReadOne(mapper datamapper.IDataMapper, id *datatypes.UUID, ep *hookhandler.EndPointInfo,
	logger Logger) (*hookhandler.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	if logger != nil {
		logger.Log(nil, "GET", strings.ToLower(ep.TypeString), "1")
	}

	cargo := hookhandler.Cargo{}
	retVal, role, retErr := mapper.ReadOne(db.Shared(), id, ep, &cargo)
	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, retVal.Fetcher, webrender.NewErrInternalServerError(retErr.Error) // TODO, probably should have a READ error
		}
		if gorm.IsRecordNotFoundError(retErr.Error) {
			return nil, retVal.Fetcher, webrender.NewErrNotFound(retErr.Error)
		}
		return nil, retVal.Fetcher, retErr.Renderer
	}

	modelObj := retVal.Ms[0]

	data := hookhandler.Data{Ms: []models.IModel{modelObj}, DB: nil, Roles: []models.UserRole{role}, Cargo: &cargo}

	if !retVal.Fetcher.HasAttemptRegisteringHandler() {
		callOldOneTransact(&data, ep) // for backward compatibility, for now
		return &data, retVal.Fetcher, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func UpdateMany(mapper datamapper.IDataMapper, modelObjs []models.IModel,
	ep *hookhandler.EndPointInfo, logger Logger) (*hookhandler.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	cargo := hookhandler.Cargo{}
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "PUT", strings.ToLower(ep.TypeString), "n")
		}

		if retVal, retErr = mapper.UpdateMany(tx, modelObjs, ep, &cargo); retErr != nil {
			return retErr
		}

		return nil
	}, "lifecycle.UpdateMany")
	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, retVal.Fetcher, webrender.NewErrUpdate(retErr.Error)
		}
		return nil, retVal.Fetcher, retErr.Renderer
	}

	modelObjs = retVal.Ms

	roles := make([]models.UserRole, len(modelObjs))
	for i := 0; i < len(roles); i++ {
		roles[i] = models.UserRoleAdmin
	}

	data := hookhandler.Data{Ms: modelObjs, DB: nil, Roles: roles, Cargo: &cargo}

	if !retVal.Fetcher.HasAttemptRegisteringHandler() {
		callOldBatchTransact(&data, ep) // for backward compatibility, for now
		return &data, retVal.Fetcher, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func UpdateOne(mapper datamapper.IDataMapper, modelObj models.IModel, id *datatypes.UUID,
	ep *hookhandler.EndPointInfo, logger Logger) (*hookhandler.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	cargo := hookhandler.Cargo{}
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "PUT", strings.ToLower(ep.TypeString), "1")
		}

		if retVal, retErr = mapper.UpdateOne(tx, modelObj, id, ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.UpdateOne")
	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, retVal.Fetcher, webrender.NewErrUpdate(retErr.Error)
		}
		return nil, retVal.Fetcher, retErr.Renderer
	}

	modelObj = retVal.Ms[0]

	role := models.UserRoleAdmin
	data := hookhandler.Data{Ms: []models.IModel{modelObj}, DB: nil, Roles: []models.UserRole{role}, Cargo: &cargo}

	if !retVal.Fetcher.HasAttemptRegisteringHandler() {
		callOldOneTransact(&data, ep) // for backward compatibility, for now
		return &data, retVal.Fetcher, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func PatchMany(mapper datamapper.IDataMapper, jsonIDPatches []models.JSONIDPatch,
	ep *hookhandler.EndPointInfo, logger Logger) (*hookhandler.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	var modelObjs []models.IModel
	cargo := hookhandler.Cargo{}
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "PATCH", strings.ToLower(ep.TypeString), "n")
		}

		if retVal, retErr = mapper.PatchMany(tx, jsonIDPatches, ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.PatchMany")

	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, retVal.Fetcher, webrender.NewErrPatch(retErr.Error)
		}
		return nil, retVal.Fetcher, retErr.Renderer
	}

	modelObjs = retVal.Ms

	roles := make([]models.UserRole, len(modelObjs))
	for i := 0; i < len(roles); i++ {
		roles[i] = models.UserRoleAdmin
	}

	data := hookhandler.Data{Ms: modelObjs, DB: nil, Roles: roles, Cargo: &cargo}

	if !retVal.Fetcher.HasAttemptRegisteringHandler() {
		callOldBatchTransact(&data, ep) // for backward compatibility, for now
		return &data, retVal.Fetcher, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func PatchOne(mapper datamapper.IDataMapper, jsonPatch []byte,
	id *datatypes.UUID, ep *hookhandler.EndPointInfo, logger Logger) (*hookhandler.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	cargo := hookhandler.Cargo{}
	var modelObj models.IModel
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "PATCH", strings.ToLower(ep.TypeString), "1")
		}

		if retVal, retErr = mapper.PatchOne(tx, jsonPatch, id, ep, &cargo); retErr != nil {
			return retErr
		}

		return nil
	}, "lifecycle.PatchOne")

	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, retVal.Fetcher, webrender.NewErrPatch(retErr.Error)
		}
		return nil, retVal.Fetcher, retErr.Renderer
	}

	modelObj = retVal.Ms[0]

	role := models.UserRoleAdmin
	data := hookhandler.Data{Ms: []models.IModel{modelObj}, DB: nil, Roles: []models.UserRole{role}, Cargo: &cargo}

	if !retVal.Fetcher.HasAttemptRegisteringHandler() {
		callOldOneTransact(&data, ep) // for backward compatibility, for now
		return &data, retVal.Fetcher, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func DeleteMany(mapper datamapper.IDataMapper, modelObjs []models.IModel,
	ep *hookhandler.EndPointInfo, logger Logger) (*hookhandler.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	cargo := hookhandler.Cargo{}
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "DELETE", strings.ToLower(ep.TypeString), "n")
		}

		if retVal, retErr = mapper.DeleteMany(tx, modelObjs, ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.DeleteMany")

	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, retVal.Fetcher, webrender.NewErrDelete(retErr.Error)
		}
		return nil, retVal.Fetcher, retErr.Renderer
	}

	modelObjs = retVal.Ms

	roles := make([]models.UserRole, len(modelObjs))
	for i := 0; i < len(roles); i++ {
		roles[i] = models.UserRoleAdmin
	}

	data := hookhandler.Data{Ms: modelObjs, DB: nil, Roles: roles, Cargo: &cargo}

	if !retVal.Fetcher.HasAttemptRegisteringHandler() {
		callOldBatchTransact(&data, ep) // for backward compatibility, for now
		return &data, retVal.Fetcher, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func DeleteOne(mapper datamapper.IDataMapper, id *datatypes.UUID,
	ep *hookhandler.EndPointInfo, logger Logger) (*hookhandler.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	cargo := hookhandler.Cargo{}
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		logger.Log(tx, "DELETE", strings.ToLower(ep.TypeString), "1")

		if retVal, retErr = mapper.DeleteOne(tx, id, ep, &cargo); retErr != nil {
			return retErr
		}
		return
	}, "lifecycle.DeleteOne")
	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, retVal.Fetcher, webrender.NewErrDelete(retErr.Error)
		}
		return nil, retVal.Fetcher, retErr.Renderer
	}

	modelObj := retVal.Ms[0]

	role := models.UserRoleAdmin
	data := hookhandler.Data{Ms: []models.IModel{modelObj}, DB: nil, Roles: []models.UserRole{role}, Cargo: &cargo}

	if !retVal.Fetcher.HasAttemptRegisteringHandler() {
		callOldOneTransact(&data, ep) // for backward compatibility, for now
		return &data, retVal.Fetcher, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}
