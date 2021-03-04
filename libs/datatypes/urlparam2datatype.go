package datatypes

import (
	"fmt"
	"log"
	"reflect"
	"strconv"

	"github.com/stoewer/go-strcase"
)

// TransformFieldValue transforms type in URL parameter to the proper data types
func TransformFieldValue(typeInString string, fieldValue string) (interface{}, error) {
	var retval interface{}
	switch typeInString {
	case "*datatypes.UUID", "datatypes.UUID":
		var data *UUID
		if fieldValue != "null" {
			var err error
			data, err = NewUUIDFromString(fieldValue)
			if err != nil {
				return nil, err
			}
		}
		retval = data
	case "*bool", "bool":
		data, err := strconv.ParseBool(fieldValue)
		if err != nil {
			return nil, err
		}
		retval = data
	// case "*int":
	// 	fallthrough
	// case "int":
	// 	log.Println("iiiiiiiiiiiiiiiiiiiiiiiiiiiiiiiint case")
	// 	for i, fieldValue := range fieldValues {
	// 		data, err := strconv.ParseBool(fieldValue)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		fieldValuesRet[i] = data
	// 	}
	// 	break
	default:
		retval = fieldValue
	}
	return retval, nil
}

// TransformFieldValues transforms type in URL parameters to the proper data types
func TransformFieldValues(typeInString string, fieldValues []string) ([]interface{}, error) {
	fieldValuesRet := make([]interface{}, len(fieldValues), len(fieldValues))
	for i, fieldValue := range fieldValues {
		data, err := TransformFieldValue(typeInString, fieldValue)
		if err != nil {
			return nil, err
		}
		fieldValuesRet[i] = data
	}
	return fieldValuesRet, nil
}

// GetModelFieldTypeIfValid make sure the fieldName is in the modelObj, and find the correct reflect.Type
// func GetModelFieldTypeIfValid(modelObj models.IModel, fieldName string) (reflect.Type, error) {
// If this is an array, get the actual type instead of the array type
func GetModelFieldTypeIfValid(modelObj interface{}, fieldName string) (reflect.Type, error) {
	var fieldType reflect.Type
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	structField, ok := v.Type().FieldByName(fieldName)
	if ok {
		fieldType = structField.Type
	} else if fieldName == "id" {
		fieldType = reflect.TypeOf(&UUID{})
	} else if fieldName == "Id" {
		fieldType = reflect.TypeOf(&UUID{})
	} else {
		// It may not exists, or the field name is capitalized. search for JSON tag
		// v.Type().FieldByIndex(0).Tag
		found := false
		snake := strcase.SnakeCase(fieldName)
		for i := 0; i < v.NumField(); i++ {
			v2 := v.Type().Field(i)
			tag := v2.Tag.Get("json")
			if tag == snake {
				found = true
				fieldType = v2.Type
			}
		}
		if !found {
			return nil, fmt.Errorf("field name %s does not exist", fieldName)
		}
	}
	return fieldType, nil
}

// func GetFieldTypeIfValid(modelObj interface{}, fieldName string) (reflect.Type, error) {
// 	var fieldType reflect.Type
// 	v := reflect.Indirect(reflect.ValueOf(modelObj))
// 	log.Printf("v? %+v\n", v)
// 	log.Printf("fieldName? %+v\n", fieldName)
// 	structField, ok := v.Type().FieldByName(fieldName)
// 	if ok {
// 		fieldType = structField.Type
// 	} else if fieldName == "id" {
// 		fieldType = reflect.TypeOf(&UUID{})
// 	} else if fieldName == "Id" {
// 		fieldType = reflect.TypeOf(&UUID{})
// 	} else {
// 		// It may not exists, or the field name is capitalized. search for JSON tag
// 		// v.Type().FieldByIndex(0).Tag
// 		found := false
// 		snake := strcase.SnakeCase(fieldName)
// 		for i := 0; i < v.NumField(); i++ {
// 			v2 := v.Type().Field(i)
// 			tag := v2.Tag.Get("json")
// 			log.Println("tag:", tag)
// 			log.Println("snake:", snake)
// 			if tag == snake {
// 				found = true
// 				fieldType = v2.Type
// 			}
// 		}
// 		if !found {
// 			return nil, fmt.Errorf("field name %s does not exist", fieldName)
// 		}
// 	}
// 	return fieldType, nil
// }

// GetModelFieldTypeElmIfValid is like GetModelFieldTypeIfValid, but get the element if it is array
func GetModelFieldTypeElmIfValid(modelObj interface{}, fieldName string) (reflect.Type, error) {
	fieldType, err := GetModelFieldTypeIfValid(modelObj, fieldName)
	if err != nil {
		log.Println("GetModelFieldTypeIfValid err:", err)
		return nil, err
	}

	fieldType, err = obtainModelTypeIfFromArrayFieldType(fieldType)
	if err != nil {
		log.Println("obtainModelTypeFromArrayFieldType err:", err)
		return nil, err
	}

	return fieldType, nil
}

func obtainModelTypeIfFromArrayFieldType(fieldType reflect.Type) (reflect.Type, error) {
	var innerTyp reflect.Type
	switch fieldType.Kind() {
	case reflect.Slice:
		innerTyp = fieldType.Elem()
	default:
		innerTyp = fieldType
		// fmt.Printf("Unknown type")
		// return nil, fmt.Errorf("unknown error occurred while processing nested field query")
	}
	return innerTyp, nil
}
