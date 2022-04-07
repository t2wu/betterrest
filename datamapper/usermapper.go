package datamapper

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/go-playground/validator/v10"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/models"

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
	options *map[urlparam.Param]interface{}, cargo *models.ModelCargo) (models.IModel, error) {
	modelObj, err := mapper.Service.HookBeforeCreateOne(db, who, typeString, modelObj)
	if err != nil {
		return nil, err
	}

	var before, after *string
	if _, ok := modelObj.(models.IBeforeCreate); ok {
		*before = "BeforeCreateDB"
	}
	if _, ok := modelObj.(models.IAfterCreate); ok {
		*after = "AfterCreateDB"
	}
	j := opJob{
		serv:       mapper.Service,
		db:         db,
		who:        who,
		typeString: typeString,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
		crupdOp:  models.CRUPDOpCreate,
		cargo:    cargo,
		options:  options,
	}

	modelObj2, err := opCore(before, after, j, mapper.Service.CreateOneCore)
	if err != nil {
		return modelObj2, err
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
	id *datatypes.UUID, options *map[urlparam.Param]interface{}, cargo *models.ModelCargo) (models.IModel, models.UserRole, error) {
	modelObj, role, err := mapper.Service.ReadOneCore(db, who, typeString, id)
	if err != nil {
		return nil, 0, err
	}

	// After CRUPD hook
	if m, ok := modelObj.(models.IAfterCRUPD); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: cargo, Role: &role, URLParams: options}
		m.AfterCRUPDDB(hpdata, models.CRUPDOpRead)
	}

	if m, ok := modelObj.(models.IAfterRead); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Role: &role,
			Cargo: cargo}
		if err := m.AfterReadDB(hpdata); err != nil {
			return nil, 0, err
		}
	}

	return modelObj, role, nil
}

// UpdateOne updates model based on this json
func (mapper *UserMapper) UpdateOne(db *gorm.DB, who models.UserIDFetchable, typeString string,
	modelObj models.IModel, id *datatypes.UUID, options *map[urlparam.Param]interface{},
	cargo *models.ModelCargo) (models.IModel, error) {

	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, modelObj, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	var before, after *string
	if _, ok := modelObj.(models.IBeforeUpdate); ok {
		b := "BeforeUpdateDB"
		before = &b
	}
	if _, ok := modelObj.(models.IAfterUpdate); ok {
		a := "AfterUpdateDB"
		after = &a
	}

	j := opJob{
		serv:        mapper.Service,
		db:          db,
		who:         who,
		typeString:  typeString,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,
		crupdOp:     models.CRUPDOpUpdate,
		cargo:       cargo,
		options:     options,
	}
	return opCore(before, after, j, mapper.Service.UpdateOneCore)
}

// PatchOne updates model based on this json
func (mapper *UserMapper) PatchOne(db *gorm.DB, who models.UserIDFetchable, typeString string, jsonPatch []byte,
	id *datatypes.UUID, options *map[urlparam.Param]interface{}, cargo *models.ModelCargo) (models.IModel, error) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, nil, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	if m, ok := oldModelObj.(models.IBeforePatchApply); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: cargo}
		if err := m.BeforePatchApplyDB(hpdata); err != nil {
			return nil, err
		}
	}

	// Apply patch operations
	modelObj, err := applyPatchCore(typeString, oldModelObj, jsonPatch)
	if err != nil {
		return nil, err
	}

	err = models.Validate.Struct(modelObj)
	if errs, ok := err.(validator.ValidationErrors); ok {
		s, err2 := models.TranslateValidationErrorMessage(errs, modelObj)
		if err2 != nil {
			log.Println("error translating validation message:", err)
		}
		err = errors.New(s)
	}

	var before, after *string
	if _, ok := modelObj.(models.IBeforePatch); ok {
		b := "BeforePatchDB"
		before = &b
	}

	if _, ok := modelObj.(models.IAfterPatch); ok {
		a := "AfterPatchDB"
		after = &a
	}

	j := opJob{
		serv:        mapper.Service,
		db:          db,
		who:         who,
		typeString:  typeString,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,
		crupdOp:     models.CRUPDOpPatch,
		cargo:       cargo,
		options:     options,
	}
	return opCore(before, after, j, mapper.Service.UpdateOneCore)
}

// DeleteOne deletes the user with the ID
func (mapper *UserMapper) DeleteOne(db *gorm.DB, who models.UserIDFetchable, typeString string, id *datatypes.UUID,
	options *map[urlparam.Param]interface{}, cargo *models.ModelCargo) (models.IModel, error) {
	modelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, nil, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	modelObj, err = mapper.Service.HookBeforeDeleteOne(db, who, typeString, modelObj)
	if err != nil {
		return nil, err
	}

	var before, after *string
	if _, ok := modelObj.(models.IBeforeDelete); ok {
		b := "BeforeDeleteDB"
		before = &b
	}
	if _, ok := modelObj.(models.IAfterDelete); ok {
		a := "AfterDeleteDB"
		after = &a
	}

	j := opJob{
		serv:       mapper.Service,
		db:         db,
		who:        who,
		typeString: typeString,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
		crupdOp:  models.CRUPDOpDelete,
		cargo:    cargo,
		options:  options,
	}

	return opCore(before, after, j, mapper.Service.DeleteOneCore)
}

// CreateMany :-
func (mapper *UserMapper) CreateMany(db *gorm.DB, who models.UserIDFetchable, typeString string,
	modelObj []models.IModel, options *map[urlparam.Param]interface{}, cargo *models.BatchHookCargo) ([]models.IModel, error) {
	return nil, fmt.Errorf("Not implemented")
}

// ReadMany :-
func (mapper *UserMapper) ReadMany(db *gorm.DB, who models.UserIDFetchable, typeString string,
	options *map[urlparam.Param]interface{}, cargo *models.BatchHookCargo) ([]models.IModel, []models.UserRole, *int, error) {
	return nil, nil, nil, fmt.Errorf("Not implemented")
}

// UpdateMany :-
func (mapper *UserMapper) UpdateMany(db *gorm.DB, who models.UserIDFetchable, typeString string,
	modelObjs []models.IModel, options *map[urlparam.Param]interface{}, cargo *models.BatchHookCargo) ([]models.IModel, error) {
	return nil, fmt.Errorf("Not implemented")
}

// PatchMany :-
func (mapper *UserMapper) PatchMany(db *gorm.DB, who models.UserIDFetchable,
	typeString string, jsonIDPatches []models.JSONIDPatch, options *map[urlparam.Param]interface{},
	cargo *models.BatchHookCargo) ([]models.IModel, error) {
	return nil, fmt.Errorf("Not implemented")
}

// DeleteMany :-
func (mapper *UserMapper) DeleteMany(db *gorm.DB, who models.UserIDFetchable,
	typeString string, modelObjs []models.IModel, options *map[urlparam.Param]interface{},
	cargo *models.BatchHookCargo) ([]models.IModel, error) {
	return nil, fmt.Errorf("Not implemented")
}
