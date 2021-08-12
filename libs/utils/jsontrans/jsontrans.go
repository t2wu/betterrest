package jsontrans

import (
	"encoding/json"
	"log"
	"reflect"
	"strings"
	"unicode"
)

// JSONFields is the fields we specify to pick out only the JSON fields we want
type JSONFields map[string]interface{}

// Field specify what to do with each field
type Field int

const (
	FieldNone Field = iota
	FieldOmitEmpty
	FieldIncludeEmpty // for black list which default to block
	FieldBlock        // block when it's blacklist
	FieldPass         // right now set it as nil is the same as pass
)

// JSONFieldEmptyList is defined for field should be an empty list if no value exists
// TODO: need to check if this works
type JSONFieldEmptyList bool

// JSONFieldEmptyString is defined for field should be an empty string if no value exists
// TODO: need to check if this works
type JSONFieldEmptyString bool

// JSONFieldBlock is defined for field that should be blocked when PermissionBlackList is used
// TODO: doesn't work yet
type JSONFieldBlock bool

// Permission of the JSON parsing
type Permission int

const (
	// PermissionWhiteList means blocking all fields by default blocked, only specify those that are there
	PermissionWhiteList Permission = iota
	// PermissionBlackList means allow fields by default allowed, but make transformation to certain field
	// including to block it (not supported yet)
	PermissionBlackList
)

// IFieldTransformModelToJSON does any transformation from model to JSON when fetching
// out of database
type IFieldTransformModelToJSON interface {
	TransformModelToJSON(field interface{}) (interface{}, error)
}

// IFieldTransformJSONToModel does any transformation from JSON when
// fetching from user JSON input
type IFieldTransformJSONToModel interface {
	TransformJSONToModel(field interface{}) (interface{}, error)
}

// Transform performs the following:
// 1. Cherry pick only fields we want by the fields specifier
// when given a json string,
// I cannot cherry pick struct, because I can't make it generic enough
// to fit all models
// Then that means I have to loop the array to json marshal it
// 2. Transform the value if necessary
// (used to be called JSONCherryPickFields)
func Transform(data map[string]interface{}, f *JSONFields, permType Permission) (map[string]interface{}, error) {
	dataPicked := make(map[string]interface{})
	if f != nil {
		// Traverse through the fields and pick only those we need
		if err := cherryPickCore(data, f, dataPicked, permType); err != nil {
			return nil, err
		}
	} else {
		dataPicked = data
	}

	return dataPicked, nil
}

// TODO permType = PermissionBlackList not implemented yet
func cherryPickCore(data map[string]interface{}, f *JSONFields, dataPicked map[string]interface{}, permType Permission) error {
	// Traverse through the fields and pick only those we need
	fi := *f
	for k, v := range *f {
		// log.Println("k, v, data[k]:", k, v, data[k])
		if datv, ok := data[k].([]interface{}); ok { // is slice after this
			// if data[k] != nil && reflect.TypeOf(data[k]).Kind() == reflect.Slice { // slice

			dataPicked[k] = make([]map[string]interface{}, len(datv))
			if datPickedv, ok := dataPicked[k].([]map[string]interface{}); ok { // should always be ok
				// datPickedv is a slice
				for i := range datv { // loop the slice
					newdat := datv[i].(map[string]interface{})
					newF := fi[k].(JSONFields)
					datPickedv[i] = make(map[string]interface{})
					newDatPicked := datPickedv[i]
					if err := cherryPickCore(newdat, &newF, newDatPicked, permType); err != nil {
						return err
					}
				}
			} else {
				// Probably never here
				log.Println("cherryPickCore probably never here ")
				dataPicked[k] = make([]interface{}, 0)
			}
		} else if data[k] == nil {
			// since value is interface{}, if we never insert it, it comes out to be nil
			// technically since run through TransFromByHidingDateFieldsFromIModel() now,
			// we now should rely on json tag, but of course user might want to configure this
			// if they don't want a customized output based on models.Who{}
			if v2, ok := v.(string); ok {
				// if nil, change to this string
				dataPicked[k] = v2
			} else if v2, ok := v.(int); ok {
				// if nil, change to this int
				dataPicked[k] = v2
			} else if v2, ok := v.(bool); ok {
				// if nil, change to this boolean
				dataPicked[k] = v2
			} else if v == nil {
				// if nil, change to this int
				dataPicked[k] = nil
			} else if _, ok := v.(JSONFieldEmptyList); ok {
				dataPicked[k] = make([]interface{}, 0)
			} else if v == FieldOmitEmpty {
				// ignore
			} else {
				// if nil, default to empty list
				dataPicked[k] = make([]interface{}, 0)
			}
			// log.Println("reflect.TypeOf(data[k]):", reflect.TypeOf(data[k]))
		} else if datastring, ok := data[k].(string); ok && datastring == "" {
			// I can use pointer in a model, but then govalidator is having issue with it
			// I don't know if I switch to Gin and govalidator things would be better?
			// So my strings in the model right now is not pointer
			// empty string
			if v != FieldOmitEmpty {
				dataPicked[k] = ""
			}
		} else if newF, ok := v.(JSONFields); ok {
			// not slice. Usually this matches the case where there is an array of structs
			// inside a table and we're looping through that instance.
			// But it could also be like deviceStatus it's actually a nested struct of one
			embeddedStruct := make(map[string]interface{})
			if err := cherryPickCore(data[k].(map[string]interface{}), &newF, embeddedStruct, permType); err != nil {
				return err
			}
			dataPicked[k] = embeddedStruct
		} else {
			if transformF, ok := v.(func(interface{}) (interface{}, error)); ok {
				transV, err := transformF(data[k])
				if err != nil {
					return err
				}
				dataPicked[k] = transV
			} else if transformStruct, ok := v.(IFieldTransformModelToJSON); ok {
				transV, err := transformStruct.TransformModelToJSON(data[k])
				if err != nil {
					return err
				}
				dataPicked[k] = transV
			} else { // v can be nil, or other default value
				dataPicked[k] = data[k]
			}
		}
	}

	return nil
}

// ContainsIFieldTransformModelToJSON check if f contains any struct comforms to IFieldTransformModelToJSON
func ContainsIFieldTransformModelToJSON(f *JSONFields) bool {
	for _, v := range *f {
		if newF, ok := v.(JSONFields); ok { // an object
			result := ContainsIFieldTransformModelToJSON(&newF)
			if result == true {
				return true
			}
		} else if _, ok := v.(IFieldTransformModelToJSON); ok {
			return true
		}
	}
	return false
}

// ----------------------------------

// ModelNameToTypeStringMapping maps name of the IModel to TypeString
// This is for "href" field of the JSON output
// Both pointer and nonpointer type names are stored
var ModelNameToTypeStringMapping = make(map[string]string)

// TransFromByHidingDateFieldsFromIModel does some default transformation the package provides
// Without user defining a model comforming to IHasPermissions interface
// It doesn't respect all json tag, but naming and omitempty does
// The reason we choose to traverse IModel is because I want to fill the "href"
// field
// This picks all the stuff from the IModel to a map
// We don't run JSON marshalling library this stage. It is expect it to be run
// on the map
func TransFromByHidingDateFieldsFromIModel(v reflect.Value, includeCUDDates bool) map[string]interface{} {
	dataPicked := make(map[string]interface{})

	if typeString, ok := ModelNameToTypeStringMapping[reflect.TypeOf(v.Interface()).String()]; ok {
		dataPicked["href"] = "/" + strings.ToLower(typeString)
	}

	for i := 0; i < v.NumField(); i++ {
		nameOfField := v.Type().Field(i).Name
		// typeOfField := v.Type().Field(i).Type

		if unicode.IsLower(rune(nameOfField[0])) {
			// Not exported, don't bother. (If I process it I need unsafe pointer too otherwise it'l panic)
			continue
		}

		if !includeCUDDates &&
			(nameOfField == "CreatedAt" || nameOfField == "UpdatedAt" || nameOfField == "DeletedAt") {
			continue // skip these fields
		}

		jsonTag := v.Type().Field(i).Tag.Get("json")
		tags := strings.Split(jsonTag, ",")
		isOmitEmpty := sliceContainsString(tags, "omitempty")
		jsonKey := ""
		if len(tags) == 2 && tags[0] != "" {
			jsonKey = tags[0] // tag can be "-," which means the name is "-"
		} else if len(tags) == 1 {
			jsonKey = tags[0]
		} else {
			jsonKey = nameOfField
		}

		if len(tags) == 1 && tags[0] == "-" { // skip
			continue
		}

		switch v.Field(i).Kind() {
		case reflect.Struct:
			// Traverse into the struct

			// If comfirm to the Marshaler interface, let the JSON handles it
			// No need to walk further in, expect JSON marshal to take care of it later
			if _, ok := v.Field(i).Addr().Interface().(json.Marshaler); ok {
				if unicode.IsUpper(rune(nameOfField[0])) { // Exported (public)
					dataPicked[jsonTag] = v.Field(i).Interface()
				}
			} else {
				// Is this a model with its own endpoint?
				dataPickedReturned := TransFromByHidingDateFieldsFromIModel(v.Field(i), includeCUDDates)
				if v.Type().Field(i).Anonymous {
					// Embedded struct
					for key, val := range dataPickedReturned {
						dataPicked[key] = val
					}
				} else {
					dataPicked[jsonKey] = dataPickedReturned
				}
			}

		case reflect.Slice:
			arr := make([]interface{}, v.Field(i).Len())
			for j := 0; j < v.Field(i).Len(); j++ {
				o := TransFromByHidingDateFieldsFromIModel(v.Field(i).Index(j), includeCUDDates)
				if !isOmitEmpty || !isEmptyValue(reflect.ValueOf(o)) {
					arr[j] = o
				}
			}
			if !isOmitEmpty || !isEmptyValue(reflect.ValueOf(arr)) {
				dataPicked[jsonKey] = arr
			}
		case reflect.Ptr:
			inside := v.Field(i).Elem()
			if inside.Kind() == reflect.Struct {
				// If comfirm to the Marshaler interface, let the JSON handles it
				// No need to walk further in, expect JSON marshal to take care of it later
				// (so the struct is not part of the IModel, more of an elementary value)
				if _, ok := v.Field(i).Interface().(json.Marshaler); ok {
					if !isOmitEmpty || !isEmptyValue(v.Field(i)) {
						dataPicked[jsonTag] = v.Field(i).Interface()
					}
				} else {
					// Traverse into the struct
					dataPickedReturned := TransFromByHidingDateFieldsFromIModel(inside, includeCUDDates)
					if !isOmitEmpty || !isEmptyValue(reflect.ValueOf(dataPickedReturned)) {
						if v.Type().Field(i).Anonymous {
							// Embedded struct
							for key, val := range dataPickedReturned {
								dataPicked[key] = val
							}
						} else {
							if !isOmitEmpty || !isEmptyValue(v.Field(i)) {
								dataPicked[jsonKey] = dataPickedReturned
							}
						}
					}
				}
			} else if inside.Kind() == reflect.Slice {
				arr := make([]map[string]interface{}, v.Field(i).Len())
				for j := 0; j < v.Field(i).Len(); j++ {
					arr[j] = TransFromByHidingDateFieldsFromIModel(v.Field(i).Index(j), includeCUDDates)
				}
				if !isOmitEmpty || !isEmptyValue(reflect.ValueOf(arr)) {
					dataPicked[jsonKey] = arr
				}
			} else {
				if !isOmitEmpty || !isEmptyValue(v.Field(i)) {
					dataPicked[jsonKey] = v.Field(i).Interface()
				}
			}
		default: // everything else stay as it is
			if !isOmitEmpty || !isEmptyValue(v.Field(i)) {
				dataPicked[jsonKey] = v.Field(i).Interface()
			}
		}
	}

	return dataPicked
}

// Taken from Go's own JSON library
func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}

	return false
}

func sliceContainsString(arr []string, s string) bool {
	for _, ele := range arr {
		if ele == s {
			return true
		}
	}
	return false
}
