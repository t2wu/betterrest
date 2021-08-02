package datamapper

import (
	"errors"
	"log"
	"time"

	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

//------------------------------------
// User model only
//------------------------------------

// IChangeEmailPasswordMapper changes email and password
type IChangeEmailPasswordMapper interface {
	ChangeEmailPasswordWithID(db *gorm.DB, who models.Who,
		typeString string, modelobj models.IModel, id *datatypes.UUID) (models.IModel, error)
}

// IEmailVerificationMapper does verification email
type IEmailVerificationMapper interface {
	SendEmailVerification(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel) error
	VerifyEmail(db *gorm.DB, typeString string, id *datatypes.UUID, code string) error
}

// IResetPasswordMapper reset password in-case user forgotten it.
type IResetPasswordMapper interface {
	SendEmailResetPassword(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel) error
	ResetPassword(db *gorm.DB, typeString string, modelObj models.IModel, id *datatypes.UUID, code string) error
}

// -----------------------------------
// Base mapper
// -----------------------------------

// BaseMapper is a basic CRUD manager
type BaseMapper struct {
	Service service.IService
}

// CreateOne creates an instance of this model based on json and store it in db
func (mapper *BaseMapper) CreateOne(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel, options *map[urlparam.Param]interface{}) (models.IModel, error) {
	modelObj, err := mapper.Service.HookBeforeCreateOne(db, who, typeString, modelObj)
	if err != nil {
		return nil, err
	}

	var before, after *string
	if _, ok := modelObj.(models.IBeforeCreate); ok {
		b := "BeforeCreateDB"
		before = &b
	}
	if _, ok := modelObj.(models.IAfterCreate); ok {
		a := "AfterCreateDB"
		after = &a
	}

	j := opJob{
		serv:       mapper.Service,
		db:         db,
		who:        who,
		typeString: typeString,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
		crupdOp:  models.CRUPDOpCreate,
		options:  options,
	}
	return opCore(before, after, j, mapper.Service.CreateOneCore)
}

// CreateMany creates an instance of this model based on json and store it in db
func (mapper *BaseMapper) CreateMany(db *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel, options *map[urlparam.Param]interface{}) ([]models.IModel, error) {
	modelObjs, err := mapper.Service.HookBeforeCreateMany(db, who, typeString, modelObjs)
	if err != nil {
		return nil, err
	}

	before := models.ModelRegistry[typeString].BeforeCreate
	after := models.ModelRegistry[typeString].AfterCreate
	j := batchOpJob{
		serv:         mapper.Service,
		db:           db,
		who:          who,
		typeString:   typeString,
		oldmodelObjs: nil,
		modelObjs:    modelObjs,
		crupdOp:      models.CRUPDOpCreate,
		options:      options,
	}
	return batchOpCore(j, before, after, mapper.Service.CreateOneCore)
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *BaseMapper) GetOneWithID(db *gorm.DB, who models.Who, typeString string, id *datatypes.UUID, options *map[urlparam.Param]interface{}) (models.IModel, models.UserRole, error) {
	// anyone permission can read as long as you are linked on db
	modelObj, role, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, nil, id, []models.UserRole{models.UserRoleAny})
	if err != nil {
		return nil, models.UserRoleInvalid, err
	}

	cargo := models.ModelCargo{}

	// After CRUPD hook
	if m, ok := modelObj.(models.IAfterCRUPD); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: &cargo, Role: &role, URLParams: options}
		m.AfterCRUPDDB(hpdata, models.CRUPDOpRead)
	}

	// AfterRead hook
	if m, ok := modelObj.(models.IAfterRead); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: &cargo, Role: &role, URLParams: options}
		if err := m.AfterReadDB(hpdata); err != nil {
			return nil, 0, err
		}
	}

	return modelObj, role, err
}

// GetAll obtains a slice of models.DomainModel
// options can be string "offset" and "limit", both of type int
// This is very Javascript-esque. I would have liked Python's optional parameter more.
// Alas, no such feature in Go. https://stackoverflow.com/questions/2032149/optional-parameters-in-go
// How does Gorm do the following? Might want to check out its source code.
// Cancel offset condition with -1
//  db.Offset(10).Find(&users1).Offset(-1).Find(&users2)
func (mapper *BaseMapper) GetAll(db *gorm.DB, who models.Who, typeString string, options *map[urlparam.Param]interface{}) ([]models.IModel, []models.UserRole, *int, error) {
	dbClean := db
	db = db.Set("gorm:auto_preload", true)

	offset, limit, cstart, cstop, order, latestn, latestnons, totalcount := urlparam.GetOptions(*options)
	rtable := models.GetTableNameFromTypeString(typeString)

	if cstart != nil && cstop != nil {
		db = db.Where(rtable+".created_at BETWEEN ? AND ?", time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))
	}

	var err error
	db, err = constructInnerFieldParamQueries(db, typeString, options, latestn, latestnons)
	if err != nil {
		return nil, nil, nil, err
	}

	db = constructOrderFieldQueries(db, rtable, order)
	db, err = mapper.Service.GetAllQueryContructCore(db, who, typeString)
	if err != nil {
		return nil, nil, nil, err
	}

	var no *int
	if totalcount {
		no = new(int)
		// Query for total count, without offset and limit (all)
		if err := db.Count(no).Error; err != nil {
			log.Println("count error:", err)
			return nil, nil, nil, err
		}
	}

	// chain offset and limit
	if offset != nil && limit != nil {
		db = db.Offset(*offset).Limit(*limit)
	} else if cstart == nil && cstop == nil { // default to 100 maximum unless time is specified
		db = db.Offset(0).Limit(100)
	}

	outmodels, err := models.NewSliceFromDBByTypeString(typeString, db.Find)
	if err != nil {
		return nil, nil, nil, err
	}

	roles, err := mapper.Service.GetAllRolesCore(db, dbClean, who, typeString, outmodels)
	if err != nil {
		return nil, nil, nil, err
	}

	// safeguard, Must be coded wrongly
	if len(outmodels) != len(roles) {
		return nil, nil, nil, errors.New("unknown query error")
	}

	// make many to many tag works
	for _, m := range outmodels {
		err = gormfixes.LoadManyToManyBecauseGormFailsWithID(dbClean, m)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	// the AfterCRUPD hookpoint
	// use dbClean cuz it's not chained
	if after := models.ModelRegistry[typeString].AfterCRUPD; after != nil {
		bhpData := models.BatchHookPointData{Ms: outmodels, DB: dbClean, Who: who, TypeString: typeString, Roles: roles, URLParams: options}
		if err = after(bhpData, models.CRUPDOpRead); err != nil {
			return nil, nil, nil, err
		}
	}

	// AfterRead hookpoint
	// use dbClean cuz it's not chained
	if after := models.ModelRegistry[typeString].AfterRead; after != nil {
		bhpData := models.BatchHookPointData{Ms: outmodels, DB: dbClean, Who: who, TypeString: typeString, Roles: roles, URLParams: options}
		if err = after(bhpData); err != nil {
			return nil, nil, nil, err
		}
	}

	return outmodels, roles, no, err
}

// UpdateOneWithID updates model based on this json
func (mapper *BaseMapper) UpdateOneWithID(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel, id *datatypes.UUID, options *map[urlparam.Param]interface{}) (models.IModel, error) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, modelObj, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	// TODO: Huh? How do we do validation here?!
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
		options:     options,
	}
	return opCore(before, after, j, mapper.Service.UpdateOneCore)
}

// UpdateMany updates multiple models
func (mapper *BaseMapper) UpdateMany(db *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel, options *map[urlparam.Param]interface{}) ([]models.IModel, error) {
	// load old model data
	ids := make([]*datatypes.UUID, len(modelObjs))
	for i, modelObj := range modelObjs {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		id := modelObj.GetID()
		if id == nil || id.String() == "" {
			return nil, service.ErrIDEmpty
		}
		ids[i] = id
	}

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper.Service, db, who, typeString, ids, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	before := models.ModelRegistry[typeString].BeforeUpdate
	after := models.ModelRegistry[typeString].AfterUpdate
	j := batchOpJob{
		serv:         mapper.Service,
		db:           db,
		who:          who,
		typeString:   typeString,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
		crupdOp:      models.CRUPDOpUpdate,
		options:      options,
	}
	return batchOpCore(j, before, after, mapper.Service.UpdateOneCore)
}

// PatchOneWithID updates model based on this json
func (mapper *BaseMapper) PatchOneWithID(db *gorm.DB, who models.Who, typeString string, jsonPatch []byte, id *datatypes.UUID, options *map[urlparam.Param]interface{}) (models.IModel, error) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, nil, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	cargo := models.ModelCargo{}
	if m, ok := oldModelObj.(models.IBeforePatchApply); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: &cargo}
		if err := m.BeforePatchApplyDB(hpdata); err != nil {
			return nil, err
		}
	}

	// Apply patch operations
	modelObj, err := applyPatchCore(typeString, oldModelObj, jsonPatch)
	if err != nil {
		return nil, err
	}

	// TODO: Huh? How do we do validation here?!
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
		cargo:       &cargo,
		options:     options,
	}
	return opCore(before, after, j, mapper.Service.UpdateOneCore)
}

// PatchMany patches multiple models
func (mapper *BaseMapper) PatchMany(db *gorm.DB, who models.Who, typeString string, jsonIDPatches []models.JSONIDPatch, options *map[urlparam.Param]interface{}) ([]models.IModel, error) {
	// Load data, patch it, then send it to the hookpoint
	// Load IDs
	ids := make([]*datatypes.UUID, len(jsonIDPatches))
	for i, jsonIDPatch := range jsonIDPatches {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		if jsonIDPatch.ID.String() == "" {
			return nil, service.ErrIDEmpty
		}
		ids[i] = jsonIDPatch.ID
	}

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper.Service, db, who, typeString, ids, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	// Hookpoint BEFORE BeforeCRUD and BeforePatch
	// This is called BEFORE the actual patch
	cargo := models.BatchHookCargo{}
	beforeApply := models.ModelRegistry[typeString].BeforePatchApply
	if beforeApply != nil {
		bhpData := models.BatchHookPointData{Ms: oldModelObjs, DB: db, Who: who, TypeString: typeString, Cargo: &cargo}
		if err := beforeApply(bhpData); err != nil {
			return nil, err
		}
	}

	// Now patch it
	modelObjs := make([]models.IModel, len(oldModelObjs))
	for i, jsonIDPatch := range jsonIDPatches {
		// Apply patch operations
		modelObjs[i], err = applyPatchCore(typeString, oldModelObjs[i], []byte(jsonIDPatch.Patch))
		if err != nil {
			log.Println("patch error: ", err, string(jsonIDPatch.Patch))
			return nil, err
		}
	}

	// Finally update them
	before := models.ModelRegistry[typeString].BeforePatch
	after := models.ModelRegistry[typeString].AfterPatch
	j := batchOpJob{
		serv:         mapper.Service,
		db:           db,
		who:          who,
		typeString:   typeString,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
		crupdOp:      models.CRUPDOpPatch,
		cargo:        &cargo,
		options:      options,
	}
	return batchOpCore(j, before, after, mapper.Service.UpdateOneCore)
}

// DeleteOneWithID delete the model
// TODO: delete the groups associated with this record?
func (mapper *BaseMapper) DeleteOneWithID(db *gorm.DB, who models.Who, typeString string, id *datatypes.UUID, options *map[urlparam.Param]interface{}) (models.IModel, error) {
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
		options:  options,
	}
	return opCore(before, after, j, mapper.Service.DeleteOneCore)
}

// DeleteMany deletes multiple models
func (mapper *BaseMapper) DeleteMany(db *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel, options *map[urlparam.Param]interface{}) ([]models.IModel, error) {
	// load old model data
	ids := make([]*datatypes.UUID, len(modelObjs))
	for i, modelObj := range modelObjs {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		id := modelObj.GetID()
		if id == nil || id.String() == "" {
			return nil, service.ErrIDEmpty
		}
		ids[i] = id
	}

	modelObjs, _, err := loadManyAndCheckBeforeModify(mapper.Service, db, who, typeString, ids, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if len(modelObjs) > 0 && modelNeedsRealDelete(modelObjs[0]) {
		db = db.Unscoped() // hookpoint will inherit this though
	}

	modelObjs, err = mapper.Service.HookBeforeDeleteMany(db, who, typeString, modelObjs)
	if err != nil {
		return nil, err
	}

	before := models.ModelRegistry[typeString].BeforeDelete
	after := models.ModelRegistry[typeString].AfterDelete

	j := batchOpJob{
		serv:       mapper.Service,
		db:         db,
		who:        who,
		typeString: typeString,
		modelObjs:  modelObjs,
		crupdOp:    models.CRUPDOpDelete,
		options:    options,
	}
	return batchOpCore(j, before, after, mapper.Service.DeleteOneCore)
}

// ----------------------------------------------------------------------------------------
