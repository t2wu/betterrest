package datamapper

import (
	"fmt"
	"log"
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
	// log.Println("LENGTH BEFORE APPEND:", o.Len())
	o.Set(reflect.Append(o, reflect.ValueOf(g).Elem()))

	return CreateWithHooks(db, oid, typeString, modelObj)
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *BasicMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, typeString string, id datatypes.UUID) (models.IModel, error) {
	modelObj := models.NewFromTypeString(typeString)

	// Is this a global setting?
	db = db.Set("gorm:auto_preload", true)

	structName := reflect.TypeOf(modelObj).Elem().Name()
	columnName := letters.PascalCaseToSnakeCase(structName) // column name
	rtable := strings.ToLower(structName)                   // table name
	firstJoin := fmt.Sprintf("INNER JOIN `%s_ownerships` ON `%s`.id = `%s_ownerships`.%s_id AND `%s`.id = UUID_TO_BIN(?) ",
		rtable, rtable, rtable, columnName, rtable)
	secondJoin := fmt.Sprintf("INNER JOIN `ownership` ON `ownership`.id = `%s_ownerships`.ownership_id AND `ownership`.role = ? ",
		rtable)
	thirdJoin := "INNER JOIN `user_ownerships` ON `user_ownerships`.ownership_id = `ownership`.id "
	fourthJoin := "INNER JOIN user ON `user_ownerships`.user_id = user.id AND user.id = UUID_TO_BIN(?)"
	err := db.Table(rtable).Joins(firstJoin, id.String()).Joins(secondJoin, models.Admin).Joins(thirdJoin).Joins(fourthJoin, oid.String()).Find(modelObj).Error

	return modelObj, err
}

// ReadAll obtains a slice of models.DomainModel
// options can be string "offset" and "limit", both of type int
// This is very Javascript-esque. I would have liked Python's optional parameter more.
// Alas, no such feature in Go. https://stackoverflow.com/questions/2032149/optional-parameters-in-go
// How does Gorm do the following? Might want to check out its source code.
// Cancel offset condition with -1
//  db.Offset(10).Find(&users1).Offset(-1).Find(&users2)
func (mapper *BasicMapper) ReadAll(db *gorm.DB, oid *datatypes.UUID, typeString string, options map[string]interface{}) ([]models.IModel, error) {
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

	// This is GetAllByField
	fieldName, fieldValue, fieldNamePascal := "", "", ""
	_, ok := options["fieldName"]
	_, ok2 := options["fieldValue"]
	if ok && ok2 {
		// FIXME this should return NewErrQueryParameter when you can't conver it
		// But we should hanlde it at the caller
		fieldName, _ = options["fieldName"].(string)
		fieldNamePascal = letters.CamelCaseToPascalCase(fieldName)
		fieldValue, _ = options["fieldValue"].(string)
	}

	var f func(interface{}, ...interface{}) *gorm.DB

	db = db.Set("gorm:auto_preload", true)

	structName := reflect.TypeOf(models.NewFromTypeString(typeString)).Elem().Name()
	columnName := letters.PascalCaseToSnakeCase(structName) // column name
	rtable := strings.ToLower(structName)                   // table name
	firstJoin := fmt.Sprintf("INNER JOIN `%s_ownerships` ON `%s`.id = `%s_ownerships`.%s_id ",
		rtable, rtable, rtable, columnName)
	secondJoin := fmt.Sprintf("INNER JOIN `ownership` ON `ownership`.id = `%s_ownerships`.ownership_id AND `ownership`.role = ? ",
		rtable)
	thirdJoin := "INNER JOIN `user_ownerships` ON `user_ownerships`.ownership_id = `ownership`.id "
	fourthJoin := "INNER JOIN user ON `user_ownerships`.user_id = user.id AND user.id = UUID_TO_BIN(?)"
	db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin, models.Admin).Joins(thirdJoin).Joins(fourthJoin, oid.String())

	whereStmt := ""
	hasDateRange := false
	whereArgs := make([]interface{}, 0)
	if cstart != 0 && cstop != 0 {
		// db = db.Where(rtable+".created_at BETWEEN ? AND ?", time.Unix(int64(cstart), 0), time.Unix(int64(cstop), 0))
		whereStmt = rtable + ".created_at BETWEEN ? AND ?"
		hasDateRange = true
		whereArgs = append(whereArgs, time.Unix(int64(cstart), 0))
		whereArgs = append(whereArgs, time.Unix(int64(cstop), 0))
	}

	if fieldName != "" && fieldValue != "" {
		// Important!! Check if fieldName is actually part of the schema, otherwise risk of sequal injection
		obj := models.NewFromTypeString(typeString)
		found := false

		// https://stackoverflow.com/questions/18926303/iterate-through-the-fields-of-a-struct-in-go
		v := reflect.ValueOf(obj).Elem()
		typeOfS := v.Type()

		// Loop, cuz FieldByName will return zero value if not found, maybe not helpful
		for i := 0; i < v.NumField(); i++ {
			if typeOfS.Field(i).Name == fieldNamePascal {
				found = true
				break
			}
		}

		if found {
			if hasDateRange {
				whereStmt = whereStmt + " AND"
			}

			whereStmt = whereStmt + letters.PascalCaseToSnakeCase(fieldName) + " = ?"
			if strings.HasSuffix(fieldName, "ID") {
				uuid_, err := datatypes.NewUUIDFromString(fieldValue)
				if err != nil {
					return nil, err
				}
				whereArgs = append(whereArgs, uuid_)
			} else {
				whereArgs = append(whereArgs, fieldValue)
			}
		} else {
			return nil, fmt.Errorf("fieldname %s does not exist", fieldName)
		}
	}

	if whereStmt != "" {
		db = db.Where(whereStmt, whereArgs)
	}

	if limit != 0 {
		f = db.Offset(offset).Limit(limit).Find
	} else {
		f = db.Find
	}

	return models.NewSliceFromDB(typeString, f) // error from db is returned from here
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

	if modelObj, err = mapper.GetOneWithID(db, oid, typeString, id); err != nil {
		return nil, err
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
	modelObj, err := mapper.GetOneWithID(db, oid, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return nil, err
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
	// Otherwise my constraint won't work...
	// Soft delete will take more work, have to verify myself manually
	// db.Unscoped().Delete(modelObj).Error
	err = db.Delete(modelObj).Error
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
		modelObj, err := mapper.GetOneWithID(db, oid, typeString, id)
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}

		// Unscoped() for REAL delete!
		// Otherwise my constraint won't work...
		// Soft delete will take more work, have to verify myself manually
		// db.Unscoped().Delete(modelObj).Error
		err = db.Delete(modelObj).Error
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
