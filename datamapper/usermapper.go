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
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
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
	Service service.IServiceV1
}

// SharedUserMapper creats a singleton of Crud object
func SharedUserMapper() *UserMapper {
	onceUser.Do(func() {
		usercrud = &UserMapper{Service: &service.UserService{BaseServiceV1: service.BaseServiceV1{}}}
	})

	return usercrud
}

//------------------------
// User specific CRUD
// Cuz user is spcial, need to create ownership and no need to check for owner
// ------------------------------------

// CreateOne creates an user model based on json and store it in db
// Also creates a ownership with admin access
func (mapper *UserMapper) CreateOne(db *gorm.DB, modelObj mdl.IModel, ep *hook.EndPoint,
	cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	modelObj, err := mapper.Service.HookBeforeCreateOne(db, ep.Who, ep.TypeString, modelObj)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(mdlutil.IBeforeCreate); ok {
		*beforeFuncName = "BeforeCreateDB"
	}
	if _, ok := modelObj.(mdlutil.IAfterCreate); ok {
		*afterFuncName = "AfterCreateDB"
	}

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Cargo: cargo}
	initData := hook.InitData{Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Ep: ep}

	j := opJobV1{
		serv: mapper.Service,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,

		beforeFuncName: beforeFuncName,
		afterFuncName:  afterFuncName,

		fetcher: hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:    &data,
		ep:      ep,
	}

	modelObj2, retval := opCoreV1(j, mapper.Service.CreateOneCore)
	if retval != nil {
		if strings.Contains(retval.Error.Error(), "duplicate key") {
			err := fmt.Errorf("account already exists")
			renderer := webrender.NewErrDuplicatedRecord(err)
			return modelObj2, webrender.NewRetValWithRendererError(err, renderer)
		}
		return modelObj2, retval
	}

	return modelObj2, nil
}

// // CreateMany is currently a dummy
// func (mapper *UserMapper) CreateMany(db *gorm.DB,  modelObjs []mdl.IModel) ([]mdl.IModel, error) {
// 	// not really implemented
// 	return nil, errors.New("not implemented")
// }

// ReadOne get one model object based on its type and its id string
func (mapper *UserMapper) ReadOne(db *gorm.DB, id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, userrole.UserRole, *webrender.RetError) {
	modelObj, role, err := mapper.Service.ReadOneCore(db, ep.Who, ep.TypeString, id)
	if err != nil {
		return nil, 0, &webrender.RetError{Error: err}
	}

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Cargo: cargo}
	initData := hook.InitData{Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Ep: ep}

	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// Begin deprecated
	if !fetcher.HasAttemptRegisteringHandler() {
		modelCargo := mdlutil.ModelCargo{Payload: cargo.Payload}
		// After CRUPD hook
		if m, ok := modelObj.(mdlutil.IAfterCRUPD); ok {
			hpdata := mdlutil.HookPointData{DB: db, Who: ep.Who, TypeString: ep.TypeString, Cargo: &modelCargo, Role: &role, URLParams: ep.URLParams}
			m.AfterCRUPDDB(hpdata, mdlutil.CRUPDOpRead)
		}

		if m, ok := modelObj.(mdlutil.IAfterRead); ok {
			hpdata := mdlutil.HookPointData{DB: db, Who: ep.Who, TypeString: ep.TypeString, Role: &role,
				Cargo: &modelCargo}
			if err := m.AfterReadDB(hpdata); err != nil {
				return nil, 0, &webrender.RetError{Error: err}
			}
		}
		cargo.Payload = modelCargo.Payload
	}
	// End deprecated

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

// UpdateOne updates model based on this json
func (mapper *UserMapper) UpdateOne(db *gorm.DB, modelObj mdl.IModel, id *datatype.UUID,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {

	oldModelObj, _, err := loadAndCheckErrorBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, modelObj, id, []userrole.UserRole{userrole.UserRoleAdmin})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Cargo: cargo}
	initData := hook.InitData{Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Ep: ep}

	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(mdlutil.IBeforeUpdate); ok {
		b := "BeforeUpdateDB"
		beforeFuncName = &b
	}
	if _, ok := modelObj.(mdlutil.IAfterUpdate); ok {
		a := "AfterUpdateDB"
		afterFuncName = &a
	}

	j := opJobV1{
		serv: mapper.Service,

		oldModelObj: oldModelObj,
		modelObj:    modelObj,

		beforeFuncName: beforeFuncName,
		afterFuncName:  afterFuncName,

		fetcher: hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:    &data,
		ep:      ep,
	}
	return opCoreV1(j, mapper.Service.UpdateOneCore)
}

// PatchOne updates model based on this json
func (mapper *UserMapper) PatchOne(db *gorm.DB, jsonPatch []byte,
	id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	oldModelObj, role, err := loadAndCheckErrorBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, nil, id, []userrole.UserRole{userrole.UserRoleAdmin})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	initData := hook.InitData{Roles: []userrole.UserRole{role}, Ep: ep}
	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// Deprecated
	if !fetcher.HasAttemptRegisteringHandler() {
		modelCargo := mdlutil.ModelCargo{Payload: cargo.Payload}
		if m, ok := oldModelObj.(mdlutil.IBeforePatchApply); ok {
			hpdata := mdlutil.HookPointData{DB: db, Who: ep.Who, TypeString: ep.TypeString, Cargo: &modelCargo}
			if err := m.BeforePatchApplyDB(hpdata); err != nil {
				return nil, &webrender.RetError{Error: err}
			}
		}
		cargo.Payload = modelCargo.Payload
	}
	// End deprecated

	data := hook.Data{Ms: nil, DB: db, Roles: []userrole.UserRole{role}, Cargo: cargo}

	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "J") {
		if retErr := hdlr.(hook.IBeforeApply).BeforeApply(&data, ep); retErr != nil {
			return nil, retErr
		}
	}

	// Apply patch operations
	modelObj, err := applyPatchCore(ep.TypeString, oldModelObj, jsonPatch)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	err = mdl.Validate.Struct(modelObj)
	if errs, ok := err.(validator.ValidationErrors); ok {
		s, err2 := mdl.TranslateValidationErrorMessage(errs, modelObj)
		if err2 != nil {
			log.Println("error translating validation message:", err)
		}
		err = errors.New(s)
		return nil, webrender.NewRetValWithError(err)
	}

	data.Ms = []mdl.IModel{modelObj}

	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(mdlutil.IBeforePatch); ok {
		b := "BeforePatchDB"
		beforeFuncName = &b
	}

	if _, ok := modelObj.(mdlutil.IAfterPatch); ok {
		a := "AfterPatchDB"
		afterFuncName = &a
	}

	j := opJobV1{
		serv: mapper.Service,

		oldModelObj: oldModelObj,
		modelObj:    modelObj,

		beforeFuncName: beforeFuncName,
		afterFuncName:  afterFuncName,

		fetcher: fetcher,
		data:    &data,
		ep:      ep,
	}
	return opCoreV1(j, mapper.Service.UpdateOneCore)
}

// DeleteOne deletes the user with the ID
func (mapper *UserMapper) DeleteOne(db *gorm.DB, id *datatype.UUID,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	modelObj, role, err := loadAndCheckErrorBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, nil, id, []userrole.UserRole{userrole.UserRoleAdmin})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	modelObj, err = mapper.Service.HookBeforeDeleteOne(db, ep.Who, ep.TypeString, modelObj)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{role}, Cargo: cargo}
	initData := hook.InitData{Roles: []userrole.UserRole{role}, Ep: ep}

	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(mdlutil.IBeforeDelete); ok {
		b := "BeforeDeleteDB"
		beforeFuncName = &b
	}
	if _, ok := modelObj.(mdlutil.IAfterDelete); ok {
		a := "AfterDeleteDB"
		afterFuncName = &a
	}

	j := opJobV1{
		serv: mapper.Service,

		// oldModelObj: oldModelObj,
		modelObj: modelObj,

		beforeFuncName: beforeFuncName,
		afterFuncName:  afterFuncName,

		fetcher: hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:    &data,
		ep:      ep,
	}

	return opCoreV1(j, mapper.Service.DeleteOneCore)
}

// CreateMany :-
func (mapper *UserMapper) CreateMany(db *gorm.DB,
	modelObj []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}

// ReadMany :-
func (mapper *UserMapper) ReadMany(db *gorm.DB, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, []userrole.UserRole, *int, *webrender.RetError) {
	return nil, nil, nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}

// UpdateMany :-
func (mapper *UserMapper) UpdateMany(db *gorm.DB,
	modelObjs []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}

// PatchMany :-
func (mapper *UserMapper) PatchMany(db *gorm.DB,
	jsonIDPatches []mdlutil.JSONIDPatch, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}

// DeleteMany :-
func (mapper *UserMapper) DeleteMany(db *gorm.DB,
	modelObjs []mdl.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}
