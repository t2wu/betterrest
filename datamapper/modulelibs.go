package datamapper

import (
	"encoding/json"
	"log"
	"reflect"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/utils/letters"
	"github.com/t2wu/betterrest/models"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/jinzhu/gorm"
)

func checkErrorBeforeUpdate(mapper IGetOneWithIDMapper, db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel, id datatypes.UUID) error {
	if id.UUID.String() == "" {
		return errIDEmpty
	}

	// TODO: Is there a more efficient way?
	_, _, err := mapper.GetOneWithIDCore(db, oid, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return err
	}

	uuidVal := modelObj.GetID()
	if uuidVal == nil || uuidVal.String() == "" {
		// in case it's empty string
		return errIDEmpty
	} else if uuidVal.String() != id.UUID.String() {
		return errIDNotMatch
	}

	return nil
}

func updateOneCore(mapper IGetOneWithIDMapper, db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel, id datatypes.UUID) (modelObj2 models.IModel, err error) {
	oldModelObj, role, err2 := mapper.GetOneWithIDCore(db, oid, typeString, id)
	if err2 != nil {
		return nil, err2
	}
	if role != models.Admin {
		return nil, errPermission
	}

	if modelNeedsRealDelete(oldModelObj) { // parent model
		db = db.Unscoped()
	}
	err2 = updatePeggedFields(db, oldModelObj, modelObj)
	if err2 != nil {
		return nil, err2
	}

	if err = db.Save(modelObj).Error; err != nil { // save updates all fields (FIXME: need to check for required)
		log.Println("Error updating:", err)
		return nil, err
	}

	// This so we have the preloading.
	modelObj2, _, err = mapper.GetOneWithID(db, oid, typeString, id)
	if err != nil { // Error is "record not found" when not found
		log.Println("Error:", err)
		return nil, err
	}

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

// removePeggedField remove nested field if it has tag `betterrest="peg"`
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
				// it's something else, delete it directly
				x := fieldVal.Interface()
				err = db.Delete(x).Error
				if err != nil {
					return err
				}
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
	v1 := reflect.Indirect(reflect.ValueOf(oldModelObj))
	v2 := reflect.Indirect(reflect.ValueOf(newModelObj))

	for i := 0; i < v1.NumField(); i++ {
		tag := v1.Type().Field(i).Tag.Get("betterrest")
		if tag == "peg" || tag == "pegassoc" {
			fieldVal1 := v1.Field(i)
			fieldVal2 := v2.Field(i)

			set1 := datatypes.NewSetString()
			set2 := datatypes.NewSetString()
			m := make(map[string]interface{})

			switch fieldVal1.Kind() {
			case reflect.Slice:
				for j := 0; j < fieldVal1.Len(); j++ {
					id := fieldVal1.Index(j).FieldByName("ID").Interface().(*datatypes.UUID)
					set1.Add(id.String())
					m[id.String()] = fieldVal1.Index(j).Interface()
				}

				for j := 0; j < fieldVal2.Len(); j++ {
					id := fieldVal2.Index(j).FieldByName("ID").Interface().(*datatypes.UUID)
					if id != nil {
						// ID doesn't exist? ignore, it's a new entry without ID
						set2.Add(id.String())
						m[id.String()] = fieldVal2.Index(j).Interface()
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
					// for data with its own endpoint, need to associate it
					columnName := v1.Type().Field(i).Name
					// assocModel := reflect.Indirect(reflect.ValueOf(modelToAdd)).Type().Name()
					// fieldName := v1.Type().Field(i).Name
					// fieldName = fieldName[0 : len(fieldName)-1] // get rid of s
					// tableName := letters.CamelCaseToPascalCase(fieldName)
					if err = db.Set("gorm:association_autoupdate", true).Model(oldModelObj).Association(columnName).Append(modelToAdd).Error; err != nil {
						return err
					}
				}
			}
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

func getJoinTableName(modelObj models.IModel) string {
	if m, ok := reflect.New(modelObj.OwnershipType()).Interface().(models.IHasTableName); ok {
		return m.TableName()
	}

	typeName := modelObj.OwnershipType().Name()
	return letters.PascalCaseToSnakeCase(typeName)
}
