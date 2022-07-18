package service

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/libs/gotag"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

// It is assumed that modelObjs are all from the same table (same IModel)
// If there is a third-level, it is not performant because it's depth first right now
// Wow, this is really difficult.
func RecursivelyQueryAllPeggedModels(db *gorm.DB, modelObjs []mdl.IModel, begin time.Time, end time.Time) error {
	result := mdl.NewPeggedIDSearch()

	for _, modelObj := range modelObjs {
		// This will attempt to dig all the way in, but since our model has only just been fetched at the top-most level
		// It can only dig one level down anyway, we'll do our recursion here
		mdl.FindAllBetterRestPeggOrPegAssocIDs(modelObj, result)
		// rtable := mdl.GetTableNameFromIModel(modelObj)
	}

	// query everything
	for parentTableName, inner := range result.ToProcess {
		for fieldAsKey, modelAndIDs := range inner {
			if fieldAsKey.Rel != mdl.RelationPeg {
				continue
			}

			// Query all
			slice := reflect.New(reflect.SliceOf(reflect.TypeOf(modelAndIDs.ModelObj)))
			sliceElem := slice.Elem()
			sliceI := slice.Interface()

			tableName := mdl.GetTableNameFromIModel(modelAndIDs.ModelObj)
			db3 := db.Where(tableName+".created_at BETWEEN ? AND ?", begin, end)
			if err := db3.Where(fmt.Sprintf("%s_id IN (?)", parentTableName), modelAndIDs.IDs.ToSlice()).
				Find(sliceI).Error; err != nil {
				return err
			}

			for i := 0; i < sliceElem.Len(); i++ {
				fieldName := strcase.UpperCamelCase(parentTableName) + "ID"
				// log.Println("fieldName:", fieldName)
				var parentID *datatype.UUID
				// FieldByName sometimes upper camel case is hard to find if having a streak of upper case
				l := sliceElem.Index(i).Elem()
				for j := 0; j < l.NumField(); j++ {
					if strings.ToLower(l.Type().Field(j).Name) == strings.ToLower(fieldName) {
						parentID = l.Field(j).Interface().(*datatype.UUID)
					}
				}

				// parentID := sliceElem.Index(i).Elem().FieldByName(fieldName).Interface().(*datatype.UUID)
				// Match the parent ID
				for j, modelObj := range modelObjs {
					if modelObj.GetID().String() == parentID.String() { // same id
						if fieldAsKey.FieldType == mdl.FieldTypeStruct {
							reflect.ValueOf(modelObjs[j]).Field(fieldAsKey.FieldNum).Set(sliceElem.Index(i).Elem())
						} else if fieldAsKey.FieldType == mdl.FieldTypeSlice {
							array := reflect.ValueOf(modelObjs[j]).Elem().Field(fieldAsKey.FieldNum)
							array.Set(reflect.Append(array, sliceElem.Index(i).Elem()))
						} else if fieldAsKey.FieldType == mdl.FieldTypeStructPtr {
							reflect.ValueOf(modelObjs[j]).Field(fieldAsKey.FieldNum).Set(sliceElem.Index(i))
						}
					}
				}
			}
		}
	}

	for _, modelObj := range modelObjs {
		v := reflect.Indirect(reflect.ValueOf(modelObj))
		for i := 0; i < v.NumField(); i++ {
			tagVal := v.Type().Field(i).Tag.Get("betterrest")
			// var mapping *map[string]map[FieldAsKey]ModelAndIDs
			if gotag.TagValueHasPrefix(tagVal, "peg-ignore") || gotag.TagValueHasPrefix(tagVal, "pegassoc") {
				continue
			}

			if !gotag.TagValueHasPrefix(tagVal, "peg") {
				continue
			}
			// pegged at this point
			switch v.Field(i).Kind() {
			case reflect.Struct:
				m2 := v.Field(i).Interface().(mdl.IModel)
				if err := RecursivelyQueryAllPeggedModels(db, []mdl.IModel{m2}, begin, end); err != nil {
					return err
				}
			case reflect.Ptr:
				m2 := v.Elem().Field(i).Interface().(mdl.IModel)
				if err := RecursivelyQueryAllPeggedModels(db, []mdl.IModel{m2}, begin, end); err != nil {
					return err
				}
			case reflect.Slice:
				arr := make([]mdl.IModel, 0)
				for j := 0; j < v.Field(i).Len(); j++ {
					m2 := v.Field(i).Index(j).Addr().Interface().(mdl.IModel)
					arr = append(arr, m2)
				}
				if err := RecursivelyQueryAllPeggedModels(db, arr, begin, end); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
