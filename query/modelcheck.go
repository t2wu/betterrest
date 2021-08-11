package query

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

func IsFieldInModel(modelObj models.IModel, field string) bool {
	toks := strings.SplitN(field, ".", 2)
	first := toks[0]

	structField, ok := reflect.TypeOf(modelObj).Elem().FieldByName(first)

	getInnerType := func(structField reflect.StructField) reflect.Type {
		if structField.Type.Kind() == reflect.Ptr || structField.Type.Kind() == reflect.Slice {
			return structField.Type.Elem() // pointer or slice both use Elem(0)
		} else {
			return structField.Type
		}
	}

	ok2 := true
	if ok && len(toks) > 1 && toks[1] != "" { // has nested field
		innerType := getInnerType(structField)
		innerModel := reflect.New(innerType).Interface().(models.IModel)

		ok2 = IsFieldInModel(innerModel, toks[1])
	}

	return ok && ok2
}

// Never returns the pointer value
// Since what we want is reflec.New() and it would be a pointer
func GetModelFieldTypeInModelIfValid(modelObj models.IModel, field string) (reflect.Type, error) {
	toks := strings.SplitN(field, ".", 2)
	first := toks[0]

	structField, ok := reflect.TypeOf(modelObj).Elem().FieldByName(first)
	if !ok {
		return nil, fmt.Errorf("invalid field")
	}
	typ := structField.Type

	if structField.Type.Kind() == reflect.Slice || structField.Type.Kind() == reflect.Slice {
		typ = structField.Type.Elem()
	}

	// If it is still pointer follows it
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// getInnerType := func(structField reflect.StructField) reflect.Type {
	// 	if structField.Type.Kind() == reflect.Ptr || structField.Type.Kind() == reflect.Slice {
	// 		return structField.Type.Elem()
	// 	} else {
	// 		return structField.Type
	// 	}
	// }

	var err error
	if ok && len(toks) > 1 && toks[1] != "" { // has nested field
		innerModel := reflect.New(typ).Interface().(models.IModel)

		typ, err = GetModelFieldTypeInModelIfValid(innerModel, toks[1])
	}

	return typ, err
}

// FieldNotInModelError is for GetModelFieldTypeIfValid.
// if field doesn't exist in the model, return this error
// We want to go ahead and skip it since this field may be other
// options that user can read in hookpoints
type FieldNotInModelError struct {
	Msg string
}

func (r *FieldNotInModelError) Error() string {
	return r.Msg
}

func GetModelFieldTypeIfValid(modelObj models.IModel, fieldName string) (reflect.Type, error) {
	var fieldType reflect.Type
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	structField, ok := v.Type().FieldByName(fieldName)
	if ok {
		fieldType = structField.Type
	} else if fieldName == "id" {
		fieldType = reflect.TypeOf(&datatypes.UUID{})
	} else if fieldName == "Id" {
		fieldType = reflect.TypeOf(&datatypes.UUID{})
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
			return nil, &FieldNotInModelError{Msg: fmt.Sprintf("field name %s does not exist", fieldName)}
		}
	}
	return fieldType, nil
}
