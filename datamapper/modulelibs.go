package datamapper

import (
	"encoding/json"
	"errors"
	"log"
	"net/url"

	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/betterrest/registry"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/jinzhu/gorm"
)

// TODO: This method repeated twice, not sure where to put it
func modelNeedsRealDelete(modelObj mdl.IModel) bool {
	// real delete by default
	realDelete := true
	if modelObj2, ok := modelObj.(mdlutil.IDoRealDelete); ok {
		realDelete = modelObj2.DoRealDelete()
	}
	return realDelete
}

func constructInnerFieldParamQueries(db *gorm.DB, typeString string, options map[urlparam.Param]interface{}, latestn *int, latestnons []string) (*gorm.DB, error) {
	if urlParams, ok := options[urlparam.ParamOtherQueries].(url.Values); ok && len(urlParams) != 0 {
		var err error
		// If I want quering into nested data
		// I need INNER JOIN that table where the field is what we search for,
		// and that table's link back to this ID is the id of this table
		db, err = constructDbFromURLFieldQuery(db, typeString, urlParams, latestn, latestnons)
		if err != nil {
			return nil, err
		}

		db, err = constructDbFromURLInnerFieldQuery(db, typeString, urlParams, latestn)
		if err != nil {
			return nil, err
		}
	} else if latestn != nil && len(latestnons) == 0 {
		return nil, errors.New("use latestnon with latestn")
	}

	return db, nil
}

func verifyModelIDCorrectnessForOne(modelObj mdl.IModel, id *datatype.UUID) error {
	if id == nil || id.UUID.String() == "" {
		// in case it's an empty string
		return service.ErrIDEmpty
	}

	// Check if ID from URL and ID in object are the same (meaningful when it's not batch edit)
	// modelObj is nil if it's a patch operation. In that case just here to load and check permission.
	// it's also nil when it's a get one op
	if modelObj != nil && modelObj.GetID().String() != id.String() {
		return service.ErrIDNotMatch
	}

	return nil
}

func loadAndCheckErrorBeforeModifyV2(serv service.IService, db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, permittedRoles []userrole.UserRole, options map[urlparam.Param]interface{}) (mdl.IModel, userrole.UserRole, error) {
	if id == nil || id.UUID.String() == "" {
		// in case it's an empty string
		return nil, userrole.UserRoleInvalid, service.ErrIDEmpty
	}

	// Check if ID from URL and ID in object are the same (meaningful when it's not batch edit)
	// modelObj is nil if it's a patch operation. In that case just here to load and check permission.
	// it's also nil when it's a get one op
	if modelObj != nil && modelObj.GetID().String() != id.String() {
		return nil, userrole.UserRoleInvalid, service.ErrIDNotMatch
	}

	// TODO: Is there a more efficient way?
	// For ownership: role is the role of the model to the user
	// for mdl under organization, the role is the role of the organization to the user
	modelObj2, role, err := serv.ReadOneCore(db, who, typeString, id, options)
	if err != nil { // Error is "record not found" when not found
		return nil, userrole.UserRoleInvalid, err
	}

	permitted := false
	for _, permittedRole := range permittedRoles {
		if permittedRole == userrole.UserRoleAny {
			permitted = true
			break
		} else if role == permittedRole {
			permitted = true
			break
		}
	}
	if !permitted {
		return nil, userrole.UserRoleInvalid, service.ErrPermission
	}

	return modelObj2, role, nil
}

// func loadAndCheckErrorBeforeModifyV1(serv service.IServiceV1, db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, permittedRoles []userrole.UserRole) (mdl.IModel, userrole.UserRole, error) {
// 	if id == nil || id.UUID.String() == "" {
// 		// in case it's an empty string
// 		return nil, userrole.UserRoleInvalid, service.ErrIDEmpty
// 	}

// 	// Check if ID from URL and ID in object are the same (meaningful when it's not batch edit)
// 	// modelObj is nil if it's a patch operation. In that case just here to load and check permission.
// 	// it's also nil when it's a get one op
// 	if modelObj != nil && modelObj.GetID().String() != id.String() {
// 		return nil, userrole.UserRoleInvalid, service.ErrIDNotMatch
// 	}

// 	// TODO: Is there a more efficient way?
// 	// For ownership: role is the role of the model to the user
// 	// for mdl under organization, the role is the role of the organization to the user
// 	modelObj2, role, err := serv.ReadOneCore(db, who, typeString, id)
// 	if err != nil { // Error is "record not found" when not found
// 		return nil, userrole.UserRoleInvalid, err
// 	}

// 	permitted := false
// 	for _, permittedRole := range permittedRoles {
// 		if permittedRole == userrole.UserRoleAny {
// 			permitted = true
// 			break
// 		} else if role == permittedRole {
// 			permitted = true
// 			break
// 		}
// 	}
// 	if !permitted {
// 		return nil, userrole.UserRoleInvalid, service.ErrPermission
// 	}

// 	return modelObj2, role, nil
// }

// // db should already be set up for all the joins needed, if any
// func loadManyAndCheckBeforeModifyV1(serv service.IServiceV1, db *gorm.DB, who mdlutil.UserIDFetchable, typeString string,
// 	ids []*datatype.UUID, permittedRoles []userrole.UserRole) ([]mdl.IModel, []userrole.UserRole, error) {
// 	// log.Println("loadManyAndCheckBeforeModifyV1 run")
// 	modelObjs, roles, err := serv.GetManyCore(db, who, typeString, ids)
// 	if err != nil {
// 		log.Println("calling getManyWithIDsCore err:", err)
// 		return nil, nil, err
// 	}

// 	// for _, role := range roles {
// 	// 	log.Printf("role: %v\n", role)
// 	// 	if role != userrole.UserRoleAdmin {
// 	// 		return nil, nil, service.ErrPermission
// 	// 	}
// 	// }

// 	for _, role := range roles {
// 		permitted := false
// 		for _, permittedRole := range permittedRoles {
// 			if permittedRole == userrole.UserRoleAny {
// 				permitted = true
// 				break
// 			} else if role == permittedRole {
// 				permitted = true
// 				break
// 			}
// 		}
// 		if !permitted {
// 			return nil, nil, service.ErrPermission
// 		}
// 	}

// 	return modelObjs, roles, nil
// }

// V3 uses the new permission, and return RetError
func loadManyAndCheckBeforeModifyV3(serv service.IService, db *gorm.DB, who mdlutil.UserIDFetchable, typeString string,
	ids []*datatype.UUID, options map[urlparam.Param]interface{}, permittedRoles map[userrole.UserRole]*webrender.RetError) ([]mdl.IModel, []userrole.UserRole, *webrender.RetError) {
	// log.Println("loadManyAndCheckBeforeModifyV2 run, who userID?", who.GetUserID())
	modelObjs, roles, err := serv.GetManyCore(db, who, typeString, ids, options)
	if err != nil {
		log.Println("calling getManyWithIDsCore err:", err)
		return nil, nil, webrender.NewRetValWithError(err)
	}

	// The user has a relationship to this object, but whether this action is allowed or not for the relationship to this object
	// is determined by accessPermissions (which was from hook)
	for _, role := range roles {
		anyRetError, ok := permittedRoles[role]
		if !ok {
			// not in there, so it's not permitted
			return nil, nil, webrender.NewRetValWithError(service.ErrPermission)
		}

		if anyRetError != nil {
			// Custom error
			return nil, nil, anyRetError
		}

		// Ok really permitted
	}

	return modelObjs, roles, nil
}

func applyPatchCore(typeString string, modelObj mdl.IModel, jsonPatch []byte) (modelObj2 mdl.IModel, err error) {
	// Apply patch operations
	// This library actually works in []byte

	var modelInBytes []byte
	modelInBytes, err = json.Marshal(modelObj)
	if err != nil {
		return nil, service.ErrPatch // the errors often not that helpful anyway
	}

	var patch jsonpatch.Patch
	patch, err = jsonpatch.DecodePatch(jsonPatch)
	if err != nil {
		return nil, err
	}

	var modified []byte
	modified, err = patch.Apply(modelInBytes)
	if err != nil {
		return nil, err
	}

	// Now turn it back to modelObj
	modelObj2 = registry.NewFromTypeString(typeString)
	err = json.Unmarshal(modified, modelObj2)
	if err != nil {
		// there shouldn't be any error unless it's a patching mistake...
		return nil, err
	}

	return modelObj2, nil
}
