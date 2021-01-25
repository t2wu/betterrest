package datamapper

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// ---------------------------------------

var onceGlobal sync.Once
var crudGlobal *GlobalMapper

// GlobalMapper is a basic CRUD manager
type GlobalMapper struct {
}

// SharedGlobalMapper creats a singleton of Crud object
func SharedGlobalMapper() *GlobalMapper {
	onceGlobal.Do(func() {
		crudGlobal = &GlobalMapper{}
	})

	return crudGlobal
}

// CreateOne creates an instance of this model based on json and store it in db
func (mapper *GlobalMapper) CreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	modelID := modelObj.GetID()
	if modelID == nil {
		modelID = datatypes.NewUUID()
		modelObj.SetID(modelID)
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
		mapper:     mapper,
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
	// This probably is not necessary
	for i, modelObj := range modelObjs {
		modelID := modelObj.GetID()
		if modelID == nil {
			modelID = datatypes.NewUUID()
			modelObj.SetID(modelID)
		}
		modelObjs[i] = modelObj
	}

	before := models.ModelRegistry[typeString].BeforeInsert
	after := models.ModelRegistry[typeString].AfterInsert
	j := batchOpJob{
		mapper:       mapper,
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
	modelObj, role, err := loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, nil, id, []models.UserRole{models.UserRoleAny})
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
		// rows.Scan()
		db = db.Offset(*offset).Limit(*limit)
		// db2 = db2.Offset(*offset).Limit(*limit)
	}

	outmodels, err := models.NewSliceFromDBByTypeString(typeString, db.Find) // error from db is returned from here

	// Don't know why this doesn't work
	roles := make([]models.UserRole, len(outmodels), len(outmodels))

	for i := range roles {
		roles[i] = models.Public
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
	if oldModelObj, _, err = loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, modelObj, id, []models.UserRole{models.UserRoleAny}); err != nil {
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
		mapper:      mapper,
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
			return nil, errIDEmpty
		}
		ids[i] = id
	}

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper, db, oid, scope, typeString, ids, []models.UserRole{models.UserRoleAny})
	if err != nil {
		return nil, err
	}

	before := models.ModelRegistry[typeString].BeforeUpdate
	after := models.ModelRegistry[typeString].AfterUpdate
	j := batchOpJob{
		mapper:       mapper,
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
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, nil, id, []models.UserRole{models.UserRoleAny})
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
		mapper:      mapper,
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
			return nil, errIDEmpty
		}
		ids[i] = jsonIDPatch.ID
	}

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper, db, oid, scope, typeString, ids, []models.UserRole{models.UserRoleAny})
	if err != nil {
		return nil, err
	}

	// Now patch it
	modelObjs := make([]models.IModel, len(oldModelObjs))
	for i, jsonIDPatch := range jsonIDPatches {
		// Any one can patch is since it's global. If there should be restrictions, it should be done
		// in hookpoints
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
		mapper:       mapper,
		db:           db,
		oid:          oid,
		scope:        scope,
		typeString:   typeString,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
		// roles:        roles,
	}
	return batchOpCore(j, before, after, updateOneCore)
}

// DeleteOneWithID delete the model
// TODO: delete the groups associated with this record?
func (mapper *GlobalMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, error) {
	modelObj, _, err := loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, nil, id, []models.UserRole{models.UserRoleAny})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
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
		mapper:     mapper,
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
			return nil, errIDEmpty
		}
		ids[i] = id
	}

	modelObjs, _, err := loadManyAndCheckBeforeModify(mapper, db, oid, scope, typeString, ids, []models.UserRole{models.UserRoleAny})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if len(modelObjs) > 0 && modelNeedsRealDelete(modelObjs[0]) {
		db = db.Unscoped()
	}

	before := models.ModelRegistry[typeString].BeforeDelete
	after := models.ModelRegistry[typeString].AfterDelete

	j := batchOpJob{
		mapper:     mapper,
		db:         db,
		oid:        oid,
		scope:      scope,
		typeString: typeString,
		modelObjs:  modelObjs,
		// roles:      roles,
	}
	return batchOpCore(j, before, after, deleteOneCore)
}

// ----------------------------------------------------------------------------------------

// getOneWithIDCore get one model object based on its type and its id string
func (mapper *GlobalMapper) getOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj := models.NewFromTypeString(typeString)
	modelObj.SetID(id)

	db = db.Set("gorm:auto_preload", true)

	// rtable := models.GetTableNameFromIModel(modelObj)

	// Global object, everyone can find it, simply find it
	err := db.Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	role := models.Public // just some default

	err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db, modelObj)
	if err != nil {
		return nil, 0, err
	}

	return modelObj, role, err
}

func (mapper *GlobalMapper) getManyWithIDsCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	rtable := models.GetTableNameFromTypeString(typeString)
	db = db.Table(rtable).Where(fmt.Sprintf("\"%s\".\"id\" IN (?)", rtable), ids).Set("gorm:auto_preload", true)

	modelObjs, err := models.NewSliceFromDBByTypeString(typeString, db.Set("gorm:auto_preload", true).Find)
	if err != nil {
		log.Println("calling NewSliceFromDBByTypeString err:", err)
		return nil, nil, err
	}

	// Just in case err didn't work (as in the case with IN clause NOT in the ID field, maybe Gorm bug)
	if len(modelObjs) == 0 {
		return nil, nil, fmt.Errorf("not found")
	}

	if len(modelObjs) != len(ids) {
		return nil, nil, errBatchUpdateOrPatchOneNotFound
	}

	for _, modelObj := range modelObjs {
		err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db, modelObj)
		if err != nil {
			return nil, nil, err
		}
	}

	return modelObjs, nil, nil
}
