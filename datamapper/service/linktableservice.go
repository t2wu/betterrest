package service

import (
	"errors"
	"fmt"
	"log"
	"reflect"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/betterrest/model/mappertype"
	"github.com/t2wu/betterrest/registry"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

type LinkTableService struct {
	BaseService
}

// func (serv *LinkTableService) HookBeforeCreateOne(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel) (mdl.IModel, error) {
// 	linker, ok := modelObj.(mdlutil.ILinker)
// 	if !ok {
// 		return nil, fmt.Errorf("model not an ILinker object")
// 	}

// 	// Might not need this
// 	if modelObj.GetID() == nil {
// 		modelObj.SetID(datatype.NewUUID())
// 	}

// 	// You gotta have admin access to the model in order to create a relation
// 	err := userHasAdminAccessToOriginalModel(db, who.GetUserID(), typeString, linker.GetModelID())
// 	if err != nil {
// 		return nil, err
// 	}

// 	return modelObj, nil
// }

// func (serv *LinkTableService) HookBeforeCreateMany(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObjs []mdl.IModel) ([]mdl.IModel, error) {
// 	for _, modelObj := range modelObjs {
// 		linker, ok := modelObj.(mdlutil.ILinker)
// 		if !ok {
// 			return nil, fmt.Errorf("model not an ILinker object")
// 		}

// 		// You gotta have admin access to the model in order to create a relation
// 		err := userHasAdminAccessToOriginalModel(db, who.GetUserID(), typeString, linker.GetModelID())
// 		if err != nil {
// 			return nil, err
// 		}
// 	}

// 	return modelObjs, nil
// }

func (serv *LinkTableService) PermissionAndRole(data *hook.Data, ep *hook.EndPoint) (*hook.Data, *webrender.RetError) {
	ids := make([]*datatype.UUID, 0)

	for _, modelObj := range data.Ms {
		linker, ok := modelObj.(mdlutil.ILinker)
		if !ok {
			return nil, webrender.NewRetValErrorWithMsg("model not an ILinker object")
		}

		ids = append(ids, linker.GetModelID())
	}

	rolesMap, err := getRolesFromLinkTableIDs(data.DB, ep.Who.GetUserID(), ep.TypeString, ids)
	if err != nil {
		return nil, webrender.NewRetValWithError(err)
	}

	roles := make([]userrole.UserRole, 0)
	for _, modelObj := range data.Ms {
		linker := modelObj.(mdlutil.ILinker)
		mid := linker.GetModelID()
		role, ok := rolesMap[*mid]
		if !ok { // if not exist, the link isn't there, it's someone else's model
			return nil, webrender.NewRetValWithError(ErrPermission)
		}
		roles = append(roles, role)
	}
	data.Roles = roles

	// It's possible that hook want us to reject this endpoint
	if registry.RoleSorter != nil {
		if err := registry.RoleSorter.PermitOnCreate(mappertype.LinkTable, data, ep); err != nil {
			return nil, err
		}
	}

	return data, nil
}

func (serv *LinkTableService) HookBeforeDeleteOne(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel) (mdl.IModel, error) {
	return modelObj, nil
}

func (serv *LinkTableService) HookBeforeDeleteMany(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObjs []mdl.IModel) ([]mdl.IModel, error) {
	return modelObjs, nil
}

// ReadOneCore get one model object based on its type and its id string
func (service *LinkTableService) ReadOneCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, id *datatype.UUID, options map[urlparam.Param]interface{}) (mdl.IModel, userrole.UserRole, error) {
	modelObj := registry.NewFromTypeString(typeString)

	// Check if link table
	if _, ok := modelObj.(mdlutil.ILinker); !ok {
		log.Printf("%s not an ILinker type\n", typeString)
		return nil, userrole.UserRoleInvalid, fmt.Errorf("%s not an ILinker type", typeString)
	}

	rtable := mdl.GetTableNameFromIModel(modelObj)

	// Subquery: find all mdl where user_id has ME in it, then find
	// record where model_id from subquery and id matches the one we query for

	// Specify user_id because you gotta own this or is a guest to this
	subquery := fmt.Sprintf("model_id IN (select model_id from %s where user_id = ?)", rtable)

	err := db.Table(rtable).Where(subquery, who.GetUserID()).Where("id = ?", id).Find(modelObj).Error
	// err := db.Table(rtable).Where(subquery, oid).Where("user_id = ?", &id).Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	// It doesn't have to be mdlutil.OwnershipModelWithIDBase but it has to have this field
	modelID := reflect.ValueOf(modelObj).Elem().FieldByName(("ModelID")).Interface().(*datatype.UUID)

	// The role for this role is determined on the role of the row where the user_id is YOU
	type result struct {
		Role userrole.UserRole
	}
	res := result{}
	if err := db.Table(rtable).Where("user_id = ? and role = ? and model_id = ?",
		who.GetUserID(), userrole.UserRoleAdmin, modelID).Select("role").Scan(&res).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, userrole.UserRoleInvalid, ErrPermission
		}
		return nil, userrole.UserRoleInvalid, err // some other error
	}

	return modelObj, res.Role, err
}

func (serv *LinkTableService) GetManyCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, ids []*datatype.UUID, options map[urlparam.Param]interface{}) ([]mdl.IModel, []userrole.UserRole, error) {
	rtable := registry.GetTableNameFromTypeString(typeString)
	subquery := fmt.Sprintf("model_id IN (select model_id from %s where user_id = ?)", rtable)
	db2 := db.Table(rtable).Where(subquery, who.GetUserID()).Where("id IN (?)", ids)
	modelObjs, err := registry.NewSliceFromDBByTypeString(typeString, db2.Set("gorm:auto_preload", true).Find)
	if err != nil {
		return nil, nil, err
	}

	// Just in case err didn't work (as in the case with IN clause NOT in the ID field, maybe Gorm bug)
	if len(modelObjs) == 0 {
		return nil, nil, fmt.Errorf("not found")
	}

	if len(modelObjs) != len(ids) {
		return nil, nil, ErrBatchUpdateOrPatchOneNotFound
	}

	// Role is more complicated. I need to find for each unique model_id, when I have a corresponding
	// link to it, what role is it?
	// So currently I only know how to fetch one by one
	// I probably can do it in one go, need extra time.
	// Probably using the query in the beginnign of the function but by selecting the Role column
	// TODO
	roles := make([]userrole.UserRole, len(modelObjs))
	for i, modelObj := range modelObjs {
		id := modelObj.GetID()
		_, roles[i], err = serv.userHasPermissionToEdit(db, who, typeString, id)
		if err != nil {
			return nil, nil, err
		}
	}

	return modelObjs, roles, nil
}

// GetAllQueryContructCore construct query core
func (serv *LinkTableService) GetAllQueryContructCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string) (*gorm.DB, error) {
	rtable := registry.GetTableNameFromTypeString(typeString)

	// Check if link table
	testModel := registry.NewFromTypeString(typeString)
	// ILinker means link table
	if _, ok := testModel.(mdlutil.ILinker); !ok {
		log.Printf("%s not an ILinker type\n", typeString)
		return nil, fmt.Errorf("%s not an ILinker type", typeString)
	}

	// select * from rtable where model_id IN (select model_id from rtable where user_id = ?)
	// subquery := db.Where("user_id = ?", oid).Table(rtable)
	subquery := fmt.Sprintf("model_id IN (select model_id from %s where user_id = ?)", rtable)
	db = db.Table(rtable).Where(subquery, who.GetUserID())

	return db, nil
}

// GetAllRolesCore gets all roles according to the criteria
func (serv *LinkTableService) GetAllRolesCore(dbChained *gorm.DB, dbClean *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObjs []mdl.IModel) ([]userrole.UserRole, error) {
	// No roles for this table, because this IS the linking table
	roles := make([]userrole.UserRole, len(modelObjs))
	for i := range roles {
		roles[i] = userrole.UserRoleInvalid // FIXME It shouldn't be Invaild, it should be the user's access to this
	}

	return roles, nil
}

func (serv *LinkTableService) userHasPermissionToEdit(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, id *datatype.UUID) (mdl.IModel, userrole.UserRole, error) {
	if id == nil || id.UUID.String() == "" {
		return nil, userrole.UserRoleInvalid, ErrIDEmpty
	}

	// Kind of hack, but update don't have any parameter anyway
	// This was for parittioned table where read has to have some date
	options := make(map[urlparam.Param]interface{}, 0)

	// Pull out entire modelObj
	modelObj, _, err := serv.ReadOneCore(db, who, typeString, id, options)
	if err != nil { // Error is "record not found" when not found
		return nil, userrole.UserRoleInvalid, err
	}

	uuidVal := modelObj.GetID()
	if uuidVal == nil || uuidVal.String() == "" {
		// in case it's an empty string
		return nil, userrole.UserRoleInvalid, ErrIDEmpty
	} else if uuidVal.String() != id.UUID.String() {
		return nil, userrole.UserRoleInvalid, ErrIDNotMatch
	}

	linker, ok := modelObj.(mdlutil.ILinker)
	if !ok {
		return nil, userrole.UserRoleInvalid, fmt.Errorf("model not an ILinker object")
	}

	// If you're admin to this model, you can only update/delete link data to other
	// If you're guest to this model, then you can remove yourself, but not others
	rtable := registry.GetTableNameFromTypeString(typeString)
	type result struct {
		Role userrole.UserRole
	}
	res := result{}
	if err := db.Table(rtable).Where("user_id = ? and model_id = ?", who.GetUserID(), linker.GetModelID()).First(&res).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, userrole.UserRoleInvalid, ErrPermission
		}
		return nil, userrole.UserRoleInvalid, err
	}

	if res.Role == userrole.UserRoleAdmin && linker.GetUserID().String() == who.GetUserID().String() {
		// You can remove other's relation, but not yours
		return nil, res.Role, ErrPermissionWrongEndPoint
	} else if res.Role != userrole.UserRoleAdmin && linker.GetUserID().String() != who.GetUserID().String() {
		// not admin, only remove yourself
		return nil, res.Role, ErrPermission
	}

	return modelObj, res.Role, nil
}

// ---------------------------------------
// returns orgID to userRole mapping
func getRolesFromLinkTableIDs(db *gorm.DB, oid *datatype.UUID, typeString string, ids []*datatype.UUID) (map[datatype.UUID]userrole.UserRole, error) {
	// We need to find at least one role with the same model id
	// where we're admin for

	// We make sure we NOT by checking the original model table
	// but check link table which we have admin access for
	rtable := registry.GetTableNameFromTypeString(typeString)

	results := []struct {
		ModelID *datatype.UUID
		Role    userrole.UserRole
	}{}

	rolesMap := make(map[datatype.UUID]userrole.UserRole)
	if err := db.Table(rtable).Where("user_id = ? and model_id IN (?)",
		oid, ids).Select([]string{"DISTINCT model_id", "role"}).Scan(&results).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPermission
		}
		return nil, err
	}

	for _, res := range results {
		rolesMap[*res.ModelID] = res.Role
	}

	return rolesMap, nil
}

// returns orgID to userRole mapping
// func obtainRoleFromLinkTable(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObjs []mdl.IModel) (map[datatype.UUID]userrole.UserRole, error) {
// 	rolesMap := make(map[datatype.UUID]userrole.UserRole)
// 	orgTableName := registry.OrgModelNameFromOrgResourceTypeString(typeString)
// 	linkTableName := orgJoinTableName(typeString)

// 	orgIDsMap := make(map[datatype.UUID]struct{})
// 	for _, modelObj := range modelObjs {
// 		orgID := mdlutil.GetFieldValueFromModelByTagKeyBetterRestAndValueKey(modelObj, "org").(*datatype.UUID)
// 		if orgID == nil {
// 			return nil, fmt.Errorf("missing %sID", orgTableName) // TODO table name is not necessarily endpoint name
// 		}
// 		orgIDsMap[*orgID] = struct{}{}
// 	}

// 	orgIDs := make([]*datatype.UUID, 0)
// 	for orgID := range orgIDsMap {
// 		orgID2 := &orgID
// 		orgIDs = append(orgIDs, datatype.NewUUIDFromStringNoErr(orgID2.String())) // just in case that reference can get resetted...
// 	}

// 	whereStmt := fmt.Sprintf(`"%s".model_id IN (?) AND "%s".user_id = ?`, linkTableName, linkTableName)

// 	// 1. Is it possible that the user has multiple ways to link to this organization?
// 	// Let's say there is not.
// 	// 2. If somehow you don't belong to this organization, then it won't be found.
// 	results := make([]struct {
// 		ModelID *datatype.UUID
// 		Role    userrole.UserRole
// 	}, 0)
// 	if err := db.Table(linkTableName).Where(whereStmt, orgIDs, who.GetUserID().String()).
// 		Select([]string{"DISTINCT model_id", "role"}).Scan(&results).Error; err != nil {
// 		return nil, err
// 	}
// 	for _, res := range results {
// 		rolesMap[*res.ModelID] = res.Role
// 	}

// 	return rolesMap, nil
// }

// func userHasAdminAccessToOriginalModel(db *gorm.DB, oid *datatype.UUID, typeString string, id *datatype.UUID) error {
// 	// We need to find at least one role with the same model id
// 	// where we're admin for

// 	// We make sure we NOT by checking the original model table
// 	// but check link table which we have admin access for
// 	rtable := registry.GetTableNameFromTypeString(typeString)

// 	result := struct {
// 		ID *datatype.UUID
// 	}{}

// 	if err := db.Table(rtable).Where("user_id = ? and role = ? and model_id = ?",
// 		oid, userrole.UserRoleAdmin, id).Scan(&result).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ErrPermission
// 		}
// 		return err
// 	}

// 	return nil
// }

// UpdateOneCore one, permissin should already be checked
// called for patch operation as well (after patch has already applied)
// Fuck, repeat the following code for now (you can't call the overriding method from the non-overriding one)
func (serv *LinkTableService) UpdateOneCore(db *gorm.DB, who mdlutil.UserIDFetchable, typeString string, modelObj mdl.IModel, id *datatype.UUID, oldModelObj mdl.IModel) (modelObj2 mdl.IModel, err error) {
	if modelNeedsRealDelete(oldModelObj) { // parent model
		db = db.Unscoped()
	}

	err = gormfixes.UpdatePeggedFields(db, oldModelObj, modelObj)
	if err != nil {
		return nil, err
	}

	if err = db.Save(modelObj).Error; err != nil { // save updates all fields (FIXME: need to check for required)
		return nil, err
	}

	// Kind of hack, but update don't have any parameter anyway
	// This was for parittioned table where read has to have some date
	options := make(map[urlparam.Param]interface{}, 0)

	// This loads the IDs
	// This so we have the preloading.
	modelObj2, _, err = serv.ReadOneCore(db, who, typeString, id, options)
	if err != nil { // Error is "record not found" when not found
		return nil, err
	}

	// ouch! for many to many we need to remove it again!!
	// because it's in a transaction so it will load up again
	gormfixes.FixManyToMany(modelObj, modelObj2)

	return modelObj2, nil
}
