package datamapper

import (
	"encoding/json"
	"log"
	"reflect"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/jinzhu/gorm"
)

func checkErrorBeforeUpdate(mapper IGetOneWithIDMapper, db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel, id datatypes.UUID) error {
	if id.UUID.String() == "" {
		return errIDEmpty
	}

	// TODO: Is there a more efficient way?
	_, err := mapper.GetOneWithID(db, oid, typeString, id)
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
	if err = db.Save(modelObj).Error; err != nil { // save updates all fields (FIXME: need to check for required)
		log.Println("Error updating:", err)
		return nil, err
	}

	// This so we have the preloading.
	modelObj2, err = mapper.GetOneWithID(db, oid, typeString, id)
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
				log.Println("it's something else")
			}
		}
	}
	return nil
}
