package jsontransform

import (
	"encoding/json"
	"log"
)

// JSONFields is the fields we specify to pick out only the JSON fields we want
type JSONFields map[string]interface{}

// JSONFieldIgnoreIfEmpty is defined for field that should not be in the JSON if no value exists
type JSONFieldIgnoreIfEmpty bool

// JSONFieldEmptyList is defined for field should be an empty list if no value exists
type JSONFieldEmptyList bool

// JSONFieldEmptyString is defined for field should be an empty string if no value exists
type JSONFieldEmptyString bool

// Transform performs the following:
// 1. Cehrry pick only fields we want by the fields specifier
// when given a json string,
// I cannot cherry pick struct, because I can't make it generic enough
// to fit all models
// Then that means I have to loop the array to json marshal it
// 2. Transform the value if necessary
// (used to be called JSONCherryPickFields)
func Transform(j []byte, f *JSONFields) ([]byte, error) {
	if f != nil {
		var dat map[string]interface{}
		var jret []byte
		var err error

		if err = json.Unmarshal(j, &dat); err != nil {
			return nil, err
		}

		datPicked := make(map[string]interface{})

		// Traverse through the fields and pick only those we need
		cherryPickCore(dat, f, datPicked)

		if jret, err = json.Marshal(datPicked); err != nil {
			return nil, err
		}

		return jret, nil
	}
	return j, nil
}

func cherryPickCore(dat map[string]interface{}, f *JSONFields, datPicked map[string]interface{}) {
	// Traverse through the fields and pick only those we need
	fi := *f
	for k, v := range *f {
		// log.Println("k, v, dat[k]:", k, v, dat[k])
		if datv, ok := dat[k].([]interface{}); ok { // is slice after this
			// if dat[k] != nil && reflect.TypeOf(dat[k]).Kind() == reflect.Slice { // slice

			datPicked[k] = make([]map[string]interface{}, len(datv))
			if datPickedv, ok := datPicked[k].([]map[string]interface{}); ok { // should always be ok
				// datPickedv is a slice
				for i := range datv { // loop the slice
					newdat := datv[i].(map[string]interface{})
					newF := fi[k].(JSONFields)
					datPickedv[i] = make(map[string]interface{})
					newDatPicked := datPickedv[i]
					cherryPickCore(newdat, &newF, newDatPicked)
				}
			} else {
				// Probably never here
				log.Println("cherryPickCore probably never here ")
				datPicked[k] = make([]interface{}, 0)
			}
			// for i := range dat[k].([]interface{}) { // loop the slice
			// 	newdat := dat[k][i].(map[string]interface{})
			// 	newF := fi[k].(JSONFields)
			// 	newDatPicked := datPicked[k][i].(map[string]interface{})
			// 	cherryPickCore(newdat, &newF, newDatPicked)
			// }
		} else if dat[k] == nil { // no data
			if v2, ok := v.(string); ok {
				// if nil, change to this string
				datPicked[k] = v2
			} else if v2, ok := v.(int); ok {
				// if nil, change to this int
				datPicked[k] = v2
			} else if v == nil {
				// if nil, change to this int
				datPicked[k] = nil
			} else if _, ok := v.(JSONFieldEmptyList); ok {
				datPicked[k] = make([]interface{}, 0)
			} else if _, ok := v.(JSONFieldIgnoreIfEmpty); ok {
				// ignore it
			} else {
				// if nil, default to empty list
				datPicked[k] = make([]interface{}, 0)
			}
			// log.Println("reflect.TypeOf(dat[k]):", reflect.TypeOf(dat[k]))
		} else if datastring, ok := dat[k].(string); ok && datastring == "" {
			// I can use pointer in a model, but then govalidator is having issue with it
			// I don't know if I switch to Gin and govalidator things would be better?
			// So my strings in the model right now is not pointer
			// empty string
			if _, ok := v.(JSONFieldIgnoreIfEmpty); ok {
				// ignore it
			} else {
				datPicked[k] = ""
			}
		} else if newF, ok := v.(JSONFields); ok {
			// not slice. Usually this matches the case where there is an array of structs
			// inside a table and we're looping through that instance.
			// But it could also be like deviceStatus it's actually a nested struct of one
			embeddedStruct := make(map[string]interface{})
			cherryPickCore(dat[k].(map[string]interface{}), &newF, embeddedStruct)
			datPicked[k] = embeddedStruct
		} else {
			transformF, ok := v.(func(interface{}) interface{})
			if ok {
				datPicked[k] = transformF(dat[k])
			} else { // v can be nil, or other default value
				datPicked[k] = dat[k]
			}
		}
	}
}
