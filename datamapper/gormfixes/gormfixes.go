package gormfixes

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// Remove strategy
// If pegassoc, record it as dissociate and you're done.
// If pegged, record it as to be removed., traverse into the struct. If then encounter a peg, record it
// to be removed and need to traverse into it further, if encounter a pegassoc, record it as to be
// dissociated.
// If no pegassoc and pegged under this struct, return.

type modelAndIds struct {
	modelObj models.IModel
	ids      []interface{} // to send to Gorm need to be interface not *datatypes.UUID
}

type cargo struct {
	peg map[string]modelAndIds
}

// DeleteModelFixManyToManyAndPeg remove nested field if it has tag \"betterrest="peg"\"
// Pegassoc is no problem, because we never tried to take care of it
// If necessary, DB foreign key constraint will do the job
func DeleteModelFixManyToManyAndPeg(db *gorm.DB, modelObj models.IModel) error {
	if err := removeManyToManyAssociationTableElem(db, modelObj); err != nil {
		return err
	}

	// Delete nested field
	// Not yet support two-level of nested field
	v := reflect.Indirect(reflect.ValueOf(modelObj))

	peg := make(map[string]modelAndIds) // key: name of the table to be deleted, val: list of ids
	car := cargo{peg: peg}

	if err := markForDelete(db, v, car); err != nil {
		return err
	}

	// Now actually delete
	for tblName := range car.peg {
		if err := db.Delete(car.peg[tblName].modelObj, car.peg[tblName].ids).Error; err != nil {
			return err
		}
	}

	return nil
}

// TODO: if there is a "pegassoc-manytomany" inside a pegged struct
// and we're deleting the pegged struct, the many-to-many relationship needs to be removed
func markForDelete(db *gorm.DB, v reflect.Value, car cargo) error {
	for i := 0; i < v.NumField(); i++ {
		t := pegPegassocOrPegManyToMany(v.Type().Field(i).Tag)
		if t == "peg" {
			switch v.Field(i).Kind() {
			case reflect.Struct:
				m := v.Field(i).Addr().Interface().(models.IModel)
				fieldTableName := models.GetTableNameFromIModel(m)
				if _, ok := car.peg[fieldTableName]; ok {
					mids := car.peg[fieldTableName]
					mids.ids = append(mids.ids, m.GetID())
					car.peg[fieldTableName] = mids
				} else {
					arr := make([]interface{}, 1)
					arr[0] = m.GetID()
					car.peg[fieldTableName] = modelAndIds{modelObj: m, ids: arr}
				}
				// Traverse into it
				if err := markForDelete(db, v.Field(i), car); err != nil {
					return err
				}
			case reflect.Slice:
				typ := v.Type().Field(i).Type.Elem()
				m, _ := reflect.New(typ).Interface().(models.IModel)
				fieldTableName := models.GetTableNameFromIModel(m)
				for j := 0; j < v.Field(i).Len(); j++ {
					if _, ok := car.peg[fieldTableName]; ok {
						mids := car.peg[fieldTableName]
						mids.ids = append(mids.ids, v.Field(i).Index(j).Addr().Interface().(models.IModel).GetID())
						car.peg[fieldTableName] = mids
					} else {
						arr := make([]interface{}, 1)
						arr[0] = v.Field(i).Index(j).Addr().Interface().(models.IModel).GetID()
						car.peg[fieldTableName] = modelAndIds{modelObj: m, ids: arr}
					}

					// Can it be a pointer type inside?, then unbox it in the next recursion
					if err := markForDelete(db, v.Field(i).Index(j), car); err != nil {
						return err
					}
				}
			case reflect.Ptr:
				// Unbox the pointer
				if err := markForDelete(db, v.Elem(), car); err != nil {
					return err
				}
			}
		} else if strings.HasPrefix(t, "pegassoc-manytomany") {
			// We're deleting. And now we have a many to many in here
			// Remove the many to many
			var m models.IModel
			switch v.Field(i).Kind() {
			case reflect.Struct:
				m = v.Field(i).Addr().Interface().(models.IModel)
			case reflect.Slice:
				typ := v.Type().Field(i).Type.Elem()
				m = reflect.New(typ).Interface().(models.IModel)
			case reflect.Ptr:
				m = v.Elem().Interface().(models.IModel)
			}
			if err := removeManyToManyAssociationTableElem(db, m); err != nil {
				return err
			}
		}
	}
	return nil
}

func removeManyToManyAssociationTableElem(db *gorm.DB, modelObj models.IModel) error {
	// many to many, here we remove the entry in the actual immediate table
	// because that's actually the link table. Thought we don't delete the
	// Model table itself
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	for i := 0; i < v.NumField(); i++ {
		tag := v.Type().Field(i).Tag.Get("betterrest")
		if strings.HasPrefix(tag, "pegassoc-manytomany") {
			// many to many, here we remove the entry in the actual immediate table
			// because that's actually the link table. Thought we don't delete the
			// Model table itself

			// The normal Delete(model, ids) doesn't quite work because
			// I don't have access to the model, it's not registered as typestring
			// nor part of the field type. It's a joining table between many to many

			linkTableName := strings.Split(tag, ":")[1]
			typ := v.Type().Field(i).Type.Elem() // Get the type of the element of slice
			m2, _ := reflect.New(typ).Interface().(models.IModel)
			fieldTableName := models.GetTableNameFromIModel(m2)
			selfTableName := models.GetTableNameFromIModel(modelObj)

			fieldVal := v.Field(i)
			if fieldVal.Len() >= 1 {
				uuidStmts := strings.Repeat("?,", fieldVal.Len())
				uuidStmts = uuidStmts[:len(uuidStmts)-1]

				allIds := make([]interface{}, 0, 10)
				allIds = append(allIds, modelObj.GetID().String())
				for j := 0; j < fieldVal.Len(); j++ {
					idToDel := fieldVal.Index(j).FieldByName("ID").Interface().(*datatypes.UUID)
					allIds = append(allIds, idToDel.String())
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

func pegPegassocOrPegManyToMany(tag reflect.StructTag) string {
	for _, tag := range strings.Split(tag.Get("betterrest"), ";") {
		if tag == "peg" || tag == "pegassoc" || strings.HasPrefix(tag, "pegassoc-manytomany") {
			return tag
		}
	}
	return ""
}

// CreatePeggedAssocFields :-
func CreatePeggedAssocFields(db *gorm.DB, modelObj models.IModel) (err error) {
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

// UpdatePeggedFields check if stuff in the pegged array
// is actually
func UpdatePeggedFields(db *gorm.DB, oldModelObj models.IModel, newModelObj models.IModel) (err error) {
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
					if err := db.Delete(modelToDel).Error; err != nil {
						return err
					}
					// Similar to directly deleting the model,
					// just deleting it won't work, need to traverse down the chain
					if err := DeleteModelFixManyToManyAndPeg(db, modelToDel.(models.IModel)); err != nil {
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

// FixManyToMany :-
func FixManyToMany(correctModel models.IModel, incorrectModel models.IModel) (err error) {
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

// LoadManyToManyBecauseGormFailsWithID :-
func LoadManyToManyBecauseGormFailsWithID(db *gorm.DB, modelObj models.IModel) error {
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
