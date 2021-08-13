package query

import (
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/models"
)

func FieldNameToColumn(modelObj models.IModel, fieldName string) (string, error) {
	toks := strings.SplitN(fieldName, ".", 2)
	first := toks[0]

	structField, ok := reflect.TypeOf(modelObj).Elem().FieldByName(first)
	if !ok {
		return "", errors.Errorf("field not found: %s", first)
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

		innerModel := reflect.New(typ).Interface().(models.IModel)
		innerColumnName, err := FieldNameToColumn(innerModel, toks[1])
		if err != nil {
			return "", err
		}
		return columnName + "." + innerColumnName, nil
	}

	return columnName, nil
}
