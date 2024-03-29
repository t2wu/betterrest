package lifecycle

import (
	"strings"

	"github.com/go-chi/render"
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper"
	"github.com/t2wu/betterrest/datamapper/hfetcher"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/utils/transact"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

type Logger interface {
	Log(tx *gorm.DB, method, url, cardinality string)
}

func CreateMany(db *gorm.DB, mapper datamapper.IDataMapper, modelObjs []mdl.IModel,
	ep *hook.EndPoint, cargo *hook.Cargo, logger Logger) (*hook.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	if cargo == nil {
		cargo = &hook.Cargo{}
	}

	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "POST", strings.ToLower(ep.TypeString), "n")
		}

		if retVal, retErr = mapper.Create(tx, modelObjs, ep, cargo); retErr != nil {
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

	roles := make([]userrole.UserRole, len(modelObjs))
	// admin is 0 so it's ok
	for i := 0; i < len(modelObjs); i++ {
		roles[i] = userrole.UserRoleAdmin
	}

	data := hook.Data{Ms: modelObjs, DB: nil, Roles: roles, Cargo: cargo}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hook.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func CreateOne(db *gorm.DB, mapper datamapper.IDataMapper, modelObj mdl.IModel,
	ep *hook.EndPoint, cargo *hook.Cargo, logger Logger) (*hook.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	if cargo == nil {
		cargo = &hook.Cargo{}
	}

	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "POST", strings.ToLower(ep.TypeString), "1")
		}

		if retVal, retErr = mapper.Create(tx, []mdl.IModel{modelObj}, ep, cargo); retErr != nil {
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

	roles := []userrole.UserRole{userrole.UserRoleAdmin} // just one item
	ms := []mdl.IModel{modelObj}

	data := hook.Data{Ms: ms, DB: nil, Roles: roles, Cargo: cargo}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hook.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

// ReadMany
func ReadMany(db *gorm.DB, mapper datamapper.IDataMapper, ep *hook.EndPoint, cargo *hook.Cargo, logger Logger) (*hook.Data, *int, *hfetcher.HandlerFetcher, render.Renderer) {
	if logger != nil {
		logger.Log(nil, "GET", strings.ToLower(ep.TypeString), "n")
	}

	if cargo == nil {
		cargo = &hook.Cargo{}
	}

	var retVal *datamapper.MapperRet
	retVal, roles, no, retErr := mapper.ReadMany(db, ep, cargo)
	if retErr != nil {
		if retErr.Renderer == nil {
			return nil, no, nil, webrender.NewErrInternalServerError(retErr.Error) // TODO, probably should have a READ error
		}
		return nil, no, nil, retErr.Renderer
	}

	modelObjs := retVal.Ms

	data := hook.Data{Ms: modelObjs, DB: nil, Roles: roles, Cargo: cargo}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hook.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, no, retVal.Fetcher, nil
}

func ReadOne(db *gorm.DB, mapper datamapper.IDataMapper, id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo,
	logger Logger) (*hook.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	if logger != nil {
		logger.Log(nil, "GET", strings.ToLower(ep.TypeString), "1")
	}

	if cargo == nil {
		cargo = &hook.Cargo{}
	}

	retVal, role, retErr := mapper.ReadOne(db, id, ep, cargo)
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

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: nil, Roles: []userrole.UserRole{role}, Cargo: cargo}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hook.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func UpdateMany(db *gorm.DB, mapper datamapper.IDataMapper, modelObjs []mdl.IModel,
	ep *hook.EndPoint, cargo *hook.Cargo, logger Logger) (*hook.Data, *hfetcher.HandlerFetcher, render.Renderer) {

	if cargo == nil {
		cargo = &hook.Cargo{}
	}

	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "PUT", strings.ToLower(ep.TypeString), "n")
		}

		if retVal, retErr = mapper.Update(tx, modelObjs, ep, cargo); retErr != nil {
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

	roles := make([]userrole.UserRole, len(modelObjs))
	for i := 0; i < len(roles); i++ {
		roles[i] = userrole.UserRoleAdmin
	}

	data := hook.Data{Ms: modelObjs, DB: nil, Roles: roles, Cargo: cargo}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hook.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func UpdateOne(db *gorm.DB, mapper datamapper.IDataMapper, modelObj mdl.IModel, id *datatype.UUID,
	ep *hook.EndPoint, cargo *hook.Cargo, logger Logger) (*hook.Data, *hfetcher.HandlerFetcher, render.Renderer) {

	if cargo == nil {
		cargo = &hook.Cargo{}
	}

	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "PUT", strings.ToLower(ep.TypeString), "1")
		}

		if retVal, retErr = mapper.Update(tx, []mdl.IModel{modelObj}, ep, cargo); retErr != nil {
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

	role := userrole.UserRoleAdmin
	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: nil, Roles: []userrole.UserRole{role}, Cargo: cargo}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hook.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func PatchMany(db *gorm.DB, mapper datamapper.IDataMapper, jsonIDPatches []mdlutil.JSONIDPatch,
	ep *hook.EndPoint, cargo *hook.Cargo, logger Logger) (*hook.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	var modelObjs []mdl.IModel

	if cargo == nil {
		cargo = &hook.Cargo{}
	}

	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "PATCH", strings.ToLower(ep.TypeString), "n")
		}

		if retVal, retErr = mapper.Patch(tx, jsonIDPatches, ep, cargo); retErr != nil {
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

	roles := make([]userrole.UserRole, len(modelObjs))
	for i := 0; i < len(roles); i++ {
		roles[i] = userrole.UserRoleAdmin
	}

	data := hook.Data{Ms: modelObjs, DB: nil, Roles: roles, Cargo: cargo}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hook.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func PatchOne(db *gorm.DB, mapper datamapper.IDataMapper, jsonPatch []byte,
	id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo, logger Logger) (*hook.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	if cargo == nil {
		cargo = &hook.Cargo{}
	}

	var modelObj mdl.IModel
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "PATCH", strings.ToLower(ep.TypeString), "1")
		}

		jsonIDPatches := []mdlutil.JSONIDPatch{
			{
				ID:    id,
				Patch: jsonPatch,
			},
		}

		if retVal, retErr = mapper.Patch(tx, jsonIDPatches, ep, cargo); retErr != nil {
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

	role := userrole.UserRoleAdmin
	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: nil, Roles: []userrole.UserRole{role}, Cargo: cargo}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hook.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func DeleteMany(db *gorm.DB, mapper datamapper.IDataMapper, modelObjs []mdl.IModel,
	ep *hook.EndPoint, cargo *hook.Cargo, logger Logger) (*hook.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	if cargo == nil {
		cargo = &hook.Cargo{}
	}

	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		if logger != nil {
			logger.Log(tx, "DELETE", strings.ToLower(ep.TypeString), "n")
		}

		if retVal, retErr = mapper.DeleteMany(tx, modelObjs, ep, cargo); retErr != nil {
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

	roles := make([]userrole.UserRole, len(modelObjs))
	for i := 0; i < len(roles); i++ {
		roles[i] = userrole.UserRoleAdmin
	}

	data := hook.Data{Ms: modelObjs, DB: nil, Roles: roles, Cargo: cargo}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hook.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}

func DeleteOne(db *gorm.DB, mapper datamapper.IDataMapper, id *datatype.UUID,
	ep *hook.EndPoint, cargo *hook.Cargo, logger Logger) (*hook.Data, *hfetcher.HandlerFetcher, render.Renderer) {
	if cargo == nil {
		cargo = &hook.Cargo{}
	}
	var retVal *datamapper.MapperRet
	retErr := transact.TransactCustomError(db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		logger.Log(tx, "DELETE", strings.ToLower(ep.TypeString), "1")

		if retVal, retErr = mapper.DeleteOne(tx, id, ep, cargo); retErr != nil {
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

	role := userrole.UserRoleAdmin
	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: nil, Roles: []userrole.UserRole{role}, Cargo: cargo}

	for _, hdlr := range retVal.Fetcher.FetchHandlersForOpAndHook(ep.Op, "T") {
		hdlr.(hook.IAfterTransact).AfterTransact(&data, ep)
	}

	return &data, retVal.Fetcher, nil
}
