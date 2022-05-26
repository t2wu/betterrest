package models

import (
	"reflect"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/gotag"
)

type FieldNumAndType struct {
	FieldNum  int
	TypeName  string
	FieldName string
	ObjType   reflect.Type
	IsSlice   bool // Could be slice of pointer or not
	IsPtr     bool
	IsStruct  bool
}

func GetPeggedFieldNumAndType(modelObj IModel) []FieldNumAndType {
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	ret := make([]FieldNumAndType, 0)

	for i := 0; i < v.NumField(); i++ {
		tagVal := v.Type().Field(i).Tag.Get("betterrest")
		if tagVal != "peg" {
			continue
		}

		switch v.Field(i).Kind() {
		case reflect.Slice: // handles slice of struct and slice of pointers..
			nextType := v.Type().Field(i).Type.Elem()
			isSlice := true
			// isSliceOfPtr := false
			// if nextType.Kind() == reflect.Ptr {
			// 	isSlice = false
			// 	isSliceOfPtr = true
			// nextType = nextType.Elem()
			// }
			// reflect.SliceOf(nextType)

			// s := reflect.New(reflect.SliceOf(nextType))
			// s2 := reflect.New(s.Type().Elem())
			// s2.Elem().Set(s.Elem())
			// obj := s2.Elem().Interface()
			ret = append(ret, FieldNumAndType{
				FieldName: v.Type().Field(i).Name,
				TypeName:  nextType.Name(),
				IsSlice:   isSlice,
				FieldNum:  i,
				ObjType:   nextType, // maybe we don't really need to differentiate
				// reflect.MakeSlice(v.Field(i).Type(), 0, 0) // doesn't seem to be any different
			})
		case reflect.Struct:
			ret = append(ret, FieldNumAndType{
				FieldName: v.Type().Field(i).Name,
				TypeName:  v.Type().Field(i).Type.Name(),
				IsStruct:  true,
				FieldNum:  i,
				ObjType:   v.Type().Field(i).Type, // Elem() otherwise after new is a pointer (interface should wrap the pointer type)
			})
		case reflect.Ptr:
			nextType := v.Type().Field(i).Type.Elem()
			ret = append(ret, FieldNumAndType{
				FieldName: v.Type().Field(i).Name,
				TypeName:  v.Type().Field(i).Type.Name(),
				IsPtr:     true,
				FieldNum:  i,
				ObjType:   nextType,
			})
		}
	}
	return ret

	// This is getting some weird output
	// typ := reflect.TypeOf(v)
	// for i := 0; i < typ.NumField(); i++ {
	// 	log.Println("fieldName:", typ.Field(i).Name)
	// 	log.Println("type:", typ.Field(i).Type)
	// 	log.Println("kind:", typ.Field(i).Type.Kind())
	// 	// f := typ.Field(i)
	// 	// fieldName := v.Type().Field(i).Name
	// }
}

func SetSliceAtFieldNum(modelObj IModel, fieldNum int, ele interface{}) {
	sl := reflect.ValueOf(modelObj).Elem().Field(fieldNum)
	sl.Set(reflect.ValueOf(ele).Elem())
}

func AppendToSliceAtFieldNum(modelObj IModel, fieldNum int, ele interface{}) {
	sl := reflect.ValueOf(modelObj).Elem().Field(fieldNum)
	sl.Set(reflect.Append(sl, reflect.ValueOf(ele).Elem()))
}

func SetStructAtFieldNum(modelObj IModel, fieldNum int, ele interface{}) {
	sl := reflect.ValueOf(modelObj).Elem().Field(fieldNum)
	sl.Set(reflect.ValueOf(ele).Elem())
}

func SetStructPtrAtFieldNum(modelObj IModel, fieldNum int, ele interface{}) {
	sl := reflect.ValueOf(modelObj).Elem().Field(fieldNum)
	sl.Set(reflect.ValueOf(ele))
}

// --------------------------------------------------------------------------------

type FieldType int

const (
	FieldTypeSlice     FieldType = iota // within slice
	FieldTypeStruct                     // within struct
	FieldTypeStructPtr                  // within struct pointer
)

type Relation int

const (
	RelationPeg Relation = iota
	RelationPegAssoc
	RelationPegAssocManyToMany // not handled yet
)

type ModelAndIDs struct {
	ModelObj IModel // this is just one or a new model we can look up

	// IDs is the parent ID who we want to query
	IDs       []interface{} // to send to Gorm need to be interface not *datatypes.UUID
	ModelObjs []IModel      // this is a storage when we've queried it
}

type FieldAsKey struct {
	FieldNum  int // which field this belongs
	FieldType FieldType
	Rel       Relation // peg or pegassoc
	// ID        *datatypes.UUID // the ID where this fieldNum is on
}

func NewPeggedIDSearch() *PeggedIDSearch {
	return &PeggedIDSearch{
		// ModelObjs: make(map[string]reflect.Value),
		ToProcess: make(map[string]map[FieldAsKey]*ModelAndIDs),
	}
}

type PeggedIDSearch struct {
	// There could be the same struct which exists in two places in the model
	// so we use FieldAsKey to store it, also tell us whether it was pointer or pointer to struct or slice
	// key is table name
	// tablename is the PARENT table name
	// tablename --> modelObj
	// ModelObjs map[string]reflect.Value
	// tablename -> FieldAsKey -> ids
	ToProcess map[string]map[FieldAsKey]*ModelAndIDs // those whose relationship with upper level is Peg
}

// FindAllBetterRestPeggOrPegAssocIDs finds all Pegged or pegassoc ids, this is for OrgPartition
// which need pegged field only, pegassoc not yet tested.
// Many to many not handled at this point
// This is modified from markForDelete in gormfix and gormfixes
// modelObj is not models.IModel just because I don't need the dependency, may make it more
// generic later
func FindAllBetterRestPeggOrPegAssocIDs(modelObj interface{}, result *PeggedIDSearch) error {
	val := reflect.ValueOf(modelObj)
	m := reflect.Indirect(val)
	id := val.Interface().(IModel).GetID()
	tableName := GetTableNameFromIModel(modelObj.(IModel))
	return findAllBetterRestPeggOrPegAssocIDsCore(m, result, tableName, id)
}

// id is the id of the model v under processing
func findAllBetterRestPeggOrPegAssocIDsCore(v reflect.Value, result *PeggedIDSearch, tableName string, id *datatypes.UUID) error {
	// v is the parent struct
	// log.Println("...............FindAllPeggedIDs called:", v)
	for i := 0; i < v.NumField(); i++ {
		tagVal := v.Type().Field(i).Tag.Get("betterrest")
		// var mapping *map[string]map[FieldAsKey]ModelAndIDs
		if gotag.TagValueHasPrefix(tagVal, "peg-ignore") {
			continue
		}

		isPeg := gotag.TagValueHasPrefix(tagVal, "peg")
		var rel Relation
		if isPeg {
			rel = RelationPeg
		}
		isPegAssoc := gotag.TagValueHasPrefix(tagVal, "pegassoc")
		if isPegAssoc {
			rel = RelationPegAssoc
		}

		if !isPeg && !isPegAssoc {
			continue // not what we want to handle
		}

		switch v.Field(i).Kind() {
		case reflect.Struct:
			m := v.Field(i).Addr().Interface().(IModel)
			// fieldTableName := GetTableNameFromIModel(m) // fieldTableName
			key := FieldAsKey{
				FieldNum:  i,
				FieldType: FieldTypeStruct,
				Rel:       rel,
				// ID:        id,
			}

			makeSpaceInPeggedIDSearch(result, key, tableName, m)
			result.ToProcess[tableName][key].IDs = append(result.ToProcess[tableName][key].IDs, id) // store parent ID!
		case reflect.Slice:
			typ := v.Type().Field(i).Type.Elem()
			m, _ := reflect.New(typ).Interface().(IModel)
			// fieldTableName := GetTableNameFromIModel(m)

			key := FieldAsKey{
				FieldNum:  i,
				FieldType: FieldTypeSlice,
				Rel:       rel,
			}

			makeSpaceInPeggedIDSearch(result, key, tableName, m)
			result.ToProcess[tableName][key].IDs = append(result.ToProcess[tableName][key].IDs, id) // store parent id!
		case reflect.Ptr:
			// is IsZero the same? if !v.Field(i).IsZero() {
			// Need to dereference and get the struct id before traversing in
			if !isNil(v.Field(i)) && !isNil(v.Field(i).Elem()) &&
				v.Field(i).IsValid() && v.Field(i).Elem().IsValid() {
				imodel := v.Field(i).Interface().(IModel)
				// fieldTableName := GetTableNameFromIModel(imodel)

				key := FieldAsKey{
					FieldNum:  i,
					FieldType: FieldTypeStructPtr,
					Rel:       rel,
				}
				makeSpaceInPeggedIDSearch(result, key, tableName, imodel)
				result.ToProcess[tableName][key].IDs = append(result.ToProcess[tableName][key].IDs, id) // stores parent ID!
			}
		}
	}
	return nil
}

func makeSpaceInPeggedIDSearch(result *PeggedIDSearch, key FieldAsKey, fieldTableName string, modelObj IModel) {
	if _, ok := result.ToProcess[fieldTableName]; !ok {
		result.ToProcess[fieldTableName] = make(map[FieldAsKey]*ModelAndIDs)
	}

	if _, ok := result.ToProcess[fieldTableName][key]; !ok {
		arr := make([]interface{}, 0)
		v := ModelAndIDs{ModelObj: modelObj, IDs: arr}
		result.ToProcess[fieldTableName][key] = &v // does it make sense for modelObj to be stored here?
	}
}

func isNil(a interface{}) bool {
	defer func() { recover() }()
	return a == nil || reflect.ValueOf(a).IsNil()
}
