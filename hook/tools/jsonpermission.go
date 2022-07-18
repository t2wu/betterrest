package tools

import (
	"encoding/json"
	"reflect"

	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/utils/jsontrans"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/qry/mdl"
)

// ToJSON pack json into this struct and the role
func ToJSON(v mdl.IModel, r userrole.UserRole, who mdlutil.UserIDFetchable) ([]byte, error) {
	var j []byte
	var err error

	var dataPicked map[string]interface{}

	// Custom permission
	if modelObjPerm, ok := v.(mdlutil.IHasPermissions); ok {
		permType, fields := modelObjPerm.Permissions(r, who)
		// FIXME
		// log.Println("Note: permissionTypeBlackList not supported yet, currently:", permType)

		// White list or black list, we need to make it available for Transform to control
		includeCUDDates := true
		dataPicked = transFromByHidingDateFieldsFromIModel(v, includeCUDDates)

		if permType == jsontrans.PermissionWhiteList {
			dataPicked, err = jsontrans.Transform(dataPicked, &fields, jsontrans.PermissionWhiteList)
		} else {
			// TODO, currently doesn't work
			dataPicked, err = jsontrans.Transform(dataPicked, &fields, jsontrans.PermissionWhiteList)
		}
		if err != nil {
			return nil, err
		}
	} else {
		// By default just hide all date fields and return everything else
		// Traversing with the original mdl.IModel
		includeCUDDates := false
		dataPicked = transFromByHidingDateFieldsFromIModel(v, includeCUDDates)
	}

	if j, err = json.Marshal(dataPicked); err != nil {
		return nil, err
	}
	return j, nil
}

// FromJSON unpacks json into this struct
func FromJSON(v interface{}, j []byte) error {
	return json.Unmarshal(j, v)
}

func transFromByHidingDateFieldsFromIModel(modelObj mdl.IModel, includeCUDDates bool) map[string]interface{} {
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	return jsontrans.TransFromByHidingDateFieldsFromIModel(v, includeCUDDates)
}
