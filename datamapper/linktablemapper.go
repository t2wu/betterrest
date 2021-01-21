package datamapper

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

// ---------------------------------------

func userHasAdminAccessToOriginalModel(db *gorm.DB, oid *datatypes.UUID, typeString string) error {
	// We make sure we do not by checking the original model table
	// but check link table which we have admin access for
	rtable := models.GetTableNameFromTypeString(typeString)
	if err := db.Table(rtable).Where("user_id = ? and role = ?", oid, models.Admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errPermission
		}
		return err
	}
	return nil
}

func userHasPermissionToEdit(mapper *LinkTableMapper, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, error) {
	// Pull out entire modelObj
	modelObj, _, err := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return nil, err
	}

	ownerModelObj, ok := modelObj.(models.IOwnership)
	if !ok {
		return nil, fmt.Errorf("model not an IOwnership object")
	}

	// If you're admin to this model, you can only update/delete link data to other
	// If you're guest to this model, then you can remove yourself, but not others
	rtable := models.GetTableNameFromTypeString(typeString)
	type result struct {
		Role models.UserRole
	}
	res := result{}
	if err := db.Table(rtable).Where("user_id = ? and model_id = ?", oid, ownerModelObj.GetModelID()).First(&res).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errPermission
		}
		return nil, err
	}

	if res.Role == models.Admin && ownerModelObj.GetUserID().String() == oid.String() {
		// You can remove other's relation, but not yours
		return nil, errPermissionWrongEndPoint
	} else if res.Role != models.Admin && ownerModelObj.GetUserID().String() != oid.String() {
		// not admin, only remove yourself
		return nil, errPermission
	}
	return modelObj, nil
}

// ---------------------------------------

var onceLinkTableMapper sync.Once
var linkeTableMapper *LinkTableMapper

// LinkTableMapper is a basic CRUD manager
type LinkTableMapper struct {
}

// SharedLinkTableMapper creats a singleton of Crud object
func SharedLinkTableMapper() *LinkTableMapper {
	onceLinkTableMapper.Do(func() {
		linkeTableMapper = &LinkTableMapper{}
	})

	return linkeTableMapper
}

// CreateOne creates an instance of this model based on json and store it in db
// when creating, need to put yourself in OrganizationUser as well.
// Well check this!!
func (mapper *LinkTableMapper) CreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	if modelObj.GetID() == nil {
		modelObj.SetID(datatypes.NewUUID())
	}

	// You gotta have admin access to the model in order to create a relation
	err := userHasAdminAccessToOriginalModel(db, oid, typeString)
	if err != nil {
		return nil, err
	}

	ownerModelObj, ok := modelObj.(models.IOwnership)
	if !ok {
		return nil, fmt.Errorf("model not an IOwnership object")
	}

	userID := ownerModelObj.GetUserID()

	// This user actually has to exists!
	// Again the user table needs to be called "user" (limitation)
	// Unless I provide an interface to register it specifically
	type result struct {
		ID *datatypes.UUID
	}
	res := result{}
	if err := db.Table("user").Select("id").Where("id = ?", userID).Scan(&res).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user does not exists")
		}
	}

	// Now, we create the object
	// Use createOneCoreOrganization, organization one fit this need
	// perhaps we should rename it. It differs from the ownership one
	// in that it expect no ownership table we need to manually link
	return createOneWithHooks(createOneCoreOrganization, db, oid, scope, typeString, modelObj)
}

// CreateMany creates an instance of this model based on json and store it in db
func (mapper *LinkTableMapper) CreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	retModels := make([]models.IModel, 0, 20)

	cargo := models.BatchHookCargo{}
	// Before batch inert hookpoint
	if before := models.ModelRegistry[typeString].BeforeInsert; before != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := before(bhpData); err != nil {
			return nil, err
		}
	}

	for _, modelObj := range modelObjs {
		if modelObj.GetID() == nil {
			modelObj.SetID(datatypes.NewUUID())
		}

		// You gotta have admin access to the model in order to create a relation
		err := userHasAdminAccessToOriginalModel(db, oid, typeString)
		if err != nil {
			return nil, err
		}

		m, err := createOneCoreOrganization(db, oid, typeString, modelObj)
		if err != nil {
			// That's weird. we have just inserted it.
			return nil, err
		}

		retModels = append(retModels, m)
	}

	// After batch insert hookpoint
	if after := models.ModelRegistry[typeString].AfterInsert; after != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := after(bhpData); err != nil {
			return nil, err
		}
	}

	return retModels, nil
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *LinkTableMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error) {

	modelObj, role, err := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil {
		return nil, 0, err
	}

	if m, ok := modelObj.(models.IAfterRead); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Role: &role}
		if err := m.AfterReadDB(hpdata); err != nil {
			return nil, 0, err
		}
	}

	return modelObj, role, err
}

// getOneWithIDCore get one model object based on its type and its id string
// since this is organizationMapper, need to make sure it's the same organization
func (mapper *LinkTableMapper) getOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj := models.NewFromTypeString(typeString)

	// Check if link table
	if _, ok := modelObj.(models.IOwnership); !ok {
		log.Printf("%s not an IOwnership type\n", typeString)
		return nil, models.Invalid, fmt.Errorf("%s not an IOwnership type", typeString)
	}

	rtable := models.GetTableNameFromIModel(modelObj)

	// Subquery: find all models where user_id has ME in it, then find
	// record where model_is from subquery and id matches the one we query for

	// Specify user_id because you gotta own this or is a guest to this
	subquery := fmt.Sprintf("model_id IN (select model_id from %s where user_id = ?)", rtable)

	err := db.Table(rtable).Where(subquery, oid).Where("id = ?", &id).Find(modelObj).Error
	// err := db.Table(rtable).Where(subquery, oid).Where("user_id = ?", &id).Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	// No roles for this table, because this IS the linking table
	return modelObj, models.Invalid, err
}

// ReadAll obtains a slice of models.DomainModel
// options can be string "offset" and "limit", both of type int
// This is very Javascript-esque. I would have liked Python's optional parameter more.
// Alas, no such feature in Go. https://stackoverflow.com/questions/2032149/optional-parameters-in-go
// How does Gorm do the following? Might want to check out its source code.
// Cancel offset condition with -1
//  db.Offset(10).Find(&users1).Offset(-1).Find(&users2)
func (mapper *LinkTableMapper) ReadAll(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, options map[URLParam]interface{}) ([]models.IModel, []models.UserRole, *int, error) {
	db2 := db
	// db = db.Set("gorm:auto_preload", true) // no preloadinig needed

	// Check if link table
	testModel := models.NewFromTypeString(typeString)
	// IOwnership means link table
	if _, ok := testModel.(models.IOwnership); !ok {
		log.Printf("%s not an IOwnership type\n", typeString)
		return nil, nil, nil, fmt.Errorf("%s not an IOwnership type", typeString)
	}

	// Read all link table who user_id is mine, role is admin or guest
	// Get all the model_ids. And then get the all user_id's linked to those model_ids
	offset, limit, cstart, cstop, order, latestn, totalcount := getOptions(options)

	rtable := models.GetTableNameFromTypeString(typeString)
	if cstart != nil && cstop != nil {
		db = db.Where(rtable+".created_at BETWEEN ? AND ?", time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))
	}

	var err error

	db, err = constructInnerFieldParamQueries(db, typeString, options, latestn)
	if err != nil {
		return nil, nil, nil, err
	}

	// select * from rtable where model_id IN (select model_id from rtable where user_id = ?)
	// subquery := db.Where("user_id = ?", oid).Table(rtable)
	subquery := fmt.Sprintf("model_id IN (select model_id from %s where user_id = ?)", rtable)
	db = db.Table(rtable).Where(subquery, oid)

	db = constructOrderFieldQueries(db, rtable, order)

	var no *int
	if totalcount {
		no = new(int)
		// Query for total count, without offset and limit (all)
		if err := db.Count(no).Error; err != nil {
			log.Println("count error:", err)
			return nil, nil, nil, err
		}
	}

	if offset != nil && limit != nil {
		db = db.Offset(*offset).Limit(*limit)
	}

	outmodels, err := models.NewSliceFromDBByTypeString(typeString, db.Find) // error from db is returned from here

	// No roles for this table, because this IS the linking table
	roles := make([]models.UserRole, len(outmodels), len(outmodels))
	for i := range roles {
		roles[i] = models.Invalid
	}

	// use db2 cuz it's not chained
	if after := models.ModelRegistry[typeString].AfterRead; after != nil {
		bhpData := models.BatchHookPointData{Ms: outmodels, DB: db2, OID: oid, Scope: scope, TypeString: typeString, Roles: roles}
		if err = after(bhpData); err != nil {
			return nil, nil, nil, err
		}
	}

	return outmodels, roles, no, err
}

// UpdateOneWithID updates model based on this json
func (mapper *LinkTableMapper) UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id datatypes.UUID) (models.IModel, error) {
	_, err := userHasPermissionToEdit(mapper, db, oid, scope, typeString, id)
	if err != nil {
		return nil, err
	}

	cargo := models.ModelCargo{}

	// Before hook
	if v, ok := modelObj.(models.IBeforeUpdate); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := v.BeforeUpdateDB(hpdata); err != nil {
			return nil, err
		}
	}

	modelObj2, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, id, models.UserRoleAny)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterUpdate); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = v.AfterUpdateDB(hpdata); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}

// UpdateMany updates multiple models
func (mapper *LinkTableMapper) UpdateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	ms := make([]models.IModel, 0, 0)
	var err error
	cargo := models.BatchHookCargo{}

	// Before batch update hookpoint
	if before := models.ModelRegistry[typeString].BeforeUpdate; before != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = before(bhpData); err != nil {
			return nil, err
		}
	}

	for _, modelObj := range modelObjs {
		id := modelObj.GetID()

		_, err := userHasPermissionToEdit(mapper, db, oid, scope, typeString, *id)
		if err != nil {
			return nil, err
		}

		m, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, *id, models.Admin)
		if err != nil { // Error is "record not found" when not found
			return nil, err
		}

		ms = append(ms, m)
	}

	// After batch update hookpoint
	if after := models.ModelRegistry[typeString].AfterUpdate; after != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = after(bhpData); err != nil {
			return nil, err
		}
	}

	return ms, nil
}

// PatchOneWithID updates model based on this json
func (mapper *LinkTableMapper) PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonPatch []byte, id datatypes.UUID) (models.IModel, error) {
	// Check id error
	if id.UUID.String() == "" {
		return nil, errIDEmpty
	}

	modelObj, err := userHasPermissionToEdit(mapper, db, oid, scope, typeString, id)
	if err != nil {
		return nil, err
	}

	// Apply patch operations
	modelObj, err = patchOneCore(typeString, modelObj, jsonPatch)
	if err != nil {
		return nil, err
	}

	cargo := models.ModelCargo{}
	// Before hook
	// It is now expected that the hookpoint for before expect that the patch
	// gets applied to the JSON, but not before actually updating to DB.
	if v, ok := modelObj.(models.IBeforePatch); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err := v.BeforePatchDB(hpdata); err != nil {
			return nil, err
		}
	}

	// Now save it
	modelObj2, err := updateOneCore(mapper, db, oid, scope, typeString, modelObj, id, models.Admin)
	if err != nil {
		return nil, err
	}

	// After hook
	if v, ok := modelObj2.(models.IAfterPatch); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = v.AfterPatchDB(hpdata); err != nil {
			return nil, err
		}
	}

	return modelObj2, nil
}

// DeleteOneWithID delete the model
func (mapper *LinkTableMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id datatypes.UUID) (models.IModel, error) {
	if id.UUID.String() == "" {
		return nil, errIDEmpty
	}

	modelObj, err := userHasPermissionToEdit(mapper, db, oid, scope, typeString, id)
	if err != nil {
		return nil, err
	}
	// Now safe to delete no matter who you are

	// You're admin, and the link is to someone else, that someone can also be an admin or not
	cargo := models.ModelCargo{}

	// Before delete hookpoint
	if v, ok := modelObj.(models.IBeforeDelete); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		err = v.BeforeDeleteDB(hpdata)
		if err != nil {
			return nil, err
		}
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	err = db.Delete(modelObj).Error
	if err != nil {
		return nil, err
	}

	// After delete hookpoint
	if v, ok := modelObj.(models.IAfterDelete); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		err = v.AfterDeleteDB(hpdata)
		if err != nil {
			return nil, err
		}
	}

	return modelObj, nil
}

// DeleteMany deletes multiple models
func (mapper *LinkTableMapper) DeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {

	ids := make([]datatypes.UUID, len(modelObjs), len(modelObjs))
	var err error
	cargo := models.BatchHookCargo{}
	for i, v := range modelObjs {
		id := v.GetID()
		if id.String() == "" {
			return nil, errIDEmpty
		}

		ids[i] = *id
	}

	ms := make([]models.IModel, 0, 0)

	// Before batch delete hookpoint
	if before := models.ModelRegistry[typeString].BeforeDelete; before != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = before(bhpData); err != nil {
			return nil, err
		}
	}

	for i, id := range ids {

		if id.UUID.String() == "" {
			return nil, errIDEmpty
		}

		modelObj, err := userHasPermissionToEdit(mapper, db, oid, scope, typeString, id)
		if err != nil {
			return nil, err
		}
		// Now safe to delete no matter who you are

		// Unscoped() for REAL delete!
		// Foreign key constraint works only on real delete
		// Soft delete will take more work, have to verify myself manually
		if modelNeedsRealDelete(modelObj) && i == 0 { // only do once
			db = db.Unscoped()
		}

		err = db.Delete(modelObj).Error
		// err = db.Delete(modelObj).Error
		if err != nil {
			return nil, err
		}

		err = removePeggedField(db, modelObj)
		if err != nil {
			return nil, err
		}

		ms = append(ms, modelObj)
	}

	// After batch delete hookpoint
	if after := models.ModelRegistry[typeString].AfterDelete; after != nil {
		bhpData := models.BatchHookPointData{Ms: modelObjs, DB: db, OID: oid, Scope: scope, TypeString: typeString, Cargo: &cargo}
		if err = after(bhpData); err != nil {
			return nil, err
		}
	}

	return ms, nil
}
