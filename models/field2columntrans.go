package models

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/stoewer/go-strcase"
)

func FieldNameToColumn(modelObj IModel, fieldName string) (string, error) {
	toks := strings.SplitN(fieldName, ".", 2)
	first := toks[0]

	structField, ok := reflect.TypeOf(modelObj).Elem().FieldByName(first)
	if !ok {
		// debug.PrintStack()
		return "", fmt.Errorf("field \"%s\" does not exist", first)
	}

	columnName := strcase.SnakeCase(first)

	// custom column name, if any
	if gormtag := structField.Tag.Get("gorm"); gormtag != "" {
		// Now we found a match, and does it have a custom column name?
		toks := strings.Split(strings.TrimSpace(gormtag), ";")
		for _, tok := range toks {
			if strings.HasPrefix(tok, "column:") {
				columnName = strings.Split(tok, ":")[1] // specified column name
			}
		}
	}

	// Now, traverse the rest
	if len(toks) > 1 {
		typ := structField.Type

		// Follows pointer, slice, slice to pointer, etc.
		for {
			if typ.Kind() == reflect.Ptr || typ.Kind() == reflect.Slice {
				typ = typ.Elem()
			} else {
				break
			}
		}

		innerModel := reflect.New(typ).Interface().(IModel)
		innerColumnName, err := FieldNameToColumn(innerModel, toks[1])
		if err != nil {
			return "", err
		}
		return columnName + "." + innerColumnName, nil
	}

	return columnName, nil
}

func FieldNameToJSONName(modelObj IModel, fieldName string) (string, error) {
	toks := strings.SplitN(fieldName, ".", 2)
	first := toks[0]

	structField, ok := reflect.TypeOf(modelObj).Elem().FieldByName(first)
	if !ok {
		// debug.PrintStack()
		return "", fmt.Errorf("field \"%s\" does not exist", first)
	}

	jsonName := strcase.LowerCamelCase(first)

	// custom column name, if any
	if jsontag := structField.Tag.Get("json"); jsontag != "" {
		// Now we found a match, and does it have a custom column name?
		toks := strings.Split(strings.TrimSpace(jsontag), ";")
		return toks[0], nil
	}

	// Now, traverse the rest
	if len(toks) > 1 {
		typ := structField.Type

		// Follows pointer, slice, slice to pointer, etc.
		for {
			if typ.Kind() == reflect.Ptr || typ.Kind() == reflect.Slice {
				typ = typ.Elem()
			} else {
				break
			}
		}

		innerModel := reflect.New(typ).Interface().(IModel)
		innerJSONName, err := FieldNameToJSONName(innerModel, toks[1])
		if err != nil {
			return "", err
		}
		return jsonName + "." + innerJSONName, nil
	}

	return jsonName, nil
}
