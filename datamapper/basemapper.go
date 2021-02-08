package datamapper

import (
	"errors"
	"log"
	"time"

	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

// IDataMapper has all the crud interfaces
type IDataMapper interface {
	CreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error)

	CreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj []models.IModel) ([]models.IModel, error)

	GetOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error)

	GetAll(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, options map[URLParam]interface{}) ([]models.IModel, []models.UserRole, *int, error)

	UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, modelobj models.IModel, id *datatypes.UUID) (models.IModel, error)

	UpdateMany(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, modelObjs []models.IModel) ([]models.IModel, error)

	PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, jsonPatch []byte, id *datatypes.UUID) (models.IModel, error)

	PatchMany(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, jsonIDPatches []models.JSONIDPatch) ([]models.IModel, error)

	DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, id *datatypes.UUID) (models.IModel, error)

	DeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, modelObjs []models.IModel) ([]models.IModel, error)
}

//------------------------------------
// User model only
//------------------------------------

// IChangeEmailPasswordMapper changes email and password
type IChangeEmailPasswordMapper interface {
	ChangeEmailPasswordWithID(db *gorm.DB, oid *datatypes.UUID, scope *string,
		typeString string, modelobj models.IModel, id *datatypes.UUID) (models.IModel, error)
}

// -----------------------------------
// Base mapper
// -----------------------------------

// BaseMapper is a basic CRUD manager
type BaseMapper struct {
	Service service.IService
}

// CreateOne creates an instance of this model based on json and store it in db
func (mapper *BaseMapper) CreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	modelObj, err := mapper.Service.HookBeforeCreateOne(db, oid, scope, typeString, modelObj)
	if err != nil {
		return nil, err
	}

	var before, after *string
	if _, ok := modelObj.(models.IBeforeCreate); ok {
		b := "BeforeInsertDB"
		before = &b
	}
	if _, ok := modelObj.(models.IAfterCreate); ok {
		a := "AfterInsertDB"
		after = &a
	}

	j := opJob{
		serv:       mapper.Service,
		db:         db,
		oid:        oid,
		scope:      scope,
		typeString: typeString,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
	}
	return opCore(before, after, j, mapper.Service.CreateOneCore)
}

// CreateMany creates an instance of this model based on json and store it in db
func (mapper *BaseMapper) CreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	modelObjs, err := mapper.Service.HookBeforeCreateMany(db, oid, scope, typeString, modelObjs)
	if err != nil {
		return nil, err
	}

	before := models.ModelRegistry[typeString].BeforeInsert
	after := models.ModelRegistry[typeString].AfterInsert
	j := batchOpJob{
		serv:         mapper.Service,
		db:           db,
		oid:          oid,
		scope:        scope,
		typeString:   typeString,
		oldmodelObjs: nil,
		modelObjs:    modelObjs,
	}
	return batchOpCore(j, before, after, mapper.Service.CreateOneCore)
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *BaseMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	// anyone permission can read as long as you are linked on db
	modelObj, role, err := loadAndCheckErrorBeforeModify(mapper.Service, db, oid, scope, typeString, nil, id, []models.UserRole{models.UserRoleAny})
	if err != nil {
		return nil, models.Invalid, err
	}

	if m, ok := modelObj.(models.IAfterRead); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Role: &role}
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
func (mapper *BaseMapper) GetAll(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, options map[URLParam]interface{}) ([]models.IModel, []models.UserRole, *int, error) {
	dbClean := db
	db = db.Set("gorm:auto_preload", true)

	offset, limit, cstart, cstop, order, latestn, totalcount := getOptions(options)
	rtable := models.GetTableNameFromTypeString(typeString)

	if cstart != nil && cstop != nil {
		db = db.Where(rtable+".created_at BETWEEN ? AND ?", time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))
	}

	var err error
	db, err = constructInnerFieldParamQueries(db, typeString, options, latestn)
	if err != nil {
		return nil, nil, nil, err
	}

	db = constructOrderFieldQueries(db, rtable, order)
	db, err = mapper.Service.GetAllQueryContructCore(db, oid, scope, typeString)
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
	} else { // default to 100 maximum
		db = db.Offset(0).Limit(100)
	}

	outmodels, err := models.NewSliceFromDBByTypeString(typeString, db.Find)
	if err != nil {
		return nil, nil, nil, err
	}

	roles, err := mapper.Service.GetAllRolesCore(db, dbClean, oid, scope, typeString, outmodels)
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

	// use dbClean cuz it's not chained
	if after := models.ModelRegistry[typeString].AfterRead; after != nil {
		bhpData := models.BatchHookPointData{Ms: outmodels, DB: dbClean, OID: oid, Scope: scope, TypeString: typeString, Roles: roles}
		if err = after(bhpData); err != nil {
			return nil, nil, nil, err
		}
	}

	return outmodels, roles, no, err
}

// UpdateOneWithID updates model based on this json
func (mapper *BaseMapper) UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID) (models.IModel, error) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, oid, scope, typeString, modelObj, id, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	// TODO: Huh? How do we do validation here?!
	var before, after *string
	if _, ok := modelObj.(models.IBeforeUpdate); ok {
		b := "BeforeUpdateDB"
		before = &b
	}
	if _, ok := modelObj.(models.IBeforeUpdate); ok {
		a := "AfterUpdateDB"
		after = &a
	}

	j := opJob{
		serv:        mapper.Service,
		db:          db,
		oid:         oid,
		scope:       scope,
		typeString:  typeString,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,
	}
	return opCore(before, after, j, mapper.Service.UpdateOneCore)
}

// UpdateMany updates multiple models
func (mapper *BaseMapper) UpdateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
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

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper.Service, db, oid, scope, typeString, ids, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	before := models.ModelRegistry[typeString].BeforeUpdate
	after := models.ModelRegistry[typeString].AfterUpdate
	j := batchOpJob{
		serv:         mapper.Service,
		db:           db,
		oid:          oid,
		scope:        scope,
		typeString:   typeString,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
	}
	return batchOpCore(j, before, after, mapper.Service.UpdateOneCore)
}

// PatchOneWithID updates model based on this json
func (mapper *BaseMapper) PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonPatch []byte, id *datatypes.UUID) (models.IModel, error) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, oid, scope, typeString, nil, id, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
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
		oid:         oid,
		scope:       scope,
		typeString:  typeString,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,
	}
	return opCore(before, after, j, mapper.Service.UpdateOneCore)
}

// PatchMany patches multiple models
func (mapper *BaseMapper) PatchMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonIDPatches []models.JSONIDPatch) ([]models.IModel, error) {
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

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper.Service, db, oid, scope, typeString, ids, []models.UserRole{models.Admin})

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
		oid:          oid,
		scope:        scope,
		typeString:   typeString,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
	}
	return batchOpCore(j, before, after, mapper.Service.UpdateOneCore)
}

// DeleteOneWithID delete the model
// TODO: delete the groups associated with this record?
func (mapper *BaseMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, error) {
	modelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, oid, scope, typeString, nil, id, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	modelObj, err = mapper.Service.HookBeforeDeleteOne(db, oid, scope, typeString, modelObj)
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
		oid:        oid,
		scope:      scope,
		typeString: typeString,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
	}
	return opCore(before, after, j, mapper.Service.DeleteOneCore)
}

// DeleteMany deletes multiple models
func (mapper *BaseMapper) DeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
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

	modelObjs, _, err := loadManyAndCheckBeforeModify(mapper.Service, db, oid, scope, typeString, ids, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if len(modelObjs) > 0 && modelNeedsRealDelete(modelObjs[0]) {
		db = db.Unscoped() // hookpoint will inherit this though
	}

	modelObjs, err = mapper.Service.HookBeforeDeleteMany(db, oid, scope, typeString, modelObjs)
	if err != nil {
		return nil, err
	}

	before := models.ModelRegistry[typeString].BeforeDelete
	after := models.ModelRegistry[typeString].AfterDelete

	j := batchOpJob{
		serv:       mapper.Service,
		db:         db,
		oid:        oid,
		scope:      scope,
		typeString: typeString,
		modelObjs:  modelObjs,
	}
	return batchOpCore(j, before, after, mapper.Service.DeleteOneCore)
}

// ----------------------------------------------------------------------------------------
