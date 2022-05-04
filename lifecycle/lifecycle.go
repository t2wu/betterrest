package lifecycle

import (
	"strings"

	"github.com/go-chi/render"
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper"
	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/utils/transact"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry"
)

type Logger interface {
	Log(tx *gorm.DB, method, url, cardinality string)
}

func callOldBatchTransact(data *hookhandler.Data, info *hookhandler.EndPointInfo) {
	oldBatchCargo := models.BatchHookCargo{Payload: data.Cargo.Payload}
	bhpData := models.BatchHookPointData{Ms: data.Ms, DB: nil, Who: data.Who,
		TypeString: data.TypeString, Roles: data.Roles, URLParams: data.URLParams, Cargo: &oldBatchCargo}

	var op models.CRUPDOp
	switch info.Op {
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
	if afterTransact := registry.ModelRegistry[data.TypeString].AfterTransact; afterTransact != nil {
		afterTransact(bhpData, op)
	}

	data.Cargo.Payload = oldBatchCargo.Payload
}

func callOldOneTransact(data *hookhandler.Data, info *hookhandler.EndPointInfo) {
	oldSingleCargo := models.ModelCargo{Payload: data.Cargo.Payload}
	hpdata := models.HookPointData{DB: nil, Who: data.Who, TypeString: data.TypeString,
		URLParams: data.URLParams, Role: &data.Roles[0], Cargo: &oldSingleCargo}

	var op models.CRUPDOp
	switch info.Op {
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

func CreateMany(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, modelObjs []models.IModel, options map[urlparam.Param]interface{}, logger Logger) (*hookhandler.Data, *hookhandler.EndPointInfo, render.Renderer) {
	cargo := hookhandler.Cargo{}

	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "POST", strings.ToLower(typeString), "n")
		}

		if retVal, retErr = mapper.CreateMany(tx, who, typeString, modelObjs, options, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.CreateMany")

	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, nil, webrender.NewErrCreate(retErr.Error)
		}
		return nil, nil, retErr.Renderer
	}

	modelObjs = retVal.Ms

	roles := make([]models.UserRole, len(modelObjs))
	// admin is 0 so it's ok
	for i := 0; i < len(modelObjs); i++ {
		roles[i] = models.UserRoleAdmin
	}

	data := hookhandler.Data{Ms: modelObjs, DB: nil, Who: who,
		TypeString: typeString, Roles: roles, URLParams: options, Cargo: &cargo}
	info := hookhandler.EndPointInfo{
		Op:          hookhandler.RESTOpCreate,
		Cardinality: hookhandler.APICardinalityMany,
	}

	// Handes transaction
	if !retVal.Fetcher.HasRegisteredHandler() {
		// It's possible that the user has no hookhandler, even if it's new code
		callOldBatchTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(info.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func CreateOne(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, modelObj models.IModel, options map[urlparam.Param]interface{}, logger Logger) (*hookhandler.Data, *hookhandler.EndPointInfo, render.Renderer) {
	cargo := hookhandler.Cargo{}

	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "POST", strings.ToLower(typeString), "1")
		}

		if retVal, retErr = mapper.CreateOne(tx, who, typeString, modelObj, options, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.CreateOne")

	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, nil, webrender.NewErrCreate(retErr.Error)
		}
		return nil, nil, retErr.Renderer
	}

	modelObj = retVal.Ms[0]

	roles := []models.UserRole{models.UserRoleAdmin} // just one item
	ms := []models.IModel{modelObj}

	data := hookhandler.Data{Ms: ms, DB: nil, Who: who,
		TypeString: typeString, Roles: roles, URLParams: options, Cargo: &cargo}
	info := hookhandler.EndPointInfo{
		Op:          hookhandler.RESTOpCreate,
		Cardinality: hookhandler.APICardinalityMany,
	}

	// Handes transaction
	if !retVal.Fetcher.HasRegisteredHandler() {
		// It's possible that the user has no hookhandler, even if it's new code
		callOldOneTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(info.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

// ReadMany
func ReadMany(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, options map[urlparam.Param]interface{}, logger Logger) (*hookhandler.Data, *hookhandler.EndPointInfo, *int, render.Renderer) {
	if logger != nil {
		logger.Log(nil, "GET", strings.ToLower(typeString), "n")
	}

	cargo := hookhandler.Cargo{}
	var retVal *datamapper.MapperRet
	retVal, roles, no, retErr := mapper.ReadMany(db.Shared(), who, typeString, options, &cargo)
	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, nil, no, webrender.NewErrInternalServerError(retErr.Error) // TODO, probably should have a READ error
		}
		return nil, nil, no, retErr.Renderer
	}

	modelObjs := retVal.Ms

	data := hookhandler.Data{Ms: modelObjs, DB: nil, Who: who,
		TypeString: typeString, Roles: roles, URLParams: options, Cargo: &cargo}
	info := hookhandler.EndPointInfo{
		Op:          hookhandler.RESTOpRead,
		Cardinality: hookhandler.APICardinalityMany,
	}

	// Handes transaction
	if !retVal.Fetcher.HasRegisteredHandler() {
		// It's possible that the user has no hookhandler, even if it's new code
		callOldBatchTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, no, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(info.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, &info)
	}

	return &data, &info, no, nil
}

func ReadOne(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, id *datatypes.UUID, options map[urlparam.Param]interface{}, logger Logger) (*hookhandler.Data, *hookhandler.EndPointInfo, render.Renderer) {
	if logger != nil {
		logger.Log(nil, "GET", strings.ToLower(typeString), "1")
	}

	cargo := hookhandler.Cargo{}
	retVal, role, retErr := mapper.ReadOne(db.Shared(), who, typeString, id, options, &cargo)
	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, nil, webrender.NewErrInternalServerError(retErr.Error) // TODO, probably should have a READ error
		}
		if gorm.IsRecordNotFoundError(retErr.Error) {
			return nil, nil, webrender.NewErrNotFound(retErr.Error)
		}
		return nil, nil, retErr.Renderer
	}

	modelObj := retVal.Ms[0]

	data := hookhandler.Data{Ms: []models.IModel{modelObj}, DB: nil, Who: who,
		TypeString: typeString, Roles: []models.UserRole{role}, URLParams: options, Cargo: &cargo}
	info := hookhandler.EndPointInfo{
		Op:          hookhandler.RESTOpRead,
		Cardinality: hookhandler.APICardinalityOne,
	}

	if !retVal.Fetcher.HasRegisteredHandler() {
		callOldOneTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(info.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func UpdateMany(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, modelObjs []models.IModel, options map[urlparam.Param]interface{}, logger Logger) (*hookhandler.Data, *hookhandler.EndPointInfo, render.Renderer) {
	cargo := hookhandler.Cargo{}
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "PUT", strings.ToLower(typeString), "n")
		}

		if retVal, retErr = mapper.UpdateMany(tx, who, typeString, modelObjs, options, &cargo); retErr != nil {
			return retErr
		}

		return nil
	}, "lifecycle.UpdateMany")
	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, nil, webrender.NewErrUpdate(retErr.Error)
		}
		return nil, nil, retErr.Renderer
	}

	modelObjs = retVal.Ms

	roles := make([]models.UserRole, len(modelObjs))
	for i := 0; i < len(roles); i++ {
		roles[i] = models.UserRoleAdmin
	}

	data := hookhandler.Data{Ms: modelObjs, DB: nil, Who: who,
		TypeString: typeString, Roles: roles, URLParams: options, Cargo: &cargo}
	info := hookhandler.EndPointInfo{
		Op:          hookhandler.RESTOpUpdate,
		Cardinality: hookhandler.APICardinalityMany,
	}

	if !retVal.Fetcher.HasRegisteredHandler() {
		callOldBatchTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(info.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func UpdateOne(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, modelObj models.IModel, id *datatypes.UUID, options map[urlparam.Param]interface{}, logger Logger) (*hookhandler.Data, *hookhandler.EndPointInfo, render.Renderer) {
	cargo := hookhandler.Cargo{}
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "PUT", strings.ToLower(typeString), "1")
		}

		if retVal, retErr = mapper.UpdateOne(tx, who, typeString, modelObj, id, options, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.UpdateOne")
	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, nil, webrender.NewErrUpdate(retErr.Error)
		}
		return nil, nil, retErr.Renderer
	}

	modelObj = retVal.Ms[0]

	role := models.UserRoleAdmin
	data := hookhandler.Data{Ms: []models.IModel{modelObj}, DB: nil, Who: who,
		TypeString: typeString, Roles: []models.UserRole{role}, URLParams: options, Cargo: &cargo}
	info := hookhandler.EndPointInfo{
		Op:          hookhandler.RESTOpUpdate,
		Cardinality: hookhandler.APICardinalityOne,
	}

	if !retVal.Fetcher.HasRegisteredHandler() {
		callOldOneTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(info.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func PatchMany(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, jsonIDPatches []models.JSONIDPatch, options map[urlparam.Param]interface{}, logger Logger) (*hookhandler.Data, *hookhandler.EndPointInfo, render.Renderer) {
	var modelObjs []models.IModel
	cargo := hookhandler.Cargo{}
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "PATCH", strings.ToLower(typeString), "n")
		}

		if retVal, retErr = mapper.PatchMany(tx, who, typeString, jsonIDPatches, options, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.PatchMany")

	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, nil, webrender.NewErrPatch(retErr.Error)
		}
		return nil, nil, retErr.Renderer
	}

	modelObjs = retVal.Ms

	roles := make([]models.UserRole, len(modelObjs))
	for i := 0; i < len(roles); i++ {
		roles[i] = models.UserRoleAdmin
	}

	data := hookhandler.Data{Ms: modelObjs, DB: nil, Who: who,
		TypeString: typeString, Roles: roles, URLParams: options, Cargo: &cargo}
	info := hookhandler.EndPointInfo{
		Op:          hookhandler.RESTOpPatch,
		Cardinality: hookhandler.APICardinalityMany,
	}

	if !retVal.Fetcher.HasRegisteredHandler() {
		callOldBatchTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(info.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func PatchOne(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, jsonPatch []byte, id *datatypes.UUID, options map[urlparam.Param]interface{}, logger Logger) (*hookhandler.Data, *hookhandler.EndPointInfo, render.Renderer) {
	cargo := hookhandler.Cargo{}
	var modelObj models.IModel
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "PATCH", strings.ToLower(typeString), "1")
		}

		if retVal, retErr = mapper.PatchOne(tx, who, typeString, jsonPatch, id, options, &cargo); retErr != nil {
			return retErr
		}

		return nil
	}, "lifecycle.PatchOne")

	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, nil, webrender.NewErrPatch(retErr.Error)
		}
		return nil, nil, retErr.Renderer
	}

	modelObj = retVal.Ms[0]

	role := models.UserRoleAdmin
	data := hookhandler.Data{Ms: []models.IModel{modelObj}, DB: nil, Who: who,
		TypeString: typeString, Roles: []models.UserRole{role}, URLParams: options, Cargo: &cargo}
	info := hookhandler.EndPointInfo{
		Op:          hookhandler.RESTOpPatch,
		Cardinality: hookhandler.APICardinalityOne,
	}

	if !retVal.Fetcher.HasRegisteredHandler() {
		callOldOneTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(info.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func DeleteMany(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, modelObjs []models.IModel, options map[urlparam.Param]interface{}, logger Logger) (*hookhandler.Data, *hookhandler.EndPointInfo, render.Renderer) {
	cargo := hookhandler.Cargo{}
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "DELETE", strings.ToLower(typeString), "n")
		}

		if retVal, retErr = mapper.DeleteMany(tx, who, typeString, modelObjs, nil, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.DeleteMany")

	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, nil, webrender.NewErrDelete(retErr.Error)
		}
		return nil, nil, retErr.Renderer
	}

	modelObjs = retVal.Ms

	roles := make([]models.UserRole, len(modelObjs))
	for i := 0; i < len(roles); i++ {
		roles[i] = models.UserRoleAdmin
	}

	data := hookhandler.Data{Ms: modelObjs, DB: nil, Who: who,
		TypeString: typeString, Roles: roles, URLParams: options, Cargo: &cargo}
	info := hookhandler.EndPointInfo{
		Op:          hookhandler.RESTOpDelete,
		Cardinality: hookhandler.APICardinalityMany,
	}

	if !retVal.Fetcher.HasRegisteredHandler() {
		callOldBatchTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(info.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func DeleteOne(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, id *datatypes.UUID, options map[urlparam.Param]interface{}, logger Logger) (*hookhandler.Data, *hookhandler.EndPointInfo, render.Renderer) {
	cargo := hookhandler.Cargo{}
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retErr *webrender.RetError) {
		logger.Log(tx, "DELETE", strings.ToLower(typeString), "1")

		if retVal, retErr = mapper.DeleteOne(tx, who, typeString, id, options, &cargo); retErr != nil {
			return retErr
		}
		return
	}, "lifecycle.DeleteOne")
	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, nil, webrender.NewErrDelete(retErr.Error)
		}
		return nil, nil, retErr.Renderer
	}

	modelObj := retVal.Ms[0]

	role := models.UserRoleAdmin
	data := hookhandler.Data{Ms: []models.IModel{modelObj}, DB: nil, Who: who,
		TypeString: typeString, Roles: []models.UserRole{role}, URLParams: options, Cargo: &cargo}
	info := hookhandler.EndPointInfo{
		Op:          hookhandler.RESTOpDelete,
		Cardinality: hookhandler.APICardinalityOne,
	}

	if !retVal.Fetcher.HasRegisteredHandler() {
		callOldOneTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(info.Op, "T") {
		hdlr.(hookhandler.IAfterTransact).AfterTransact(&data, &info)
	}

	return &data, &info, nil
}