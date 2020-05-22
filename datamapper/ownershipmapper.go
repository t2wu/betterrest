package datamapper

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/utils/letters"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

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
func (mapper *OwnershipMapper) CreateOne(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel) (models.IModel, error) {
	// db.Model(&u).Association("Ownerships")

	// Get ownership type and creates it
	// field, _ := reflect.TypeOf(modelObj).Elem().FieldByName("Ownerships")
	// ownershipType := field.Type

	ownershipType := modelObj.OwnershipType()

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

	return CreateWithHooks(db, oid, typeString, modelObj)
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *OwnershipMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error) {

	modelObj, role, err := mapper.getOneWithIDCore(db, oid, typeString, id)
	if err != nil {
		return nil, 0, err
	}

	if m, ok := modelObj.(models.IAfterRead); ok {
		if err := m.AfterReadDB(db, oid, typeString, &role); err != nil {
			return nil, 0, err
		}
	}

	return modelObj, role, err
}

// getOneWithIDCore get one model object based on its type and its id string
func (mapper *OwnershipMapper) getOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj := models.NewFromTypeString(typeString)

	db = db.Set("gorm:auto_preload", true)

	// o := reflect.ValueOf(modelObj).Elem().FieldByName("Ownerships")

	structName := reflect.TypeOf(modelObj).Elem().Name()
	rtable := strings.ToLower(structName) // table name

	/*
		SELECT * from some_model
		INNER JOIN user_owns_somemodel ON somemodel.id = user_owns_somemodel.model_id AND somemodel.id = UUID_TO_BIN(id)
		INNER JOIN user ON user.id = user_owns_somemodel.user_id AND user.id = UUID_TO_BIN(oid)
	*/

	joinTableName := getJoinTableName(modelObj)

	firstJoin := fmt.Sprintf("INNER JOIN `%s` ON `%s`.id = `%s`.model_id AND `%s`.id = UUID_TO_BIN(?)", joinTableName, rtable, joinTableName, rtable)
	secondJoin := fmt.Sprintf("INNER JOIN `user` ON `user`.id = `%s`.user_id AND `%s`.user_id = UUID_TO_BIN(?)", joinTableName, joinTableName)
	// db2 := db

	err := db.Table(rtable).Joins(firstJoin, id.String()).Joins(secondJoin, oid.String()).Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	joinTable := reflect.New(modelObj.OwnershipType()).Interface()
	role := models.Guest // just some default
	stmt := fmt.Sprintf("SELECT * FROM %s WHERE user_id = UUID_TO_BIN(?) AND model_id = UUID_TO_BIN(?)", joinTableName)
	if err2 := db.Raw(stmt, oid.String(), id.String()).Scan(joinTable).Error; err2 != nil {
		return nil, 0, err2
	}

	if m, ok := joinTable.(models.IOwnership); ok {
		role = m.GetRole()
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
func (mapper *OwnershipMapper) ReadAll(db *gorm.DB, oid *datatypes.UUID, typeString string, options map[string]interface{}) ([]models.IModel, []models.UserRole, error) {
	db2 := db
	offset, limit := 0, 0
	if _, ok := options["offset"]; ok {
		offset, _ = options["offset"].(int)
	}
	if _, ok := options["limit"]; ok {
		limit, _ = options["limit"].(int)
	}

	cstart, cstop := 0, 0
	if _, ok := options["cstart"]; ok {
		cstart, _ = options["cstart"].(int)
	}
	if _, ok := options["cstop"]; ok {
		cstop, _ = options["cstop"].(int)
	}

	// var f func(interface{}, ...interface{}) *gorm.DB
	// var f func(dest interface{}) *gorm.DB

	db = db.Set("gorm:auto_preload", true)

	structName := reflect.TypeOf(models.NewFromTypeString(typeString)).Elem().Name()
	rtable := strings.ToLower(structName) // table name
	joinTableName := getJoinTableName(models.NewFromTypeString(typeString))

	firstJoin := fmt.Sprintf("INNER JOIN `%s` ON `%s`.id = `%s`.model_id", joinTableName, rtable, joinTableName)
	secondJoin := fmt.Sprintf("INNER JOIN `user` ON `user`.id = `%s`.user_id AND `%s`.user_id = UUID_TO_BIN(?)", joinTableName, joinTableName)

	if cstart != 0 && cstop != 0 {
		db = db.Where(rtable+".created_at BETWEEN ? AND ?", time.Unix(int64(cstart), 0), time.Unix(int64(cstop), 0))
	}

	// any other fields?
	if values, ok := options["better_otherqueries"].(url.Values); ok {
		// Important!! Check if fieldName is actually part of the schema, otherwise risk of sequal injection

		obj := models.NewFromTypeString(typeString)
		v := reflect.Indirect(reflect.ValueOf(obj))
		typeOfS := v.Type()
		fieldMap := make(map[string]bool)

		// https://stackoverflow.com/questions/18926303/iterate-through-the-fields-of-a-struct-in-go
		for i := 0; i < v.NumField(); i++ {
			fieldMap[typeOfS.Field(i).Name] = true
		}

		for fieldName, fieldValues := range values {
			fieldValue2 := fieldValues[0]

			if fieldMap[letters.CamelCaseToPascalCase(fieldName)] == false {
				return nil, nil, fmt.Errorf("fieldname %s does not exist", fieldName)
			}

			whereStmt := rtable + "." + letters.PascalCaseToSnakeCase(fieldName) + " = ?"
			if strings.HasSuffix(fieldName, "ID") {
				uuid2, err := datatypes.NewUUIDFromString(fieldValue2)
				if err != nil {
					return nil, nil, err
				}
				db = db.Where(whereStmt, uuid2)
			} else {
				db = db.Where(whereStmt, fieldValue2)
			}
		}
	}

	// Admin or guest..doesn't matter
	// db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin, models.Admin).Joins(thirdJoin).Joins(fourthJoin, oid.String())
	// Hackish!
	// db2 := db

	db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin, oid.String())

	// roleField := fmt.Sprintf("%s.role", joinTableName)
	// db2 = db2.Table(rtable).Joins(firstJoin).Joins(secondJoin, oid.String()).Select(roleField)

	// db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin).Joins(thirdJoin).Joins(fourthJoin, oid.String())

	// TODO: this makes unnecessary query, but then then if I only want ONE query I gotta
	// db2 = db2.Table(rtable).Joins(firstJoin).Joins(secondJoin).Joins(thirdJoin).Select("ownership.role").Joins(fourthJoin, oid.String())

	if order := options["order"].(string); order != "" {
		stmt := fmt.Sprintf("`%s`.created_at %s", rtable, order)
		db = db.Order(stmt)
		// db2 = db2.Order(stmt)
	}

	if limit != 0 {
		// rows.Scan()
		db = db.Offset(offset).Limit(limit)
		// db2 = db2.Offset(offset).Limit(limit)
	}

	// Don't know why this doesn't work
	roles := make([]models.UserRole, 0)
	// if err := db2.Find(&roles).Error; err != nil {
	// 	return nil, nil, err
	// }
	outmodels, err := models.NewSliceFromDB(typeString, db.Find) // error from db is returned from here

	for _, outmodel := range outmodels {
		joinTable := reflect.New(outmodel.OwnershipType()).Interface()
		role := models.Admin // just some default
		mid := outmodel.GetID()
		stmt := fmt.Sprintf("SELECT * FROM %s WHERE user_id = UUID_TO_BIN(?) AND model_id = UUID_TO_BIN(?)", joinTableName)
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

	// use db2 cuz it's not chained
	if after := models.ModelRegistry[typeString].AfterRead; after != nil {
		if err = after(outmodels, db2, oid, typeString, roles); err != nil {
			return nil, nil, err
		}
	}

	return outmodels, roles, err
}

// UpdateOneWithID updates model based on this json
func (mapper *OwnershipMapper) UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel, id datatypes.UUID) (models.IModel, error) {
	if err := checkErrorBeforeUpdate(mapper, db, oid, typeString, modelObj, id); err != nil {
		return nil, err
	}

	cargo := models.ModelCargo{}

	// Before hook
	if v, ok := modelObj.(models.IBeforeUpdate); ok {
		if err := v.BeforeUpdateDB(db, oid, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	modelObj2, err := updateOneCore(mapper, db, oid, typeString, modelObj, id)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterUpdate); ok {
		if err = v.AfterUpdateDB(db, oid, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}

// UpdateMany updates multiple models
func (mapper *OwnershipMapper) UpdateMany(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	ms := make([]models.IModel, 0, 0)
	var err error
	cargo := models.BatchHookCargo{}

	// Before batch update hookopint
	if before := models.ModelRegistry[typeString].BeforeUpdate; before != nil {
		if err = before(modelObjs, db, oid, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	for _, modelObj := range modelObjs {
		id := modelObj.GetID()

		if err = checkErrorBeforeUpdate(mapper, db, oid, typeString, modelObj, *id); err != nil {
			return nil, err
		}

		m, err := updateOneCore(mapper, db, oid, typeString, modelObj, *id)
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}

		ms = append(ms, m)
	}

	// After batch update hookopint
	if after := models.ModelRegistry[typeString].AfterUpdate; after != nil {
		if err = after(modelObjs, db, oid, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	return ms, nil
}

// PatchOneWithID updates model based on this json
func (mapper *OwnershipMapper) PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, jsonPatch []byte, id datatypes.UUID) (models.IModel, error) {
	var modelObj models.IModel
	var err error
	cargo := models.ModelCargo{}
	var role models.UserRole

	// Check id error
	if id.UUID.String() == "" {
		return nil, errIDEmpty
	}

	// role already chcked in checkErrorBeforeUpdate
	if modelObj, role, err = mapper.getOneWithIDCore(db, oid, typeString, id); err != nil {
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
		if err := v.BeforePatchDB(db, oid, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	// Now save it
	modelObj2, err := updateOneCore(mapper, db, oid, typeString, modelObj, id)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterPatch); ok {
		if err = v.AfterPatchDB(db, oid, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}

// DeleteOneWithID delete the model
// TODO: delete the groups associated with this record?
func (mapper *OwnershipMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, id datatypes.UUID) (models.IModel, error) {
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
	modelObj, role, err := mapper.getOneWithIDCore(db, oid, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return nil, err
	}
	if role != models.Admin {
		return nil, errPermission
	}

	cargo := models.ModelCargo{}

	// Before delete hookpoint
	if v, ok := modelObj.(models.IBeforeDelete); ok {
		err = v.BeforeDeleteDB(db, oid, typeString, &cargo)
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

	stmt := fmt.Sprintf("DELETE FROM %s WHERE user_id = UUID_TO_BIN(?) AND model_id = UUID_TO_BIN(?) AND role = ?", getJoinTableName(modelObj))

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
		err = v.AfterDeleteDB(db, oid, typeString, &cargo)
		if err != nil {
			return nil, err
		}
	}

	return modelObj, nil
}

// DeleteMany deletes multiple models
func (mapper *OwnershipMapper) DeleteMany(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {

	log.Println("DeleteMany called 1")
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

	// Before batch delete hookopint
	if before := models.ModelRegistry[typeString].BeforeDelete; before != nil {
		if err = before(modelObjs, db, oid, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	log.Println("DeleteMany called 2")
	for _, id := range ids {

		if id.UUID.String() == "" {
			return nil, errIDEmpty
		}

		// Pull out entire modelObj
		modelObj, role, err := mapper.getOneWithIDCore(db, oid, typeString, id)
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}
		if role != models.Admin {
			return nil, errPermission
		}

		// Unscoped() for REAL delete!
		// Foreign key constraint works only on real delete
		// Soft delete will take more work, have to verify myself manually
		log.Println("DeleteMany called 3")
		if modelNeedsRealDelete(modelObj) {
			db = db.Unscoped()
		}

		log.Println("DeleteMany called 4")
		// Also remove entries from ownership table
		stmt := fmt.Sprintf("DELETE FROM %s WHERE user_id = UUID_TO_BIN(?) AND model_id = UUID_TO_BIN(?) AND role = ?", getJoinTableName(modelObj))
		db2 := db.Exec(stmt, oid.String(), modelObj.GetID().String(), models.Admin)
		if db2.Error != nil {
			return nil, err
		}

		log.Println("DeleteMany called 5")
		if db2.RowsAffected == 0 {
			// If guest, goes here, so no delete the actual model
			return nil, errPermission
		}

		log.Println("DeleteMany called 6")
		err = db.Delete(modelObj).Error
		// err = db.Delete(modelObj).Error
		if err != nil {
			return nil, err
		}

		log.Println("DeleteMany called 7")
		err = removePeggedField(db, modelObj)
		if err != nil {
			return nil, err
		}

		ms = append(ms, modelObj)
	}

	// After batch delete hookopint
	if after := models.ModelRegistry[typeString].AfterDelete; after != nil {
		if err = after(modelObjs, db, oid, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	return ms, nil
}