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

var once sync.Once
var crud *BasicMapper

// BasicMapper is a basic CRUD manager
type BasicMapper struct {
}

// SharedBasicMapper creats a singleton of Crud object
func SharedBasicMapper() *BasicMapper {
	once.Do(func() {
		crud = &BasicMapper{}
	})

	return crud
}

// CreateOne creates an instance of this model based on json and store it in db
func (mapper *BasicMapper) CreateOne(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel) (models.IModel, error) {
	// db.Model(&u).Association("Ownerships")
	g := reflect.New(models.OwnershipTyp).Interface().(models.IRole)
	firstJoin := "INNER JOIN `user_ownerships` ON `ownership`.id = `user_ownerships`.ownership_id AND user_id=UUID_TO_BIN(?) AND role=?"
	err := db.Table("ownership").Joins(firstJoin, oid.String(), models.Admin).Find(g).Error
	if err != nil {
		log.Println("Error in finding ownership", err)
		return nil, err
	}

	// Associate a ownership group with this model
	o := reflect.ValueOf(modelObj).Elem().FieldByName("Ownerships")
	o.Set(reflect.Append(o, reflect.ValueOf(g).Elem()))

	return CreateWithHooks(db, oid, typeString, modelObj)
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *BasicMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj := models.NewFromTypeString(typeString)

	// Is this a global setting?
	db = db.Set("gorm:auto_preload", true)

	structName := reflect.TypeOf(modelObj).Elem().Name()
	columnName := letters.PascalCaseToSnakeCase(structName) // column name
	rtable := strings.ToLower(structName)                   // table name
	firstJoin := fmt.Sprintf("INNER JOIN `%s_ownerships` ON `%s`.id = `%s_ownerships`.%s_id AND `%s`.id = UUID_TO_BIN(?) ",
		rtable, rtable, rtable, columnName, rtable)
	// secondJoin := fmt.Sprintf("INNER JOIN `ownership` ON `ownership`.id = `%s_ownerships`.ownership_id AND `ownership`.role = ? ",
	// 	rtable)
	secondJoin := fmt.Sprintf("INNER JOIN `ownership` ON `ownership`.id = `%s_ownerships`.ownership_id",
		rtable)
	thirdJoin := "INNER JOIN `user_ownerships` ON `user_ownerships`.ownership_id = `ownership`.id "
	fourthJoin := "INNER JOIN user ON `user_ownerships`.user_id = user.id AND user.id = UUID_TO_BIN(?)"

	// hack
	db2 := db
	err := db.Table(rtable).Joins(firstJoin, id.String()).Joins(secondJoin).Joins(thirdJoin).Joins(fourthJoin, oid.String()).Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	roles := make([]models.UserRole, 0)
	err = db2.Table(rtable).Joins(firstJoin, id.String()).Joins(secondJoin).Joins(thirdJoin).Joins(fourthJoin, oid.String()).Select("ownership.role").Find(&roles).Error

	return modelObj, roles[0], err
}

// ReadAll obtains a slice of models.DomainModel
// options can be string "offset" and "limit", both of type int
// This is very Javascript-esque. I would have liked Python's optional parameter more.
// Alas, no such feature in Go. https://stackoverflow.com/questions/2032149/optional-parameters-in-go
// How does Gorm do the following? Might want to check out its source code.
// Cancel offset condition with -1
//  db.Offset(10).Find(&users1).Offset(-1).Find(&users2)
func (mapper *BasicMapper) ReadAll(db *gorm.DB, oid *datatypes.UUID, typeString string, options map[string]interface{}) ([]models.IModel, []models.UserRole, error) {
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
	columnName := letters.PascalCaseToSnakeCase(structName) // column name
	rtable := strings.ToLower(structName)                   // table name
	firstJoin := fmt.Sprintf("INNER JOIN `%s_ownerships` ON `%s`.id = `%s_ownerships`.%s_id ",
		rtable, rtable, rtable, columnName)

	// secondJoin := fmt.Sprintf("INNER JOIN `ownership` ON `ownership`.id = `%s_ownerships`.ownership_id AND `ownership`.role = ? ",
	// 	rtable)
	// Admin or guest doesn't mattter
	secondJoin := fmt.Sprintf("INNER JOIN `ownership` ON `ownership`.id = `%s_ownerships`.ownership_id", rtable)
	thirdJoin := "INNER JOIN `user_ownerships` ON `user_ownerships`.ownership_id = `ownership`.id "
	fourthJoin := "INNER JOIN user ON `user_ownerships`.user_id = user.id AND user.id = UUID_TO_BIN(?)"

	// whereStmt := ""
	// hasDateRange := false
	// whereArgs := make([]interface{}, 0)
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
	db2 := db
	db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin).Joins(thirdJoin).Joins(fourthJoin, oid.String())

	// TODO: this makes unnecessary query, but then then if I only want ONE query I gotta
	db2 = db2.Table(rtable).Joins(firstJoin).Joins(secondJoin).Joins(thirdJoin).Select("ownership.role").Joins(fourthJoin, oid.String())

	if order := options["order"].(string); order != "" {
		db = db.Order("created_at " + order)
		db2 = db2.Order("created_at " + order)
	}

	if limit != 0 {
		// rows.Scan()
		db = db.Offset(offset).Limit(limit)
		db2 = db2.Offset(offset).Limit(limit)
	}

	roles := make([]models.UserRole, 0)
	if err := db2.Find(&roles).Error; err != nil {
		return nil, nil, err
	}

	outmodels, err := models.NewSliceFromDB(typeString, db.Find) // error from db is returned from here

	// safeguard, Must be coded wrongly
	if len(outmodels) != len(roles) {
		return nil, nil, errors.New("unknown query error")
	}

	return outmodels, roles, err
}

// UpdateOneWithID updates model based on this json
func (mapper *BasicMapper) UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel, id datatypes.UUID) (models.IModel, error) {
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
func (mapper *BasicMapper) UpdateMany(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
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
func (mapper *BasicMapper) PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, jsonPatch []byte, id datatypes.UUID) (models.IModel, error) {
	// if err := checkErrorBeforeUpdate(mapper, db, oid, typeString, modelObj, id); err != nil {
	// 	return nil, err
	// }
	var modelObj models.IModel
	var err error
	cargo := models.ModelCargo{}

	// Check id error
	if id.UUID.String() == "" {
		return nil, errIDEmpty
	}

	var role models.UserRole
	if modelObj, role, err = mapper.GetOneWithID(db, oid, typeString, id); err != nil {
		return nil, err
	}

	if role != models.Admin {
		return nil, errPermission
	}

	// Before hook
	if v, ok := modelObj.(models.IBeforePatch); ok {
		if err := v.BeforePatchDB(db, oid, typeString, &cargo); err != nil {
			return nil, err
		}
	}

	// Apply patch operations
	modelObj, err = patchOneCore(typeString, modelObj, jsonPatch)
	if err != nil {
		return nil, err
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
func (mapper *BasicMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, id datatypes.UUID) (models.IModel, error) {
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
	modelObj, role, err := mapper.GetOneWithID(db, oid, typeString, id)
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

	err = db.Delete(modelObj).Error
	if err != nil {
		return nil, err
	}

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
func (mapper *BasicMapper) DeleteMany(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
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

	for _, id := range ids {

		if id.UUID.String() == "" {
			return nil, errIDEmpty
		}

		// Pull out entire modelObj
		modelObj, role, err := mapper.GetOneWithID(db, oid, typeString, id)
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}
		if role != models.Admin {
			return nil, errPermission
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
