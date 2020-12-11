package datamapper

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/jinzhu/gorm"
)

// URLParam is the URL parameter
type URLParam string

const (
	URLParamOffset       URLParam = "offset"
	URLParamLimit        URLParam = "limit"
	URLParamOrder        URLParam = "order"
	URLParamLatestN      URLParam = "latestn"
	URLParamCstart       URLParam = "cstart"
	URLParamCstop        URLParam = "cstop"
	URLParamOtherQueries URLParam = "better_otherqueries"
)

func getOptions(options map[URLParam]interface{}) (offset *int, limit *int, cstart *int, cstop *int, order *string, latestn *int) {
	// If key is in it, even if value is nil, ok will be true

	if _, ok := options[URLParamOffset]; ok {
		offset, _ = options[URLParamOffset].(*int)
	}

	if _, ok := options[URLParamLimit]; ok {
		limit, _ = options[URLParamLimit].(*int)
	}

	if _, ok := options[URLParamOrder]; ok {
		order, _ = options[URLParamOrder].(*string)
	}

	if _, ok := options[URLParamCstart]; ok {
		cstart, _ = options[URLParamCstart].(*int)
	}
	if _, ok := options[URLParamCstop]; ok {
		cstop, _ = options[URLParamCstop].(*int)
	}

	latestn = nil
	if n, ok := options[URLParamLatestN]; ok {
		if n != nil {
			if n2, err := strconv.Atoi(*(n.(*string))); err == nil {
				latestn = &n2
			}
		}
	}

	return offset, limit, cstart, cstop, order, latestn
}

func getModelTableNameAndJoinTableNameFromTypeString(typeString string) (string, string, error) {
	modelObjOwnership, ok := models.NewFromTypeString(typeString).(models.IHasOwnershipLink)
	if !ok {
		return "", "", errNoOwnership
	}
	joinTableName := models.GetJoinTableName(modelObjOwnership)
	modelTableName := models.GetTableNameFromTypeString(typeString)
	return modelTableName, joinTableName, nil
}

func getOwnershipModelTypeFromTypeString(typeString string) reflect.Type {
	modelObjOwnership, _ := models.NewFromTypeString(typeString).(models.IHasOwnershipLink)
	return modelObjOwnership.OwnershipType()
}

func constructInnerFieldParamQueries(db *gorm.DB, typeString string, options map[URLParam]interface{}, latestn *int) (*gorm.DB, error) {
	if urlParams, ok := options[URLParamOtherQueries].(url.Values); ok && len(urlParams) != 0 {
		var err error
		// If I want quering into nested data
		// I need INNER JOIN that table where the field is what we search for,
		// and that table's link back to this ID is the id of this table
		db, err = constructDbFromURLFieldQuery(db, typeString, urlParams, latestn)
		if err != nil {
			return nil, err
		}

		db, err = constructDbFromURLInnerFieldQuery(db, typeString, urlParams, latestn)
		if err != nil {
			return nil, err
		}
	} else if latestn != nil {
		return nil, errors.New("latestn cannot be used without querying field value")
	}

	return db, nil
}

func constructOrderFieldQueries(db *gorm.DB, tableName string, order *string) *gorm.DB {
	if order != nil && *order == "asc" {
		db = db.Order(fmt.Sprintf("\"%s\".created_at ASC", tableName))
	} else {
		db = db.Order(fmt.Sprintf("\"%s\".created_at DESC", tableName)) // descending by default
	}
	return db
}

func checkErrorBeforeUpdate(mapper IGetOneWithIDMapper, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id datatypes.UUID, permittedRole models.UserRole) error {
	if id.UUID.String() == "" {
		return errIDEmpty
	}

	// If you're able to read, you hvea the permission to update...
	// Not really now you have to check role
	// TODO: Is there a more efficient way?
	_, role, err := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return err
	}
	if role != permittedRole {
		return errPermission
	}

	uuidVal := modelObj.GetID()
	if uuidVal == nil || uuidVal.String() == "" {
		// in case it's an empty string
		return errIDEmpty
	} else if uuidVal.String() != id.UUID.String() {
		return errIDNotMatch
	}

	return nil
}

func updateOneCore(mapper IGetOneWithIDMapper, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id datatypes.UUID, permittedRole models.UserRole) (modelObj2 models.IModel, err error) {
	oldModelObj, role, err2 := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err2 != nil {
		return nil, err2
	}

	log.Println("===>role is:", role)

	if role != permittedRole {
		return nil, errPermission
	}

	if modelNeedsRealDelete(oldModelObj) { // parent model
		db = db.Unscoped()
	}
	err2 = updatePeggedFields(db, oldModelObj, modelObj)
	if err2 != nil {
		return nil, err2
	}

	// For some unknown reason
	// insert many-to-many works cuz Gorm does and works???
	// [2020-05-22 18:50:17]  [1.63ms]  INSERT INTO \"dock_group\" (\"group_id\",\"dock_id\") SELECT '<binary>','<binary>' FROM DUAL WHERE NOT EXISTS (SELECT * FROM \"dock_group\" WHERE \"group_id\" = '<binary>' AND \"dock_id\" = '<binary>')
	// [0 rows affected or returned ]

	// (/Users/t2wu/Documents/Go/pkg/mod/github.com/t2wu/betterrest@v0.1.19/datamapper/modulelibs.go:62)
	// [2020-05-22 18:50:17]  [1.30ms]  UPDATE \"dock\" SET \"updated_at\" = '2020-05-22 18:50:17', \"deleted_at\" = NULL, \"name\" = '', \"model\" = '', \"serial_no\" = '', \"mac\" = '', \"hub_id\" = NULL, \"is_online\" = false, \"room_id\" = NULL  WHERE \"dock\".\"deleted_at\" IS NULL AND \"dock\".\"id\" = '{2920e86e-33b1-4848-a773-e68e5bde4fc0}'
	// [1 rows affected or returned ]

	// (/Users/t2wu/Documents/Go/pkg/mod/github.com/t2wu/betterrest@v0.1.19/datamapper/modulelibs.go:62)
	// [2020-05-22 18:50:17]  [2.84ms]  INSERT INTO \"dock_group\" (\"dock_id\",\"group_id\") SELECT ') �n3�HH�s�[�O�','<binary>' FROM DUAL WHERE NOT EXISTS (SELECT * FROM \"dock_group\" WHERE \"dock_id\" = ') �n3�HH�s�[�O�' AND \"group_id\" = '<binary>')
	// [1 rows affected or returned ]
	if err = db.Save(modelObj).Error; err != nil { // save updates all fields (FIXME: need to check for required)
		log.Println("Error updating:", err)
		return nil, err
	}

	// This so we have the preloading.
	modelObj2, _, err = mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil { // Error is "record not found" when not found
		log.Println("Error:", err)
		return nil, err
	}

	// ouch! for many to many we need to remove it again!!
	// because it's in a transaction so it will load up again
	fixManyToMany(modelObj, modelObj2)

	return modelObj2, nil
}

func patchOneCore(typeString string, modelObj models.IModel, jsonPatch []byte) (modelObj2 models.IModel, err error) {
	// Apply patch operations
	// This library actually works in []byte

	var modelInBytes []byte
	modelInBytes, err = json.Marshal(modelObj)
	if err != nil {
		return nil, errPatch // the errors often not that helpful anyway
	}

	var patch jsonpatch.Patch
	patch, err = jsonpatch.DecodePatch(jsonPatch)
	if err != nil {
		return nil, err
	}

	var modified []byte
	modified, err = patch.Apply(modelInBytes)
	if err != nil {
		return nil, err
	}

	// Now turn it back to modelObj
	modelObj = models.NewFromTypeString(typeString)
	err = json.Unmarshal(modified, modelObj)
	if err != nil {
		// there shouldn't be any error unless it's a patching mistake...
		return nil, err
	}

	return modelObj, nil
}

// removePeggedField remove nested field if it has tag \"betterrest="peg"\"
// Only support one-level
func removePeggedField(db *gorm.DB, modelObj models.IModel) (err error) {
	// Delete nested field
	// Not yet support two-level of nested field
	v := reflect.Indirect(reflect.ValueOf(modelObj))

	for i := 0; i < v.NumField(); i++ {
		tag := v.Type().Field(i).Tag.Get("betterrest")
		if tag == "peg" {
			fieldVal := v.Field(i)
			switch fieldVal.Kind() {
			case reflect.Slice:
				for j := 0; j < fieldVal.Len(); j++ {
					x := fieldVal.Index(j).Interface()
					err = db.Delete(x).Error

					if err != nil {
						return err
					}
				}
			default:
				if !fieldVal.FieldByName("ID").IsNil() {
					x := fieldVal.Interface()
					err = db.Delete(x).Error
					if err != nil {
						return err
					}
				}
			}
		} else if strings.HasPrefix(tag, "pegassoc-manytomany") {
			// many to many, here we remove the entry in the actual immediate table
			// because that's actually the link table. Thought we don't delete the
			// Model table itself
			linkTableName := strings.Split(tag, ":")[1]

			// selfTableName := models.GetTableNameFromIModel(correctModel)
			// selfID := selfTableName + "_id"
			inter := v.Field(i).Interface()
			typ := reflect.TypeOf(inter).Elem() // Get the type of the element of slice
			m2, _ := reflect.New(typ).Interface().(models.IModel)
			fieldTableName := models.GetTableNameFromIModel(m2)
			selfTableName := models.GetTableNameFromIModel(modelObj)

			// fieldTableName := models.GetTableNameFromIModel(m2)
			fieldVal := v.Field(i)

			if fieldVal.Len() >= 1 {
				uuidStmts := strings.Repeat("?,", fieldVal.Len())

				uuidStmts = uuidStmts[:len(uuidStmts)-1]

				allIds := make([]interface{}, 0, 10)
				allIds = append(allIds, modelObj.GetID().String())
				for j := 0; j < fieldVal.Len(); j++ {
					idToDel := fieldVal.Index(j).FieldByName("ID").Interface().(*datatypes.UUID)
					allIds = append(allIds, idToDel.String())
					// idToDel := reflect.Indirect(reflect.ValueOf(modelToDel)).FieldByName("ID").Interface()

					// stmt := fmt.Sprintf("DELETE FROM \"%s\" WHERE \"%s\" = ? AND \"%s\" = ?",
					// 	linkTableName, selfTableName+"_id", fieldTableName+"_id")
					// err := db.Exec(stmt, modelObj.GetID().String(), idToDel.String()).Error
					// if err != nil {
					// 	return err
					// }
				}

				stmt := fmt.Sprintf("DELETE FROM \"%s\" WHERE \"%s\" = ? AND \"%s\" IN (%s)",
					linkTableName, selfTableName+"_id", fieldTableName+"_id", uuidStmts)
				err := db.Exec(stmt, allIds...).Error
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func createPeggedAssocFields(db *gorm.DB, modelObj models.IModel) (err error) {
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	for i := 0; i < v.NumField(); i++ {
		tag := v.Type().Field(i).Tag.Get("betterrest")
		// columnName := v.Type().Field(i).Name
		if tag == "pegassoc" {
			fieldVal := v.Field(i)
			switch fieldVal.Kind() {
			case reflect.Slice:
				// Loop through the slice
				for j := 0; j < fieldVal.Len(); j++ {
					// nestedModelID := fieldVal.Index(j).FieldByName("ID").Interface().(*datatypes.UUID)
					nestedModel := fieldVal.Index(j).Addr().Interface()

					// Load the full model
					if err = db.First(nestedModel).Error; err != nil {
						return err
					}

					tableName := models.GetTableNameFromIModel(modelObj)
					correspondingColumnName := tableName + "_id"

					db.Model(nestedModel).Update(correspondingColumnName, modelObj.GetID())

					// // this loops forever unlike update, why?
					// if err = db.Set("gorm:association_autoupdate", true).Model(modelObj).Association(columnName).Append(nestedModel).Error; err != nil {
					// 	return err
					// }
				}

			default:
				// embedded object is considered part of the structure, so no removal
			}
		}
	}
	return nil
}

// updatePeggedFields check if stuff in the pegged array
// is actually
func updatePeggedFields(db *gorm.DB, oldModelObj models.IModel, newModelObj models.IModel) (err error) {
	// Delete nested field
	// Not yet support two-level of nested field

	// Indirect is dereference
	// Interface() is extract content than re-wrap to interface
	// Since reflect.New() returns pointer to the object, after reflect.ValueOf
	// We need to deference it, hence "Indirect", now v1 and v2 are the actual object, not
	// ptr to objects
	v1 := reflect.Indirect(reflect.ValueOf(oldModelObj))
	v2 := reflect.Indirect(reflect.ValueOf(newModelObj))

	for i := 0; i < v1.NumField(); i++ {
		tag := v1.Type().Field(i).Tag.Get("betterrest")

		if tag == "peg" || tag == "pegassoc" || strings.HasPrefix(tag, "pegassoc-manytomany") {
			fieldVal1 := v1.Field(i)
			fieldVal2 := v2.Field(i)

			set1 := datatypes.NewSetString()
			set2 := datatypes.NewSetString()
			m := make(map[string]interface{})

			switch fieldVal1.Kind() {
			case reflect.Slice:
				// Loop through the slice
				for j := 0; j < fieldVal1.Len(); j++ {
					// For example, each fieldVal1.Index(j) is a "Dock{}"
					id := fieldVal1.Index(j).FieldByName("ID").Interface().(*datatypes.UUID)
					set1.Add(id.String())

					m[id.String()] = fieldVal1.Index(j).Addr().Interface() // re-wrap a dock
					// log.Println("----> tim: fieldVal1's type?", fieldVal1.Index(j).Type())
				}

				for j := 0; j < fieldVal2.Len(); j++ {
					id := fieldVal2.Index(j).FieldByName("ID").Interface().(*datatypes.UUID)
					if id != nil {
						// ID doesn't exist? ignore, it's a new entry without ID
						set2.Add(id.String())
						m[id.String()] = fieldVal2.Index(j).Addr().Interface()
					}
				}
			default:
				// embedded object is considered part of the structure, so no removal
			}

			// remove when stuff in the old set that's not in the new set
			setIsGone := set1.Difference(set2)

			for uuid := range setIsGone.List {
				modelToDel := m[uuid]

				if tag == "peg" {
					err = db.Delete(modelToDel).Error
					if err != nil {
						return err
					}
				} else if tag == "pegassoc" {
					columnName := v1.Type().Field(i).Name
					// assocModel := reflect.Indirect(reflect.ValueOf(modelToDel)).Type().Name()
					// fieldName := v1.Type().Field(i).Name
					// fieldName = fieldName[0 : len(fieldName)-1] // get rid of s
					// tableName := letters.CamelCaseToPascalCase(fieldName)
					if err = db.Model(oldModelObj).Association(columnName).Delete(modelToDel).Error; err != nil {
						return err
					}
				} else if strings.HasPrefix(tag, "pegassoc-manytomany") {
					// many to many, here we remove the entry in the actual immediate table
					// because that's actually the link table. Thought we don't delete the
					// Model table itself
					linkTableName := strings.Split(tag, ":")[1]
					// Get the base type of this field

					inter := v1.Field(i).Interface()
					typ := reflect.TypeOf(inter).Elem() // Get the type of the element of slice
					m2, _ := reflect.New(typ).Interface().(models.IModel)

					fieldTableName := models.GetTableNameFromIModel(m2)
					fieldIDName := fieldTableName + "_id"

					selfTableName := models.GetTableNameFromIModel(oldModelObj)
					selfID := selfTableName + "_id"

					// The following line seems to puke on a many-to-many, I hope I don't need it anywhere
					// else in another many-to-many
					// idToDel := reflect.Indirect(reflect.ValueOf(modelToDel)).Elem().FieldByName("ID").Interface()
					idToDel := reflect.Indirect(reflect.ValueOf(modelToDel)).FieldByName("ID").Interface()

					stmt := fmt.Sprintf("DELETE FROM \"%s\" WHERE \"%s\" = ? AND \"%s\" = ?",
						linkTableName, fieldIDName, selfID)
					err := db.Exec(stmt, idToDel.(*datatypes.UUID).String(), oldModelObj.GetID().String()).Error
					if err != nil {
						return err
					}

				}
			}

			setIsNew := set2.Difference(set1)
			for uuid := range setIsNew.List {
				modelToAdd := m[uuid]

				if tag == "peg" {
					// Don't need peg, because gorm already does auto-create by default
					// for truly nested data without endpoint
					// err = db.Save(modelToAdd).Error
					// if err != nil {
					// 	return err
					// }
				} else if tag == "pegassoc" {
					columnName := v1.Type().Field(i).Name
					// id, _ := reflect.ValueOf(modelToAdd).Elem().FieldByName(("ID")).Interface().(*datatypes.UUID)

					// Load the full model
					if err = db.First(modelToAdd).Error; err != nil {
						return err
					}

					if err = db.Set("gorm:association_autoupdate", true).Model(oldModelObj).Association(columnName).Append(modelToAdd).Error; err != nil {
						return err
					}
				} else if strings.HasPrefix(tag, "pegassoc-manytomany") {
					// No need either, Gorm already creates it
					// It's the preloading that's the issue.
				}
			}
		}
	}

	return nil
}

func fixManyToMany(correctModel models.IModel, incorrectModel models.IModel) (err error) {
	// Copy many to many from the correct to the incorrect model

	v1 := reflect.Indirect(reflect.ValueOf(correctModel))
	v2 := reflect.Indirect(reflect.ValueOf(incorrectModel))

	for i := 0; i < v1.NumField(); i++ {
		tag := v1.Type().Field(i).Tag.Get("betterrest")
		// log.Println("tag:", tag)
		if strings.HasPrefix(tag, "pegassoc-manytomany") {
			v2.Field(i).Set(v1.Field(i))
		}
	}

	return nil
}

func modelNeedsRealDelete(modelObj models.IModel) bool {
	// real delete by default
	realDelete := true
	if modelObj2, ok := modelObj.(models.IDoRealDelete); ok {
		realDelete = modelObj2.DoRealDelete()
	}
	return realDelete
}

func loadManyToManyBecauseGormFailsWithID(db *gorm.DB, modelObj models.IModel) error {
	v1 := reflect.Indirect(reflect.ValueOf(modelObj))

	for i := 0; i < v1.NumField(); i++ {
		tag := v1.Type().Field(i).Tag.Get("betterrest")

		// log.Println("tag:", tag)
		if strings.HasPrefix(tag, "pegassoc-manytomany") {
			tableName := models.GetTableNameFromIModel(reflect.ValueOf(modelObj).Interface().(models.IModel))

			linkTableName := strings.Split(tag, ":")[1]

			// Get the base type of this field
			inter := v1.Field(i).Interface()
			typ := reflect.TypeOf(inter).Elem() // Get the type of the element of slice

			m2, _ := reflect.New(typ).Interface().(models.IModel)
			fieldTableName := models.GetTableNameFromIModel(m2)

			sliceOfField := reflect.New(reflect.TypeOf(inter))

			join1 := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".\"id\" = \"%s\".\"%s\"", linkTableName, fieldTableName,
				linkTableName, fieldTableName+"_id")
			select1 := fmt.Sprintf("\"%s\".*", fieldTableName)
			where1 := fmt.Sprintf("\"%s\".\"%s\" = ?", linkTableName, tableName+"_id")

			err := db.Table(fieldTableName).Joins(join1).Where(where1, modelObj.GetID().String()).Select(select1).Find(sliceOfField.Interface()).Error
			if err != nil {
				return err
			}

			// 1. This just set it
			v1.Field(i).Set(sliceOfField.Elem())

			// 2. This is append
			// o := v1.Field(i)
			// s := sliceOfField.Elem()
			// for j := 0; j < s.Len(); j++ {
			// 	o.Set(reflect.Append(o, s.Index(j)))
			// }
		}
	}
	return nil
}
