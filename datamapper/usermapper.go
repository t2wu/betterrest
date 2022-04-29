package datamapper

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/t2wu/betterrest/controller"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
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
	Service service.IService
}

// SharedUserMapper creats a singleton of Crud object
func SharedUserMapper() *UserMapper {
	onceUser.Do(func() {
		usercrud = &UserMapper{Service: &service.UserService{BaseService: service.BaseService{}}}
	})

	return usercrud
}

//------------------------
// User specific CRUD
// Cuz user is spcial, need to create ownership and no need to check for owner
// ------------------------------------

// CreateOne creates an user model based on json and store it in db
// Also creates a ownership with admin access
func (mapper *UserMapper) CreateOne(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel,
	options map[urlparam.Param]interface{}, cargo *controller.Cargo) (*MapperRet, *webrender.RetError) {
	modelObj, err := mapper.Service.HookBeforeCreateOne(db, who, typeString, modelObj)
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

	data := controller.Data{Ms: []models.IModel{modelObj}, DB: db, Who: who,
		TypeString: typeString, Roles: []models.UserRole{models.UserRoleAdmin}, URLParams: options, Cargo: cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpCreate,
		Cardinality: controller.APICardinalityOne,
	}

	j := opJob{
		serv: mapper.Service,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,

		beforeFuncName: beforeFuncName,
		afterFuncName:  afterFuncName,

		fetcher: NewCtrlFetcher(registry.ModelRegistry[typeString].ControllerMap),
		data:    &data,
		info:    &info,
	}

	modelObj2, retval := opCore(j, mapper.Service.CreateOneCore)
	if retval != nil {
		return modelObj2, retval
	}

	return modelObj2, nil
}

// // CreateMany is currently a dummy
// func (mapper *UserMapper) CreateMany(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
// 	// not really implemented
// 	return nil, errors.New("not implemented")
// }

// ReadOne get one model object based on its type and its id string
func (mapper *UserMapper) ReadOne(db *gorm.DB, who models.UserIDFetchable, typeString string,
	id *datatypes.UUID, options map[urlparam.Param]interface{}, cargo *controller.Cargo) (*MapperRet, models.UserRole, *webrender.RetError) {
	modelObj, role, err := mapper.Service.ReadOneCore(db, who, typeString, id)
	if err != nil {
		return nil, 0, &webrender.RetError{Error: err}
	}

	data := controller.Data{Ms: []models.IModel{modelObj}, DB: db, Who: who,
		TypeString: typeString, Roles: []models.UserRole{models.UserRoleAdmin}, URLParams: options, Cargo: cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpRead,
		Cardinality: controller.APICardinalityOne,
	}

	fetcher := NewCtrlFetcher(registry.ModelRegistry[typeString].ControllerMap)

	// Begin deprecated
	if !fetcher.HasRegisteredController() {
		modelCargo := models.ModelCargo{Payload: cargo.Payload}
		// After CRUPD hook
		if m, ok := modelObj.(models.IAfterCRUPD); ok {
			hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: &modelCargo, Role: &role, URLParams: options}
			m.AfterCRUPDDB(hpdata, models.CRUPDOpRead)
		}

		if m, ok := modelObj.(models.IAfterRead); ok {
			hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Role: &role,
				Cargo: &modelCargo}
			if err := m.AfterReadDB(hpdata); err != nil {
				return nil, 0, &webrender.RetError{Error: err}
			}
		}
		cargo.Payload = modelCargo.Payload
	}
	// End deprecated

	// fetch all controllers with before hooks
	for _, ctrl := range fetcher.FetchControllersForOpAndHook(info.Op, "A") {
		if retErr := ctrl.(controller.IAfter).After(&data, &info); retErr != nil {
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
func (mapper *UserMapper) UpdateOne(db *gorm.DB, who models.UserIDFetchable, typeString string,
	modelObj models.IModel, id *datatypes.UUID, options map[urlparam.Param]interface{},
	cargo *controller.Cargo) (*MapperRet, *webrender.RetError) {

	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, modelObj, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	data := controller.Data{Ms: []models.IModel{modelObj}, DB: db, Who: who,
		TypeString: typeString, Roles: []models.UserRole{models.UserRoleAdmin}, URLParams: options, Cargo: cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpUpdate,
		Cardinality: controller.APICardinalityOne,
	}

	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(models.IBeforeUpdate); ok {
		b := "BeforeUpdateDB"
		beforeFuncName = &b
	}
	if _, ok := modelObj.(models.IAfterUpdate); ok {
		a := "AfterUpdateDB"
		afterFuncName = &a
	}

	j := opJob{
		serv: mapper.Service,

		oldModelObj: oldModelObj,
		modelObj:    modelObj,

		beforeFuncName: beforeFuncName,
		afterFuncName:  afterFuncName,

		fetcher: NewCtrlFetcher(registry.ModelRegistry[typeString].ControllerMap),
		data:    &data,
		info:    &info,
	}
	return opCore(j, mapper.Service.UpdateOneCore)
}

// PatchOne updates model based on this json
func (mapper *UserMapper) PatchOne(db *gorm.DB, who models.UserIDFetchable, typeString string, jsonPatch []byte,
	id *datatypes.UUID, options map[urlparam.Param]interface{}, cargo *controller.Cargo) (*MapperRet, *webrender.RetError) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, nil, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	fetcher := NewCtrlFetcher(registry.ModelRegistry[typeString].ControllerMap)

	// Deprecated
	if !fetcher.HasRegisteredController() {
		modelCargo := models.ModelCargo{Payload: cargo.Payload}
		if m, ok := oldModelObj.(models.IBeforePatchApply); ok {
			hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: &modelCargo}
			if err := m.BeforePatchApplyDB(hpdata); err != nil {
				return nil, &webrender.RetError{Error: err}
			}
		}
		cargo.Payload = modelCargo.Payload
	}
	// End deprecated

	data := controller.Data{Ms: nil, DB: db, Who: who,
		TypeString: typeString, Roles: []models.UserRole{models.UserRoleAdmin}, URLParams: options, Cargo: cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpPatch,
		Cardinality: controller.APICardinalityOne,
	}

	for _, ctrl := range fetcher.FetchControllersForOpAndHook(info.Op, "J") {
		if retErr := ctrl.(controller.IBeforeApply).BeforeApply(&data, &info); retErr != nil {
			return nil, retErr
		}
	}

	// Apply patch operations
	modelObj, err := applyPatchCore(typeString, oldModelObj, jsonPatch)
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

	j := opJob{
		serv: mapper.Service,

		oldModelObj: oldModelObj,
		modelObj:    modelObj,

		beforeFuncName: beforeFuncName,
		afterFuncName:  afterFuncName,

		fetcher: fetcher,
		data:    &data,
		info:    &info,
	}
	return opCore(j, mapper.Service.UpdateOneCore)
}

// DeleteOne deletes the user with the ID
func (mapper *UserMapper) DeleteOne(db *gorm.DB, who models.UserIDFetchable, typeString string, id *datatypes.UUID,
	options map[urlparam.Param]interface{}, cargo *controller.Cargo) (*MapperRet, *webrender.RetError) {
	modelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, nil, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	modelObj, err = mapper.Service.HookBeforeDeleteOne(db, who, typeString, modelObj)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	data := controller.Data{Ms: []models.IModel{modelObj}, DB: db, Who: who,
		TypeString: typeString, Roles: []models.UserRole{models.UserRoleAdmin}, URLParams: options, Cargo: cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpDelete,
		Cardinality: controller.APICardinalityOne,
	}

	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(models.IBeforeDelete); ok {
		b := "BeforeDeleteDB"
		beforeFuncName = &b
	}
	if _, ok := modelObj.(models.IAfterDelete); ok {
		a := "AfterDeleteDB"
		afterFuncName = &a
	}

	j := opJob{
		serv: mapper.Service,

		// oldModelObj: oldModelObj,
		modelObj: modelObj,

		beforeFuncName: beforeFuncName,
		afterFuncName:  afterFuncName,

		fetcher: NewCtrlFetcher(registry.ModelRegistry[typeString].ControllerMap),
		data:    &data,
		info:    &info,
	}

	return opCore(j, mapper.Service.DeleteOneCore)
}

// CreateMany :-
func (mapper *UserMapper) CreateMany(db *gorm.DB, who models.UserIDFetchable, typeString string,
	modelObj []models.IModel, options map[urlparam.Param]interface{}, cargo *controller.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}

// ReadMany :-
func (mapper *UserMapper) ReadMany(db *gorm.DB, who models.UserIDFetchable, typeString string,
	options map[urlparam.Param]interface{}, cargo *controller.Cargo) (*MapperRet, []models.UserRole, *int, *webrender.RetError) {
	return nil, nil, nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}

// UpdateMany :-
func (mapper *UserMapper) UpdateMany(db *gorm.DB, who models.UserIDFetchable, typeString string,
	modelObjs []models.IModel, options map[urlparam.Param]interface{}, cargo *controller.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}

// PatchMany :-
func (mapper *UserMapper) PatchMany(db *gorm.DB, who models.UserIDFetchable,
	typeString string, jsonIDPatches []models.JSONIDPatch, options map[urlparam.Param]interface{},
	cargo *controller.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}

// DeleteMany :-
func (mapper *UserMapper) DeleteMany(db *gorm.DB, who models.UserIDFetchable,
	typeString string, modelObjs []models.IModel, options map[urlparam.Param]interface{},
	cargo *controller.Cargo) (*MapperRet, *webrender.RetError) {
	return nil, webrender.NewRetValWithError(fmt.Errorf("Not implemented"))
}
