package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"
)

// StructToMapAndArray turns struct to map and arrays
func StructToMapAndArray(x interface{}) []interface{} {
	v := reflect.ValueOf(x)
	values := make([]interface{}, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		values[i] = v.Field(i).Interface()
	}
	return values
}

// JSONFields is the fields we specify to pick out only the JSON fields we want
type JSONFields map[string]interface{}

// JSONCherryPickFields picks only fields we want by the fields specifier
// when given a json string,
// I cannot cherry pick struct, because I can't make it generic enough
// to fit all models
// Then that means I have to loop the array to json marshal it
func JSONCherryPickFields(j []byte, f *JSONFields) ([]byte, error) {
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
		} else if dat[k] == nil {
			// if ID is in the name, it's a to-one reference that is optional,
			// so we don't put array in there, just omit it.
			if strings.HasSuffix(k, "ID") {
				datPicked[k] = nil
			} else {
				datPicked[k] = make([]interface{}, 0)
			}
			// log.Println("reflect.TypeOf(dat[k]):", reflect.TypeOf(dat[k]))
		} else { // not slice
			if v == nil {
				datPicked[k] = dat[k]
			} else { // transform (such as created_at to unix time)
				transformF := v.(func(interface{}) interface{})
				// transformF := (func(interface{}) interface{})(v)
				datPicked[k] = transformF(dat[k])
			}

		}
	}
}

// ParseBasicAuth is basically taken from HTTP
// It's just that I have to handle both basic and bearer
func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "

	// Case insensitive prefix match. See Issue 22736.
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return
	}

	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return
	}

	cs := string(c)
	s := strings.IndexByte(cs, ':')

	if s < 0 {
		return
	}

	return cs[:s], cs[s+1:], true
}

// BasicAuth get stuf from basicAuth
func BasicAuth(r *http.Request) (username, password string, ok bool) {
	var clientID, secret string
	ok = false

	authstring := r.Header.Get("Authorization")
	fields := strings.Split(authstring, ",")

	for _, v := range fields {
		if clientID, secret, ok = parseBasicAuth(v); ok {
			return clientID, secret, ok
		}
	}

	return clientID, secret, ok
}

// BearerToken returns bearer token
func BearerToken(r *http.Request) (token string, ok bool) {
	ok = false

	authstring := r.Header.Get("Authorization")
	fields := strings.Split(authstring, ",")

	for _, v := range fields {
		if ok = strings.HasPrefix(v, "Bearer"); ok {
			fmt.Sscanf(v, "Bearer %s", &token)
			return token, ok
		}
	}

	return "", false
}

// TimeNowInStr returns time in the format of "2006-01-02 15:04:05"
func TimeNowInStr() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}
