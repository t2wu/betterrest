package datamapper

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

// check out
// https://stackoverflow.com/questions/52124137/cant-set-field-of-a-struct-that-is-typed-as-an-interface
/*
	a := reflect.ValueOf(modelObj).Elem()
	b := reflect.Indirect(a).FieldByName("ID")
	b.Set(reflect.ValueOf(uint(id)))
*/

// ---------------------------------------

var onceOwnership sync.Once
var crudOwnership *OwnershipMapper

// OwnershipMapper is a basic CRUD manager
type OwnershipMapper struct {
}

// SharedOwnershipMapper creats a singleton of Crud object
func SharedOwnershipMapper() *OwnershipMapper {
	onceOwnership.Do(func() {
		crudOwnership = &OwnershipMapper{}
	})

	return crudOwnership
}

// CreateOne creates an instance of this model based on json and store it in db
func (mapper *OwnershipMapper) CreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	// db.Model(&u).Association("Ownerships")

	// Get ownership type and creates it
	// field, _ := reflect.TypeOf(modelObj).Elem().FieldByName("Ownerships")
	// ownershipType := field.Type

	// has to be true otherwise shouldn't use this mapper
	modelObjOwnership, ok := modelObj.(models.IHasOwnershipLink)
	if !ok {
		return nil, errNoOwnership
	}

	ownershipType := modelObjOwnership.OwnershipType()

	// reflect.SliceOf
	g := reflect.New(ownershipType).Interface().(models.IOwnership)

	modelID := modelObj.GetID()
	if modelID == nil {
		modelID = datatypes.NewUUID()
		modelObj.SetID(modelID)
	}

	g.SetUserID(oid)
	g.SetModelID(modelID)
	g.SetRole(models.Admin)

	// ownerships := reflect.New(reflect.SliceOf(ownershipType))
	// o.Set(reflect.Append(ownerships, reflect.ValueOf(g)))

	// Associate a ownership group with this model
	// This is not strictly really necessary as actual SQL table has no such field. I could have
	// just save the "g", But it's for hooks
	o := reflect.ValueOf(modelObj).Elem().FieldByName("Ownerships")
	o.Set(reflect.Append(o, reflect.ValueOf(g).Elem()))

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
	return opCore(before, after, j, createOneCoreOwnership)
}

// CreateMany creates an instance of this model based on json and store it in db
func (mapper *OwnershipMapper) CreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	// db.Model(&u).Association("Ownerships")

	// Get ownership type and creates it
	// field, _ := reflect.TypeOf(modelObj).Elem().FieldByName("Ownerships")
	// ownershipType := field.Type
	modelObjOwnership, ok := modelObjs[0].(models.IHasOwnershipLink)
	if !ok {
		return nil, errNoOwnership
	}

	ownershipType := modelObjOwnership.OwnershipType()
	for _, modelObj := range modelObjs {
		// reflect.SliceOf
		g := reflect.New(ownershipType).Interface().(models.IOwnership)

		modelID := modelObj.GetID()
		if modelID == nil {
			modelID = datatypes.NewUUID()
			modelObj.SetID(modelID)
		}

		g.SetUserID(oid)
		g.SetModelID(modelID)
		g.SetRole(models.Admin)

		// ownerships := reflect.New(reflect.SliceOf(ownershipType))
		// o.Set(reflect.Append(ownerships, reflect.ValueOf(g)))

		// Associate a ownership group with this model
		// This is not strictly really necessary as actual SQL table has no such field. I could have
		// just save the "g", But it's for hooks
		o := reflect.ValueOf(modelObj).Elem().FieldByName("Ownerships")
		o.Set(reflect.Append(o, reflect.ValueOf(g).Elem()))
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
	return batchOpCore(j, before, after, createOneCoreOwnership)
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *OwnershipMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	// anyone permission can read as long as you are linked on db
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

// GetAll obtains a slice of models.DomainModel
// options can be string "offset" and "limit", both of type int
// This is very Javascript-esque. I would have liked Python's optional parameter more.
// Alas, no such feature in Go. https://stackoverflow.com/questions/2032149/optional-parameters-in-go
// How does Gorm do the following? Might want to check out its source code.
// Cancel offset condition with -1
//  db.Offset(10).Find(&users1).Offset(-1).Find(&users2)
func (mapper *OwnershipMapper) GetAll(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, options map[URLParam]interface{}) ([]models.IModel, []models.UserRole, *int, error) {
	db2 := db
	db = db.Set("gorm:auto_preload", true)

	offset, limit, cstart, cstop, order, latestn, totalcount := getOptions(options)
	rtable, joinTableName, err := getModelTableNameAndJoinTableNameFromTypeString(typeString)
	if err != nil {
		return nil, nil, nil, err
	}

	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id", joinTableName, rtable, joinTableName)
	secondJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)

	if cstart != nil && cstop != nil {
		db = db.Where(rtable+".created_at BETWEEN ? AND ?", time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))
	}

	db, err = constructInnerFieldParamQueries(db, typeString, options, latestn)
	if err != nil {
		return nil, nil, nil, err
	}

	db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin, oid.String())

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

	db3 := db

	roles := make([]models.UserRole, 0)
	outmodels, err := models.NewSliceFromDBByTypeString(typeString, db.Find) // error from db is returned from here

	// ---------------------------
	// ownershipModelTyp := getOwnershipModelTypeFromTypeString(typeString)

	// role := models.Admin // just some default
	// The difference between this method and the find is that it's missing the
	// WHERE "model"."deleted_at" IS NULL, so we need to add it
	if err = db3.Where(fmt.Sprintf("\"%s\".\"deleted_at\" IS NULL", rtable)).
		Select(fmt.Sprintf("\"%s\".\"role\"", joinTableName)).Scan(&roles).Error; err != nil {
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
func (mapper *OwnershipMapper) UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID) (models.IModel, error) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, modelObj, id, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	// TODO: Huh? How do we do validation here?!

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
func (mapper *OwnershipMapper) UpdateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	// load old model data
	ids := make([]*datatypes.UUID, len(modelObjs))
	for i, modelObj := range modelObjs {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		id := modelObj.GetID()
		if id != nil && id.String() != "" {
			return nil, errIDEmpty
		}
		ids[i] = id
	}

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper, db, oid, scope, typeString, ids, []models.UserRole{models.Admin})
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
func (mapper *OwnershipMapper) PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonPatch []byte, id *datatypes.UUID) (models.IModel, error) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, nil, id, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	// Apply patch operations
	modelObj, err := applyPatchCore(typeString, oldModelObj, jsonPatch)
	if err != nil {
		return nil, err
	}

	// TODO: Huh? How do we do validation here?!
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

// PatchMany patches multiple models
func (mapper *OwnershipMapper) PatchMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonIDPatches []models.JSONIDPatch) ([]models.IModel, error) {
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

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper, db, oid, scope, typeString, ids, []models.UserRole{models.Admin})

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

// DeleteOneWithID delete the model
// TODO: delete the groups associated with this record?
func (mapper *OwnershipMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, error) {
	modelObj, _, err := loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, nil, id, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	// I'm removing stuffs from this link table, I cannot just remove myself from this. I have to remove
	// everyone who is linked to this table!
	modelObjOwnership, ok := modelObj.(models.IHasOwnershipLink)
	if !ok {
		return nil, errNoOwnership
	}

	// stmt := fmt.Sprintf("DELETE FROM %s WHERE user_id = ? AND model_id = ? AND role = ?", models.GetJoinTableName(modelObjOwnership))
	stmt := fmt.Sprintf("DELETE FROM %s WHERE model_id = ?", models.GetJoinTableName(modelObjOwnership))

	// Can't do db.Raw and db.Delete at the same time?!
	db2 := db.Exec(stmt, modelObj.GetID().String())
	if db2.Error != nil {
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
func (mapper *OwnershipMapper) DeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	// load old model data
	ids := make([]*datatypes.UUID, len(modelObjs))
	for i, modelObj := range modelObjs {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		id := modelObj.GetID()
		if id != nil && id.String() != "" {
			return nil, errIDEmpty
		}
		ids[i] = id
	}

	modelObjs, _, err := loadManyAndCheckBeforeModify(mapper, db, oid, scope, typeString, ids, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	// Delete link table because GORM isn't automatic here when we customize it with UUID or when we have role
	for _, modelObj := range modelObjs {
		// Also remove entries from ownership table
		modelObjOwnership, ok := modelObj.(models.IHasOwnershipLink)
		if !ok {
			return nil, errNoOwnership
		}
		stmt := fmt.Sprintf("DELETE FROM %s WHERE model_id = ?", models.GetJoinTableName(modelObjOwnership))
		db2 := db.Exec(stmt, modelObj.GetID().String())
		if db2.Error != nil {
			return nil, err
		}
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if len(modelObjs) > 0 && modelNeedsRealDelete(modelObjs[0]) {
		db = db.Unscoped() // hookpoint wil inherit this though
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
	}
	return batchOpCore(j, before, after, deleteOneCore)
}

// ----------------------------------------------------------------------------------------

// getOneWithIDCore get one model object based on its type and its id string
func (mapper *OwnershipMapper) getOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj := models.NewFromTypeString(typeString)

	db = db.Set("gorm:auto_preload", true)

	rtable := models.GetTableNameFromIModel(modelObj)

	/*
		SELECT * from some_model
		INNER JOIN user_owns_somemodel ON somemodel.id = user_owns_somemodel.model_id AND somemodel.id = UUID_TO_BIN(id)
		INNER JOIN user ON user.id = user_owns_somemodel.user_id AND user.id = UUID_TO_BIN(oid)
	*/

	modelObjOwnership, ok := modelObj.(models.IHasOwnershipLink)
	if !ok {
		return nil, 0, errNoOwnership
	}

	joinTableName := models.GetJoinTableName(modelObjOwnership)

	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id AND \"%s\".id = ?", joinTableName, rtable, joinTableName, rtable)
	secondJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)
	// db2 := db

	err := db.Table(rtable).Joins(firstJoin, id.String()).Joins(secondJoin, oid.String()).Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	joinTable := reflect.New(modelObjOwnership.OwnershipType()).Interface()
	role := models.Invalid // just some default
	stmt := fmt.Sprintf("SELECT * FROM %s WHERE user_id = ? AND model_id = ?", joinTableName)
	if err2 := db.Raw(stmt, oid.String(), id.String()).Scan(joinTable).Error; err2 != nil {
		return nil, 0, err2
	}

	if m, ok := joinTable.(models.IOwnership); ok {
		role = m.GetRole()
	}

	err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db, modelObj)
	if err != nil {
		return nil, 0, err
	}

	return modelObj, role, err
}

func (mapper *OwnershipMapper) getManyWithIDsCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	// If I can load it, I have permission to edit it. So no need to call loadAndCheckErrorBeforeModify
	// like when I do for update. Just get the role and check if it's admin
	rtable, joinTableName, err := getModelTableNameAndJoinTableNameFromTypeString(typeString)
	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id AND \"%s\".id IN (?)", joinTableName, rtable, joinTableName, rtable)
	secondJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)

	db2 := db.Table(rtable).Joins(firstJoin, ids).Joins(secondJoin, oid)
	modelObjs, err := models.NewSliceFromDBByTypeString(typeString, db2.Set("gorm:auto_preload", true).Find)
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

	// Check error
	// Load the roles and check if they're admin
	roles := make([]models.UserRole, 0)
	if err := db2.Select(fmt.Sprintf("\"%s\".\"role\"", joinTableName)).Scan(&roles).Error; err != nil {
		log.Printf("err getting roles")
		return nil, nil, err
	}

	return modelObjs, roles, nil
}
