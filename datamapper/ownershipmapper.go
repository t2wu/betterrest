package datamapper

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

// ---------------------------------------

// createOneCoreOwnershipMapper creates a model
func createOneCoreOwnershipMapper(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel) (models.IModel, error) {
	// It looks like I need to explicitly call create here
	o := reflect.ValueOf(modelObj).Elem().FieldByName("Ownerships")
	g, _ := o.Index(0).Addr().Interface().(models.IOwnership)

	// No need to check if primary key is blank.
	// If it is it'll be created by Gorm's BeforeCreate hook
	// (defined in base model)
	// if dbc := db.Create(modelObj); dbc.Error != nil {
	if dbc := db.Create(modelObj).Create(g); dbc.Error != nil {
		// create failed: UNIQUE constraint failed: user.email
		// It looks like this error may be dependent on the type of database we use
		return nil, dbc.Error
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

	return createOneWithHooks(createOneCoreOwnershipMapper, db, oid, scope, typeString, modelObj)
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
	retModels := make([]models.IModel, 0, 20)

	cargo := models.BatchHookCargo{}
	// Before batch inert hookpoint
	if before := models.ModelRegistry[typeString].BeforeInsert; before != nil {
		if err := before(modelObjs, db, oid, scope, typeString, &cargo); err != nil {
			return nil, err
		}
	}

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

		m, err := createOneCoreOwnershipMapper(db, oid, typeString, modelObj)
		if err != nil {
			return nil, err
		}

		retModels = append(retModels, m)
	}

	// After batch insert hookpoint
	if after := models.ModelRegistry[typeString].AfterInsert; after != nil {
		if err := after(modelObjs, db, oid, scope, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	return retModels, nil
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *OwnershipMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error) {

	modelObj, role, err := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil {
		return nil, 0, err
	}

	if m, ok := modelObj.(models.IAfterRead); ok {
		if err := m.AfterReadDB(db, oid, scope, typeString, &role); err != nil {
			return nil, 0, err
		}
	}

	return modelObj, role, err
}

// getOneWithIDCore get one model object based on its type and its id string
func (mapper *OwnershipMapper) getOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error) {
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
	role := models.Guest // just some default
	stmt := fmt.Sprintf("SELECT * FROM %s WHERE user_id = ? AND model_id = ?", joinTableName)
	if err2 := db.Raw(stmt, oid.String(), id.String()).Scan(joinTable).Error; err2 != nil {
		return nil, 0, err2
	}

	if m, ok := joinTable.(models.IOwnership); ok {
		role = m.GetRole()
	}

	err = loadManyToManyBecauseGormFailsWithID(db, modelObj)
	if err != nil {
		return nil, 0, err
	}

	return modelObj, role, err
}

// ReadAll obtains a slice of models.DomainModel
// options can be string "offset" and "limit", both of type int
// This is very Javascript-esque. I would have liked Python's optional parameter more.
// Alas, no such feature in Go. https://stackoverflow.com/questions/2032149/optional-parameters-in-go
// How does Gorm do the following? Might want to check out its source code.
// Cancel offset condition with -1
//  db.Offset(10).Find(&users1).Offset(-1).Find(&users2)
func (mapper *OwnershipMapper) ReadAll(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, options map[URLParam]interface{}) ([]models.IModel, []models.UserRole, error) {
	db2 := db
	db = db.Set("gorm:auto_preload", true)

	offset, limit, cstart, cstop, order, latestn := getOptions(options)
	rtable, joinTableName, err := getModelTableNameAndJoinTableNameFromTypeString(typeString)
	if err != nil {
		return nil, nil, err
	}

	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id", joinTableName, rtable, joinTableName)
	secondJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)

	if cstart != nil && cstop != nil {
		db = db.Where(rtable+".created_at BETWEEN ? AND ?", time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))
	}

	db, err = constructInnerFieldParamQueries(db, typeString, options, latestn)
	if err != nil {
		return nil, nil, err
	}

	db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin, oid.String())

	db = constructOrderFieldQueries(db, rtable, order)

	if offset != nil && limit != nil {
		db = db.Offset(*offset).Limit(*limit)
	}

	roles := make([]models.UserRole, 0)
	outmodels, err := models.NewSliceFromDBByTypeString(typeString, db.Find) // error from db is returned from here

	// ---------------------------
	ownershipModelTyp := getOwnershipModelTypeFromTypeString(typeString)
	// FIXME: this performance horrible...might as well query the above again for a role
	for _, outmodel := range outmodels {
		joinTable := reflect.New(ownershipModelTyp).Interface()
		role := models.Admin // just some default
		mid := outmodel.GetID()
		stmt := fmt.Sprintf("SELECT * FROM %s WHERE user_id = ? AND model_id = ?", joinTableName)
		if err2 := db2.Raw(stmt, oid.String(), mid.String()).Scan(joinTable).Error; err2 != nil {
			return nil, nil, err2
		}

		if m, ok := joinTable.(models.IOwnership); ok {
			role = m.GetRole()
			roles = append(roles, role)
		}
	}

	// safeguard, Must be coded wrongly
	if len(outmodels) != len(roles) {
		return nil, nil, errors.New("unknown query error")
	}

	// make many to many tag works
	for _, m := range outmodels {
		err = loadManyToManyBecauseGormFailsWithID(db2, m)
		if err != nil {
			return nil, nil, err
		}
	}

	// use db2 cuz it's not chained
	if after := models.ModelRegistry[typeString].AfterRead; after != nil {
		if err = after(outmodels, db2, oid, scope, typeString, roles); err != nil {
			return nil, nil, err
		}
	}

	return outmodels, roles, err
}

// UpdateOneWithID updates model based on this json
func (mapper *OwnershipMapper) UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id datatypes.UUID) (models.IModel, error) {
	if err := checkErrorBeforeUpdate(mapper, db, oid, scope, typeString, modelObj, id, models.Admin); err != nil {
		return nil, err
	}

	cargo := models.ModelCargo{}

	// Before hook
	if v, ok := modelObj.(models.IBeforeUpdate); ok {
		if err := v.BeforeUpdateDB(db, oid, scope, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	modelObj2, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, id, models.Admin)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterUpdate); ok {
		if err = v.AfterUpdateDB(db, oid, scope, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}

// UpdateMany updates multiple models
func (mapper *OwnershipMapper) UpdateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	ms := make([]models.IModel, 0, 0)
	var err error
	cargo := models.BatchHookCargo{}

	// Before batch update hookpoint
	if before := models.ModelRegistry[typeString].BeforeUpdate; before != nil {
		if err = before(modelObjs, db, oid, scope, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	for _, modelObj := range modelObjs {
		id := modelObj.GetID()

		if err = checkErrorBeforeUpdate(mapper, db, oid, scope, typeString, modelObj, *id, models.Admin); err != nil {
			return nil, err
		}

		m, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, *id, models.Admin)
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}

		ms = append(ms, m)
	}

	// After batch update hookpoint
	if after := models.ModelRegistry[typeString].AfterUpdate; after != nil {
		if err = after(modelObjs, db, oid, scope, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	return ms, nil
}

// PatchOneWithID updates model based on this json
func (mapper *OwnershipMapper) PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonPatch []byte, id datatypes.UUID) (models.IModel, error) {
	var modelObj models.IModel
	var err error
	cargo := models.ModelCargo{}
	var role models.UserRole

	// Check id error
	if id.UUID.String() == "" {
		return nil, errIDEmpty
	}

	// role already chcked in checkErrorBeforeUpdate
	if modelObj, role, err = mapper.getOneWithIDCore(db, oid, scope, typeString, id); err != nil {
		return nil, err
	}

	// calling checkErrorBeforeUpdate is redundant in this case since we need to fetch it out first in order to patch it
	// Just check if role matches models.Admin
	if role != models.Admin {
		return nil, errPermission
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
		if err := v.BeforePatchDB(db, oid, scope, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	// Now save it
	modelObj2, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, id, models.Admin)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterPatch); ok {
		if err = v.AfterPatchDB(db, oid, scope, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}

// DeleteOneWithID delete the model
// TODO: delete the groups associated with this record?
func (mapper *OwnershipMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, error) {
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
	modelObj, role, err := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return nil, err
	}
	if role != models.Admin {
		return nil, errPermission
	}

	cargo := models.ModelCargo{}

	// Before delete hookpoint
	if v, ok := modelObj.(models.IBeforeDelete); ok {
		err = v.BeforeDeleteDB(db, oid, scope, typeString, &cargo)
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

	// // TODO: Probably just get tag and do a delete with raw SQL should be faster
	// // Remove association first
	// arr := reflect.Indirect(reflect.ValueOf(modelObj)).FieldByName("Ownerships")
	// for i := 0; i < arr.Len(); i++ {
	// 	log.Println("arr.Index(i).Interface():", arr.Index(i).Interface())
	// 	// if err := db.Model(modelObj).Association("Ownerships").Delete(arr.Index(i).Interface()).Error; err != nil {
	// 	// 	return nil, err
	// 	// }
	// }
	modelObjOwnership, ok := modelObj.(models.IHasOwnershipLink)
	if !ok {
		return nil, errNoOwnership
	}
	stmt := fmt.Sprintf("DELETE FROM %s WHERE user_id = ? AND model_id = ? AND role = ?", models.GetJoinTableName(modelObjOwnership))

	// Can't do db.Raw and db.Delete at the same time?!
	db2 := db.Exec(stmt, oid.String(), modelObj.GetID().String(), models.Admin)
	if db2.Error != nil {
		return nil, err
	}

	if db2.RowsAffected == 0 {
		// If guest, goes here, so no delete the actual model
		return nil, errPermission
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
		err = v.AfterDeleteDB(db, oid, scope, typeString, &cargo)
		if err != nil {
			return nil, err
		}
	}

	return modelObj, nil
}

// DeleteMany deletes multiple models
func (mapper *OwnershipMapper) DeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {

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
		if err = before(modelObjs, db, oid, scope, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	for i, id := range ids {

		if id.UUID.String() == "" {
			return nil, errIDEmpty
		}

		// Pull out entire modelObj
		modelObj, role, err := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}
		if role != models.Admin {
			return nil, errPermission
		}

		// Unscoped() for REAL delete!
		// Foreign key constraint works only on real delete
		// Soft delete will take more work, have to verify myself manually
		if modelNeedsRealDelete(modelObj) && i == 0 { // only do once
			db = db.Unscoped()
		}

		// Also remove entries from ownership table
		modelObjOwnership, ok := modelObj.(models.IHasOwnershipLink)
		if !ok {
			return nil, errNoOwnership
		}
		stmt := fmt.Sprintf("DELETE FROM %s WHERE user_id = ? AND model_id = ? AND role = ?", models.GetJoinTableName(modelObjOwnership))
		db2 := db.Exec(stmt, oid.String(), modelObj.GetID().String(), models.Admin)
		if db2.Error != nil {
			return nil, err
		}

		if db2.RowsAffected == 0 {
			// If guest, goes here, so no delete the actual model
			return nil, errPermission
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
		if err = after(modelObjs, db, oid, scope, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	return ms, nil
}
