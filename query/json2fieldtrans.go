package query

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/t2wu/betterrest/models"
)

// JSONKeyToColumnName transforms json name to column name
// if not found, return err
// By doing this we don't need model check?
func JSONKeysToFieldName(modelObj models.IModel, key string) (string, error) {
	toks := strings.SplitN(key, ".", 2)
	first := toks[0]

	fieldName := ""

	// Need to loop for each tag
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	var typ reflect.Type
loop:
	for i := 0; i < v.NumField(); i++ {
		jsontag := v.Type().Field(i).Tag.Get("json")
		if jsontag != "" {
			toks := strings.Split(strings.TrimSpace(jsontag), ",")
			if toks[0] == first {
				// no designated column name, so we go with Upper-camel case
				fieldName = v.Type().Field(i).Name
				typ = v.Type().Field(i).Type
				break loop
			}
		}
	}

	if fieldName == "" {
		// Not found
		return "", fmt.Errorf("field \"%s\" does not exist", first)
	}

	// Now, traverse the rest
	if len(toks) > 1 {
		// Follows pointer, slice, slice to pointer, etc.
		for {
			if typ.Kind() == reflect.Ptr || typ.Kind() == reflect.Slice {
				typ = typ.Elem()
			} else {
				break
			}
		}

		innerModel := reflect.New(typ).Interface().(models.IModel)
		innerFieldName, err := JSONKeysToFieldName(innerModel, toks[1])
		if err != nil {
			return "", err
		}
		return fieldName + "." + innerFieldName, nil
	}

	return fieldName, nil
}
