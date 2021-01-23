package datamapper

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// createOneCoreGlobalMapper creates a model
func createOneCoreGlobalMapper(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel) (models.IModel, error) {
	if err := db.Create(modelObj).Error; err != nil {
		return nil, err
	}

	// For pegassociated, the since we expect association_autoupdate:false
	// need to manually create it
	if err := createPeggedAssocFields(db, modelObj); err != nil {
		return nil, err
	}

	// For table with trigger which update before insert, we need to load it again
	if err := db.First(modelObj).Error; err != nil {
		// That's weird. we just inserted it.
		return nil, err
	}

	return modelObj, nil
}

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

	return createOneWithHooks(createOneCoreGlobalMapper, db, oid, scope, typeString, modelObj)
}

// CreateMany creates an instance of this model based on json and store it in db
func (mapper *GlobalMapper) CreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	retModels := make([]models.IModel, 0, 20)

	cargo := models.BatchHookCargo{}
	// Before batch inert hookpoint
	if before := models.ModelRegistry[typeString].BeforeInsert; before != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := before(bhpData); err != nil {
			return nil, err
		}
	}

	for _, modelObj := range modelObjs {
		modelID := modelObj.GetID()
		if modelID == nil {
			modelID = datatypes.NewUUID()
			modelObj.SetID(modelID)
		}

		m, err := createOneCoreGlobalMapper(db, oid, typeString, modelObj)
		if err != nil {
			return nil, err
		}

		retModels = append(retModels, m)
	}

	// After batch insert hookpoint
	if after := models.ModelRegistry[typeString].AfterInsert; after != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := after(bhpData); err != nil {
			return nil, err
		}
	}

	return retModels, nil
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *GlobalMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error) {

	modelObj, role, err := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil {
		return nil, 0, err
	}

	if m, ok := modelObj.(models.IAfterRead); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Role: &role}
		if err := m.AfterReadDB(hpdata); err != nil {
			return nil, 0, err
		}
	}

	return modelObj, role, err
}

// getOneWithIDCore get one model object based on its type and its id string
func (mapper *GlobalMapper) getOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj := models.NewFromTypeString(typeString)
	modelObj.SetID(&id)

	db = db.Set("gorm:auto_preload", true)

	// rtable := models.GetTableNameFromIModel(modelObj)

	// Global object, everyone can find it, simply find it
	err := db.Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	role := models.Public // just some default

	err = loadManyToManyBecauseGormFailsWithID(db, modelObj)
	if err != nil {
		return nil, 0, err
	}

	return modelObj, role, err
}

// ReadAll is when user do a read
func (mapper *GlobalMapper) ReadAll(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, options map[URLParam]interface{}) ([]models.IModel, []models.UserRole, *int, error) {
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
		err = loadManyToManyBecauseGormFailsWithID(db2, m)
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
func (mapper *GlobalMapper) UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id datatypes.UUID) (models.IModel, error) {
	if err := checkErrorBeforeUpdate(mapper, db, oid, scope, typeString, modelObj, id, models.Public); err != nil {
		return nil, err
	}

	cargo := models.ModelCargo{}

	// Before hook
	if v, ok := modelObj.(models.IBeforeUpdate); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := v.BeforeUpdateDB(hpdata); err != nil {
			return nil, err
		}
	}

	modelObj2, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, id, models.Public)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterUpdate); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = v.AfterUpdateDB(hpdata); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}

// UpdateMany updates multiple models
// This is the same as ownershipdata mapper (inheritance?)
func (mapper *GlobalMapper) UpdateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	ms := make([]models.IModel, 0, 0)
	var err error
	cargo := models.BatchHookCargo{}

	// Before batch update hookpoint
	if before := models.ModelRegistry[typeString].BeforeUpdate; before != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = before(bhpData); err != nil {
			return nil, err
		}
	}

	for _, modelObj := range modelObjs {
		id := modelObj.GetID()

		if err = checkErrorBeforeUpdate(mapper, db, oid, scope, typeString, modelObj, *id, models.Public); err != nil {
			return nil, err
		}

		m, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, *id, models.Public)
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}

		ms = append(ms, m)
	}

	// After batch update hookpoint
	if after := models.ModelRegistry[typeString].AfterUpdate; after != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = after(bhpData); err != nil {
			return nil, err
		}
	}

	return ms, nil
}

// PatchOneWithID updates model based on this json
// This is the same as ownershipdata mapper (inheritance?)
func (mapper *GlobalMapper) PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonPatch []byte, id datatypes.UUID) (models.IModel, error) {
	var modelObj models.IModel
	var err error
	cargo := models.ModelCargo{}

	// Check id error
	if id.UUID.String() == "" {
		return nil, errIDEmpty
	}

	// role already chcked in checkErrorBeforeUpdate
	if modelObj, _, err = mapper.getOneWithIDCore(db, oid, scope, typeString, id); err != nil {
		return nil, err
	}

	// Apply patch operations
	modelObj, err = patchOneCore(typeString, modelObj, jsonPatch)
	if err != nil {
		return nil, err
	}

	// Before hook
	// It is now expected that the hookpoint for before expect that the patch
	// gets applied to the JSON, but not before actually updating to DB.
	if v, ok := modelObj.(models.IBeforePatch); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := v.BeforePatchDB(hpdata); err != nil {
			return nil, err
		}
	}

	// Now save it
	modelObj2, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, id, models.Public)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterPatch); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = v.AfterPatchDB(hpdata); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}

// PatchMany updates models based on JSON
func (mapper *GlobalMapper) PatchMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonIDPatches []models.JSONIDPatch) ([]models.IModel, error) {
	ms := make([]models.IModel, 0, len(jsonIDPatches)) // len(ms)=0, cap(ms)=len(jsonIDPatches)
	// var err error

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

	rtable := models.GetTableNameFromTypeString(typeString)
	db2 := db.Table(rtable).Where(fmt.Sprintf("\"%s\".\"id\" IN (?)", rtable), ids).Set("gorm:auto_preload", true)

	modelObjs, err := models.NewSliceFromDBByTypeString(typeString, db2.Set("gorm:auto_preload", true).Find)
	if err != nil {
		log.Println("calling NewSliceFromDBByTypeString err:", err)
		return nil, err
	}

	// Just in case err didn't work (as in the case with IN clause NOT in the ID field, maybe Gorm bug)
	if len(modelObjs) == 0 {
		return nil, fmt.Errorf("not found")
	}

	if len(modelObjs) != len(jsonIDPatches) {
		return nil, errBatchUpdateOrPatchOneNotFound
	}

	// Now patch it
	for i, jsonIDPatch := range jsonIDPatches {
		// Any one can patch is since it's global. If there should be restrictions, it should be done
		// in hookpoints
		// Apply patch operations
		modelObjs[i], err = patchOneCore(typeString, modelObjs[i], []byte(jsonIDPatch.Patch))
		if err != nil {
			log.Println("patch error: ", err, string(jsonIDPatch.Patch))
			return nil, err
		}
	}

	cargo := models.BatchHookCargo{}
	// Before batch update hookpoint
	if before := models.ModelRegistry[typeString].BeforeUpdate; before != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = before(bhpData); err != nil {
			return nil, err
		}
	}

	// TODO: Could update all at once, then load all at once again
	for _, modelObj := range modelObjs {
		id := modelObj.GetID()

		m, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, *id, models.Public)
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}

		ms = append(ms, m)
	}

	// After batch update hookpoint
	if after := models.ModelRegistry[typeString].AfterUpdate; after != nil {
		bhpData := models.BatchHookPointData{Ms: ms, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = after(bhpData); err != nil {
			return nil, err
		}
	}

	return ms, nil
}

// DeleteOneWithID delete the model
// TODO: delete the groups associated with this record?
func (mapper *GlobalMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, error) {
	if id.UUID.String() == "" {
		return nil, errIDEmpty
	}

	// check out
	// https://stackoverflow.com/questions/52124137/cant-set-field-of-a-struct-that-is-typed-as-an-interface
	/*
		a := reflect.ValueOf(modelObj).Elem()
		b := reflect.Indirect(a).FieldByName("ID")
		b.Set(reflect.ValueOf(uint(id)))
	*/

	// Pull out entire modelObj
	modelObj, _, err := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return nil, err
	}

	cargo := models.ModelCargo{}

	// Before delete hookpoint
	if v, ok := modelObj.(models.IBeforeDelete); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		err = v.BeforeDeleteDB(hpdata)
		if err != nil {
			return nil, err
		}
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	err = db.Delete(modelObj).Error
	if err != nil {
		return nil, err
	}

	// Remove ownership
	// modelObj.
	// db.Model(modelObj).Association("Ownerships").Delete(modelObj.)
	// c.DB.Model(&user).Association("Roles").Delete(&role)

	err = removePeggedField(db, modelObj)
	if err != nil {
		return nil, err
	}

	// After delete hookpoint
	if v, ok := modelObj.(models.IAfterDelete); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		err = v.AfterDeleteDB(hpdata)
		if err != nil {
			return nil, err
		}
	}

	return modelObj, nil
}

// DeleteMany deletes multiple models
func (mapper *GlobalMapper) DeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {

	ids := make([]datatypes.UUID, len(modelObjs), len(modelObjs))
	var err error
	cargo := models.BatchHookCargo{}
	for i, v := range modelObjs {
		id := v.GetID()
		if id.String() == "" {
			return nil, errIDEmpty
		}

		ids[i] = *id
	}

	ms := make([]models.IModel, 0, 0)

	// Before batch delete hookpoint
	if before := models.ModelRegistry[typeString].BeforeDelete; before != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = before(bhpData); err != nil {
			return nil, err
		}
	}

	for i, id := range ids {

		if id.UUID.String() == "" {
			return nil, errIDEmpty
		}

		// Pull out entire modelObj
		modelObj, _, err := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}

		// Unscoped() for REAL delete!
		// Foreign key constraint works only on real delete
		// Soft delete will take more work, have to verify myself manually
		if modelNeedsRealDelete(modelObj) && i == 0 { // only do once
			db = db.Unscoped()
		}

		err = db.Delete(modelObj).Error
		// err = db.Delete(modelObj).Error
		if err != nil {
			return nil, err
		}

		err = removePeggedField(db, modelObj)
		if err != nil {
			return nil, err
		}

		ms = append(ms, modelObj)
	}

	// After batch delete hookpoint
	if after := models.ModelRegistry[typeString].AfterDelete; after != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = after(bhpData); err != nil {
			return nil, err
		}
	}

	return ms, nil
}
