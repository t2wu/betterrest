package datamapper

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// ---------------------------------------

var onceGlobal sync.Once
var crudGlobal *GlobalMapper

// GlobalMapper is a basic CRUD manager
type GlobalMapper struct {
	Service service.IService
}

// SharedGlobalMapper creats a singleton of Crud object
func SharedGlobalMapper() *GlobalMapper {
	onceGlobal.Do(func() {
		crudGlobal = &GlobalMapper{Service: &service.GlobalService{}}
	})

	return crudGlobal
}

// CreateOne creates an instance of this model based on json and store it in db
func (mapper *GlobalMapper) CreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	modelObj, err := mapper.Service.HookBeforeCreateOne(db, oid, scope, typeString, modelObj)
	if err != nil {
		return nil, err
	}

	var before func(hpdata models.HookPointData) error
	var after func(hpdata models.HookPointData) error
	if v, ok := modelObj.(models.IBeforeCreate); ok {
		before = v.BeforeInsertDB
	}
	if v, ok := modelObj.(models.IAfterCreate); ok {
		after = v.AfterInsertDB
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
	return opCore(before, after, j, createOneCoreBasic)
}

// CreateMany creates an instance of this model based on json and store it in db
func (mapper *GlobalMapper) CreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
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
	return batchOpCore(j, before, after, createOneCoreBasic)
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *GlobalMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
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

// GetAll is when user do a read
func (mapper *GlobalMapper) GetAll(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, options map[URLParam]interface{}) ([]models.IModel, []models.UserRole, *int, error) {
	db2 := db
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

	db = db.Table(rtable)

	db = constructOrderFieldQueries(db, rtable, order)

	var no *int
	if totalcount {
		no = new(int)
		// Query for total count, without offset and limit (all)
		if err := db.Count(no).Error; err != nil {
			log.Println("count error:", err)
			return nil, nil, nil, err
		}
	}

	if offset != nil && limit != nil {
		db = db.Offset(*offset).Limit(*limit)
	}

	outmodels, roles, err := mapper.Service.GetAllCore(db, oid, scope, typeString)
	if err != nil {
		return nil, nil, nil, err
	}

	// safeguard, Must be coded wrongly
	if len(outmodels) != len(roles) {
		return nil, nil, nil, errors.New("unknown query error")
	}

	// make many to many tag works
	for _, m := range outmodels {
		err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db2, m)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	// use db2 cuz it's not chained
	if after := models.ModelRegistry[typeString].AfterRead; after != nil {
		bhpData := models.BatchHookPointData{Ms: outmodels, DB: db2, OID: oid, Scope: scope, TypeString: typeString, Roles: roles}
		if err = after(bhpData); err != nil {
			return nil, nil, nil, err
		}
	}

	return outmodels, roles, no, err
}

// UpdateOneWithID updates model based on this json
// This is the same as ownershipdata mapper (inheritance?)
func (mapper *GlobalMapper) UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID) (models.IModel, error) {
	var oldModelObj models.IModel
	var err error
	if oldModelObj, _, err = loadAndCheckErrorBeforeModify(mapper.Service, db, oid, scope, typeString, modelObj, id, []models.UserRole{models.UserRoleAny}); err != nil {
		return nil, err
	}

	var before func(hpdata models.HookPointData) error
	var after func(hpdata models.HookPointData) error
	if v, ok := modelObj.(models.IBeforeUpdate); ok {
		before = v.BeforeUpdateDB
	}
	if v, ok := modelObj.(models.IAfterUpdate); ok {
		after = v.AfterUpdateDB
	}

	j := opJob{
		// mapper:      mapper,
		serv:        mapper.Service,
		db:          db,
		oid:         oid,
		scope:       scope,
		typeString:  typeString,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,
	}
	return opCore(before, after, j, updateOneCore)
}

// UpdateMany updates multiple models
// This is the same as ownershipdata mapper (inheritance?)
func (mapper *GlobalMapper) UpdateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
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

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper.Service, db, oid, scope, typeString, ids, []models.UserRole{models.UserRoleAny})
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
	return batchOpCore(j, before, after, updateOneCore)
}

// PatchOneWithID updates model based on this json
// This is the same as ownershipdata mapper (inheritance?)
func (mapper *GlobalMapper) PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonPatch []byte, id *datatypes.UUID) (models.IModel, error) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, oid, scope, typeString, nil, id, []models.UserRole{models.UserRoleAny})
	if err != nil {
		return nil, err
	}

	var modelObj models.IModel
	// Apply patch operations
	modelObj, err = applyPatchCore(typeString, oldModelObj, jsonPatch)
	if err != nil {
		return nil, err
	}

	var before func(hpdata models.HookPointData) error
	var after func(hpdata models.HookPointData) error
	if v, ok := modelObj.(models.IBeforePatch); ok {
		before = v.BeforePatchDB
	}
	if v, ok := modelObj.(models.IAfterPatch); ok {
		after = v.AfterPatchDB
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
	return opCore(before, after, j, updateOneCore)
}

// PatchMany updates models based on JSON
func (mapper *GlobalMapper) PatchMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonIDPatches []models.JSONIDPatch) ([]models.IModel, error) {
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

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper.Service, db, oid, scope, typeString, ids, []models.UserRole{models.UserRoleAny})
	if err != nil {
		return nil, err
	}

	// Now patch it
	modelObjs := make([]models.IModel, len(oldModelObjs))
	for i, jsonIDPatch := range jsonIDPatches {
		// Any one can patch is since it's global. If there should be restrictions, it should be done
		// in model hookpoints
		// Apply patch operations
		modelObjs[i], err = applyPatchCore(typeString, oldModelObjs[i], []byte(jsonIDPatch.Patch))
		if err != nil {
			log.Println("patch error: ", err, string(jsonIDPatch.Patch))
			return nil, err
		}
	}

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
	return batchOpCore(j, before, after, updateOneCore)
}

// DeleteOneWithID delete the model
// TODO: delete the groups associated with this record?
func (mapper *GlobalMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, error) {
	modelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, oid, scope, typeString, nil, id, []models.UserRole{models.UserRoleAny})
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

	var before func(hpdata models.HookPointData) error
	var after func(hpdata models.HookPointData) error
	if v, ok := modelObj.(models.IBeforeDelete); ok {
		before = v.BeforeDeleteDB
	}
	if v, ok := modelObj.(models.IAfterDelete); ok {
		after = v.AfterDeleteDB
	}

	j := opJob{
		// mapper:     mapper,
		serv:       mapper.Service,
		db:         db,
		oid:        oid,
		scope:      scope,
		typeString: typeString,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
	}
	return opCore(before, after, j, deleteOneCore)
}

// DeleteMany deletes multiple models
func (mapper *GlobalMapper) DeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	// Since it's global, no permission to check here
	// only need to check if id is empty

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

	modelObjs, _, err := loadManyAndCheckBeforeModify(mapper.Service, db, oid, scope, typeString, ids, []models.UserRole{models.UserRoleAny})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if len(modelObjs) > 0 && modelNeedsRealDelete(modelObjs[0]) {
		db = db.Unscoped()
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
	return batchOpCore(j, before, after, deleteOneCore)
}

// ----------------------------------------------------------------------------------------
