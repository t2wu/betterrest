package datamapper

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/t2wu/betterrest/datamapper/hfetcher"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry"

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
func (mapper *UserMapper) CreateOne(db *gorm.DB, modelObj models.IModel, ep *hook.EndPoint,
	cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	modelObj, err := mapper.Service.HookBeforeCreateOne(db, ep.Who, ep.TypeString, modelObj)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(models.IBeforeCreate); ok {
		*beforeFuncName = "BeforeCreateDB"
	}
	if _, ok := modelObj.(models.IAfterCreate); ok {
		*afterFuncName = "AfterCreateDB"
	}

	data := hook.Data{Ms: []models.IModel{modelObj}, DB: db, Roles: []models.UserRole{models.UserRoleAdmin}, Cargo: cargo}
	initData := hook.InitData{Roles: []models.UserRole{models.UserRoleAdmin}, Ep: ep}

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
		return modelObj2, retval
	}

	return modelObj2, nil
}

// // CreateMany is currently a dummy
// func (mapper *UserMapper) CreateMany(db *gorm.DB,  modelObjs []models.IModel) ([]models.IModel, error) {
// 	// not really implemented
// 	return nil, errors.New("not implemented")
// }

// ReadOne get one model object based on its type and its id string
func (mapper *UserMapper) ReadOne(db *gorm.DB, id *datatypes.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, models.UserRole, *webrender.RetError) {
	modelObj, role, err := mapper.Service.ReadOneCore(db, ep.Who, ep.TypeString, id)
	if err != nil {
		return nil, 0, &webrender.RetError{Error: err}
	}

	data := hook.Data{Ms: []models.IModel{modelObj}, DB: db, Roles: []models.UserRole{models.UserRoleAdmin}, Cargo: cargo}
	initData := hook.InitData{Roles: []models.UserRole{models.UserRoleAdmin}, Ep: ep}

	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// Begin deprecated
	if !fetcher.HasAttemptRegisteringHandler() {
		modelCargo := models.ModelCargo{Payload: cargo.Payload}
		// After CRUPD hook
		if m, ok := modelObj.(models.IAfterCRUPD); ok {
			hpdata := models.HookPointData{DB: db, Who: ep.Who, TypeString: ep.TypeString, Cargo: &modelCargo, Role: &role, URLParams: ep.URLParams}
			m.AfterCRUPDDB(hpdata, models.CRUPDOpRead)
		}

		if m, ok := modelObj.(models.IAfterRead); ok {
			hpdata := models.HookPointData{DB: db, Who: ep.Who, TypeString: ep.TypeString, Role: &role,
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
		Ms:      []models.IModel{modelObj},
		Fetcher: fetcher,
	}

	return &ret, role, nil
}

// UpdateOne updates model based on this json
func (mapper *UserMapper) UpdateOne(db *gorm.DB, modelObj models.IModel, id *datatypes.UUID,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {

	oldModelObj, _, err := loadAndCheckErrorBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, modelObj, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	data := hook.Data{Ms: []models.IModel{modelObj}, DB: db, Roles: []models.UserRole{models.UserRoleAdmin}, Cargo: cargo}
	initData := hook.InitData{Roles: []models.UserRole{models.UserRoleAdmin}, Ep: ep}

	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(models.IBeforeUpdate); ok {
		b := "BeforeUpdateDB"
		beforeFuncName = &b
	}
	if _, ok := modelObj.(models.IAfterUpdate); ok {
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
	id *datatypes.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	oldModelObj, role, err := loadAndCheckErrorBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, nil, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	initData := hook.InitData{Roles: []models.UserRole{role}, Ep: ep}
	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// Deprecated
	if !fetcher.HasAttemptRegisteringHandler() {
		modelCargo := models.ModelCargo{Payload: cargo.Payload}
		if m, ok := oldModelObj.(models.IBeforePatchApply); ok {
			hpdata := models.HookPointData{DB: db, Who: ep.Who, TypeString: ep.TypeString, Cargo: &modelCargo}
			if err := m.BeforePatchApplyDB(hpdata); err != nil {
				return nil, &webrender.RetError{Error: err}
			}
		}
		cargo.Payload = modelCargo.Payload
	}
	// End deprecated

	data := hook.Data{Ms: nil, DB: db, Roles: []models.UserRole{role}, Cargo: cargo}

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

	err = models.Validate.Struct(modelObj)
	if errs, ok := err.(validator.ValidationErrors); ok {
		s, err2 := models.TranslateValidationErrorMessage(errs, modelObj)
		if err2 != nil {
			log.Println("error translating validation message:", err)
		}
		err = errors.New(s)
	}

	data.Ms = []models.IModel{modelObj}

	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(models.IBeforePatch); ok {
		b := "BeforePatchDB"
		beforeFuncName = &b
	}

	if _, ok := modelObj.(models.IAfterPatch); ok {
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
func (mapper *UserMapper) DeleteOne(db *gorm.DB, id *datatypes.UUID,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	modelObj, role, err := loadAndCheckErrorBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, nil, id, []models.UserRole{models.UserRoleAdmin})
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

	data := hook.Data{Ms: []models.IModel{modelObj}, DB: db, Roles: []models.UserRole{role}, Cargo: cargo}
	initData := hook.InitData{Roles: []models.UserRole{role}, Ep: ep}

	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(models.IBeforeDelete); ok {
		b := "BeforeDeleteDB"
		beforeFuncName = &b
	}
	if _, ok := modelObj.(models.IAfterDelete); ok {
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
	modelObj []models.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}

// ReadMany :-
func (mapper *UserMapper) ReadMany(db *gorm.DB, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, []models.UserRole, *int, *webrender.RetError) {
	return nil, nil, nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}

// UpdateMany :-
func (mapper *UserMapper) UpdateMany(db *gorm.DB,
	modelObjs []models.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}

// PatchMany :-
func (mapper *UserMapper) PatchMany(db *gorm.DB,
	jsonIDPatches []models.JSONIDPatch, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}

// DeleteMany :-
func (mapper *UserMapper) DeleteMany(db *gorm.DB,
	modelObjs []models.IModel, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}
