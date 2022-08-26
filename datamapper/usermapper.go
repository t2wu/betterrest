package datamapper

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/t2wu/betterrest/datamapper/hfetcher"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/rest"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/betterrest/model/mappertype"
	"github.com/t2wu/betterrest/registry"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"

	"github.com/jinzhu/gorm"
)

// ---------------------------------------

var (
	onceUser sync.Once
	usercrud *UserMapper
)

// SetUserMapper allows one to mock UserMapper for testing
// func SetUserMapper(mapper IDataMapper) {
// 	onceUser.Do(func() {
// 		usercrud = mapper
// 	})
// }

// UserMapper is a User CRUD manager
type UserMapper struct {
	Service    service.IService
	MapperType mappertype.MapperType
}

// SharedUserMapper creats a singleton of Crud object
func SharedUserMapper() *UserMapper {
	onceUser.Do(func() {
		usercrud = &UserMapper{Service: &service.UserService{
			BaseService: service.BaseService{}},
			MapperType: mappertype.User,
		}
	})

	return usercrud
}

//------------------------
// User specific CRUD
// Cuz user is spcial, need to create ownership and no need to check for owner
// ------------------------------------

// CreateOne creates an user model based on json and store it in db
// Also creates a ownership with admin access
func (mapper *UserMapper) Create(db *gorm.DB, modelObjs []mdl.IModel,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	if ep.Cardinality == rest.CardinalityMany {
		err := fmt.Errorf("CardinalityOne endpoint not supported")
		renderer := webrender.NewErrPermissionDeniedForAPIEndpoint(err)
		return nil, webrender.NewRetValWithRendererError(err, renderer)
	}

	// modelObj, err := mapper.Service.HookBeforeCreateOne(db, ep.Who, ep.TypeString, modelObj)
	// if err != nil {
	// 	return nil, &webrender.RetError{Error: err}
	// }

	modelObj := modelObjs[0]

	if modelObj.GetID() == nil {
		modelObj.SetID(datatype.NewUUID())
	}

	data := &hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Cargo: cargo}
	data, retErr := mapper.Service.PermissionAndRole(data, ep) // data's role should be filled or not depending on the mapper
	if retErr != nil {
		return nil, retErr
	}

	initData := hook.InitData{Roles: data.Roles, Ep: ep}

	j := batchOpJob{
		serv:         mapper.Service,
		oldmodelObjs: nil,
		modelObjs:    []mdl.IModel{modelObj},

		fetcher: hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:    data,
		ep:      ep,
	}
	mapperRet, retval := batchOpCore(j, mapper.Service.CreateOneCore)
	if retval != nil {
		if strings.Contains(retval.Error.Error(), "duplicate key") {
			err := fmt.Errorf("account already exists")
			renderer := webrender.NewErrDuplicatedRecord(err)
			return mapperRet, webrender.NewRetValWithRendererError(err, renderer)
		}
		return mapperRet, retval
	}
	return mapperRet, nil
}

// // CreateMany is currently a dummy
// func (mapper *UserMapper) CreateMany(db *gorm.DB,  modelObjs []mdl.IModel) ([]mdl.IModel, error) {
// 	// not really implemented
// 	return nil, errors.New("not implemented")
// }

// ReadOne get one model object based on its type and its id string
func (mapper *UserMapper) ReadOne(db *gorm.DB, id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, userrole.UserRole, *webrender.RetError) {
	modelObj, role, err := mapper.Service.ReadOneCore(db, ep.Who, ep.TypeString, id, ep.URLParams)
	if err != nil {
		return nil, 0, &webrender.RetError{Error: err}
	}

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Cargo: cargo}
	initData := hook.InitData{Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Ep: ep}

	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// fetch all handlers with before hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "A") {
		if retErr := hdlr.(hook.IAfter).After(&data, ep); retErr != nil {
			return nil, 0, retErr
		}
	}

	ret := MapperRet{
		Ms:      []mdl.IModel{modelObj},
		Fetcher: fetcher,
	}

	return &ret, role, nil
}

func (mapper *UserMapper) Update(db *gorm.DB, modelObjs []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	if ep.Cardinality == rest.CardinalityMany {
		err := fmt.Errorf("CardinalityOne endpoint not supported")
		renderer := webrender.NewErrPermissionDeniedForAPIEndpoint(err)
		return nil, webrender.NewRetValWithRendererError(err, renderer)
	}

	// load old model data
	ids := make([]*datatype.UUID, len(modelObjs))
	for i, modelObj := range modelObjs {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		id := modelObj.GetID()
		if id == nil || id.String() == "" {
			return nil, &webrender.RetError{Error: service.ErrIDEmpty}
		}
		ids[i] = id
	}

	// oldModelObj, _, err := loadAndCheckErrorBeforeModifyV2(mapper.Service, db, ep.Who, ep.TypeString, modelObj, id, []userrole.UserRole{userrole.UserRoleAdmin}, ep.URLParams)
	// if err != nil {
	// 	if err.Error() == "record not found" {
	// 		return nil, webrender.NewRetValWithRendererError(err, webrender.NewErrNotFound(err))
	// 	}

	// 	return nil, &webrender.RetError{Error: err}
	// }

	var rolesToErrMap = make(map[userrole.UserRole]*webrender.RetError)
	rolesToErrMap[userrole.UserRoleAdmin] = nil // at least contain this
	if registry.RoleSorter != nil {
		var err error
		rolesToErrMap, err = registry.RoleSorter.Permitted(mapper.MapperType, ep)
		if err != nil {
			return nil, webrender.NewRetValWithError(err)
		}
	}

	oldModelObjs, roles, err2 := loadManyAndCheckBeforeModifyV3(mapper.Service, db, ep.Who, ep.TypeString, ids, ep.URLParams, rolesToErrMap)
	if err2 != nil {
		if err2.Error.Error() == "record not found" {
			return nil, webrender.NewRetValWithRendererError(err2.Error, webrender.NewErrNotFound(err2.Error))
		}
		return nil, err2
	}

	// There is only one model and one role

	data := hook.Data{Ms: modelObjs, DB: db, Roles: roles, Cargo: cargo}
	initData := hook.InitData{Roles: roles, Ep: ep}

	j := batchOpJob{
		serv:         mapper.Service,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
		fetcher:      hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:         &data,
		ep:           ep,
	}
	return batchOpCore(j, mapper.Service.UpdateOneCore)

	// data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Cargo: cargo}
	// initData := hook.InitData{Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Ep: ep}

	// j := opJob{
	// 	serv:        mapper.Service,
	// 	oldModelObj: oldModelObjs,
	// 	modelObj:    modelObj,
	// 	fetcher:     hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
	// 	data:        &data,
	// 	ep:          ep,
	// }
	// return opCore(j, mapper.Service.UpdateOneCore)
}

// PatchOne updates model based on this json
func (mapper *UserMapper) Patch(db *gorm.DB, jsonIDPatches []mdlutil.JSONIDPatch, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	// func (mapper *UserMapper) PatchOne(db *gorm.DB, jsonPatch []byte, id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {

	if ep.Cardinality == rest.CardinalityMany {
		err := fmt.Errorf("CardinalityOne endpoint not supported")
		renderer := webrender.NewErrPermissionDeniedForAPIEndpoint(err)
		return nil, webrender.NewRetValWithRendererError(err, renderer)
	}

	// Load data, patch it, then send it to the hookpoint
	// Load IDs
	ids := make([]*datatype.UUID, len(jsonIDPatches))
	for i, jsonIDPatch := range jsonIDPatches {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		if jsonIDPatch.ID.String() == "" {
			return nil, &webrender.RetError{Error: service.ErrIDEmpty}
		}
		ids[i] = jsonIDPatch.ID
	}

	// mapper.Service.PermissionAndRole()
	// It's possible that hook want us to reject this endpoint
	rolesToErrMap, err := registry.RoleSorter.Permitted(mapper.MapperType, ep)
	if err != nil {
		return nil, webrender.NewRetValWithError(err)
	}

	oldModelObjs, roles, err2 := loadManyAndCheckBeforeModifyV3(mapper.Service, db, ep.Who, ep.TypeString, ids, ep.URLParams, rolesToErrMap)
	if err2 != nil {
		return nil, err2
	}

	// oldModelObj, role, err := loadAndCheckErrorBeforeModifyV2(mapper.Service, db, ep.Who, ep.TypeString, nil, id, []userrole.UserRole{userrole.UserRoleAdmin}, ep.URLParams)
	// if err != nil {
	// 	if err.Error() == "record not found" {
	// 		return nil, webrender.NewRetValWithRendererError(err, webrender.NewErrNotFound(err))
	// 	}

	// 	return nil, &webrender.RetError{Error: err}
	// }

	initData := hook.InitData{Roles: roles, Ep: ep}
	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// here we put the oldModelObjs, not patched yet
	data := hook.Data{Ms: oldModelObjs, DB: db, Roles: roles, Cargo: cargo}

	// fetch all handlers with before hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "J") {
		if retErr := hdlr.(hook.IBeforeApply).BeforeApply(&data, ep); retErr != nil {
			return nil, retErr
		}
	}

	// Now patch it
	modelObjs := make([]mdl.IModel, len(oldModelObjs))
	for i, jsonIDPatch := range jsonIDPatches {
		// Apply patch operations
		modelObjs[i], err = applyPatchCore(ep.TypeString, oldModelObjs[i], []byte(jsonIDPatch.Patch))
		if err != nil {
			return nil, &webrender.RetError{Error: err}
		}
	}

	// Validation is done here, maybe this should go into mapper as well
	modelObj := modelObjs[0]
	err = mdl.Validate.Struct(modelObj)
	if errs, ok := err.(validator.ValidationErrors); ok {
		s, err2 := mdl.TranslateValidationErrorMessage(errs, modelObj)
		if err2 != nil {
			log.Println("error translating validation message:", err)
		}
		err = errors.New(s)
		return nil, webrender.NewRetValWithError(err)
	}

	// here we put the new model objs (this should modify the one already in hook)
	data.Ms = modelObjs

	// Finally update them
	j := batchOpJob{
		serv:         mapper.Service,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
		fetcher:      fetcher,
		data:         &data,
		ep:           ep,
	}
	return batchOpCore(j, mapper.Service.UpdateOneCore)

	// initData := hook.InitData{Roles: []userrole.UserRole{role}, Ep: ep}
	// fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// data := hook.Data{Ms: nil, DB: db, Roles: []userrole.UserRole{role}, Cargo: cargo}

	// for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "J") {
	// 	if retErr := hdlr.(hook.IBeforeApply).BeforeApply(&data, ep); retErr != nil {
	// 		return nil, retErr
	// 	}
	// }

	// // Apply patch operations
	// modelObj, err := applyPatchCore(ep.TypeString, oldModelObj, jsonPatch)
	// if err != nil {
	// 	return nil, &webrender.RetError{Error: err}
	// }

	// err = mdl.Validate.Struct(modelObj)
	// if errs, ok := err.(validator.ValidationErrors); ok {
	// 	s, err2 := mdl.TranslateValidationErrorMessage(errs, modelObj)
	// 	if err2 != nil {
	// 		log.Println("error translating validation message:", err)
	// 	}
	// 	err = errors.New(s)
	// 	return nil, webrender.NewRetValWithError(err)
	// }

	// data.Ms = []mdl.IModel{modelObj}

	// j := opJob{
	// 	serv: mapper.Service,

	// 	oldModelObj: oldModelObj,
	// 	modelObj:    modelObj,

	// 	fetcher: fetcher,
	// 	data:    &data,
	// 	ep:      ep,
	// }
	// return opCore(j, mapper.Service.UpdateOneCore)
}

// DeleteOne deletes the user with the ID
func (mapper *UserMapper) DeleteOne(db *gorm.DB, id *datatype.UUID,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {

	var rolesToErrMap = make(map[userrole.UserRole]*webrender.RetError)
	rolesToErrMap[userrole.UserRoleAdmin] = nil // at least contain this
	if registry.RoleSorter != nil {
		var err error
		rolesToErrMap, err = registry.RoleSorter.Permitted(mapper.MapperType, ep)
		if err != nil {
			return nil, webrender.NewRetValWithError(err)
		}
	}

	modelObjs, roles, err2 := loadManyAndCheckBeforeModifyV3(mapper.Service, db, ep.Who, ep.TypeString, []*datatype.UUID{id}, ep.URLParams, rolesToErrMap)
	if err2 != nil {
		if err2.Error.Error() == "record not found" {
			return nil, webrender.NewRetValWithRendererError(err2.Error, webrender.NewErrNotFound(err2.Error))
		}
		return nil, err2
	}

	modelObj := modelObjs[0]
	role := roles[0]

	// modelObj, role, err := loadAndCheckErrorBeforeModifyV2(mapper.Service, db, ep.Who, ep.TypeString, nil, id, []userrole.UserRole{userrole.UserRoleAdmin}, ep.URLParams)
	// if err != nil {
	// 	if err.Error() == "record not found" {
	// 		return nil, webrender.NewRetValWithRendererError(err, webrender.NewErrNotFound(err))
	// 	}
	// 	return nil, &webrender.RetError{Error: err}
	// }

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	var err error
	modelObj, err = mapper.Service.HookBeforeDeleteOne(db, ep.Who, ep.TypeString, modelObj)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{role}, Cargo: cargo}
	initData := hook.InitData{Roles: []userrole.UserRole{role}, Ep: ep}

	j := batchOpJob{
		serv:      mapper.Service,
		modelObjs: []mdl.IModel{modelObj},
		fetcher:   hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:      &data,
		ep:        ep,
	}
	return batchOpCore(j, mapper.Service.DeleteOneCore)
}

// CreateMany :-
func (mapper *UserMapper) CreateMany(db *gorm.DB,
	modelObj []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("not implemented"))
}

// ReadMany :-
func (mapper *UserMapper) ReadMany(db *gorm.DB, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, []userrole.UserRole, *int, *webrender.RetError) {
	return nil, nil, nil, webrender.NewRetValWithError(fmt.Errorf("not implemented"))
}

// UpdateMany :-
func (mapper *UserMapper) UpdateMany(db *gorm.DB,
	modelObjs []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("not implemented"))
}

// PatchMany :-
func (mapper *UserMapper) PatchMany(db *gorm.DB,
	jsonIDPatches []mdlutil.JSONIDPatch, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("not implemented"))
}

// DeleteMany :-
func (mapper *UserMapper) DeleteMany(db *gorm.DB,
	modelObjs []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("not implemented"))
}
