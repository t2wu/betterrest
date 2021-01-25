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

func userHasAdminAccessToOriginalModel(db *gorm.DB, oid *datatypes.UUID, typeString string, id *datatypes.UUID) error {
	// We need to find at least one role with the same model id
	// where we're admin for

	// We make sure we NOT by checking the original model table
	// but check link table which we have admin access for
	rtable := models.GetTableNameFromTypeString(typeString)
	if err := db.Table(rtable).Where("user_id = ? and role = ? and model_id = ?",
		oid, models.Admin, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errPermission
		}
		return err
	}
	return nil
}

func userHasPermissionToEdit(mapper *LinkTableMapper, db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	if id == nil || id.UUID.String() == "" {
		return nil, models.Invalid, errIDEmpty
	}

	// Pull out entire modelObj
	modelObj, _, err := mapper.getOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil { // Error is "record not found" when not found
		return nil, models.Invalid, err
	}

	uuidVal := modelObj.GetID()
	if uuidVal == nil || uuidVal.String() == "" {
		// in case it's an empty string
		return nil, models.Invalid, errIDEmpty
	} else if uuidVal.String() != id.UUID.String() {
		return nil, models.Invalid, errIDNotMatch
	}

	ownerModelObj, ok := modelObj.(models.IOwnership)
	if !ok {
		return nil, models.Invalid, fmt.Errorf("model not an IOwnership object")
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
			return nil, models.Invalid, errPermission
		}
		return nil, models.Invalid, err
	}

	if res.Role == models.Admin && ownerModelObj.GetUserID().String() == oid.String() {
		// You can remove other's relation, but not yours
		return nil, res.Role, errPermissionWrongEndPoint
	} else if res.Role != models.Admin && ownerModelObj.GetUserID().String() != oid.String() {
		// not admin, only remove yourself
		return nil, res.Role, errPermission
	}

	return modelObj, res.Role, nil
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
	ownerModelObj, ok := modelObj.(models.IOwnership)
	if !ok {
		return nil, fmt.Errorf("model not an IOwnership object")
	}

	// Might not need this
	if modelObj.GetID() == nil {
		modelObj.SetID(datatypes.NewUUID())
	}

	// You gotta have admin access to the model in order to create a relation
	err := userHasAdminAccessToOriginalModel(db, oid, typeString, ownerModelObj.GetModelID())
	if err != nil {
		return nil, err
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

	var before func(hpdata models.HookPointData) error
	var after func(hpdata models.HookPointData) error
	if v, ok := modelObj.(models.IBeforeCreate); ok {
		before = v.BeforeInsertDB
	}
	if v, ok := modelObj.(models.IAfterCreate); ok {
		after = v.AfterInsertDB
	}

	j := opJob{
		mapper:     mapper,
		db:         db,
		oid:        oid,
		scope:      scope,
		typeString: typeString,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
	}
	return opCore(before, after, j, createOneCoreBasic)
}

// CreateMany creates an instance of this model based on json and store it in db
func (mapper *LinkTableMapper) CreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	for i, modelObj := range modelObjs {
		ownerModelObj, ok := modelObj.(models.IOwnership)
		if !ok {
			return nil, fmt.Errorf("model not an IOwnership object")
		}

		// Probably not necessary
		if modelObj.GetID() == nil {
			modelObjs[i].SetID(datatypes.NewUUID())
		}

		// You gotta have admin access to the model in order to create a relation
		err := userHasAdminAccessToOriginalModel(db, oid, typeString, ownerModelObj.GetModelID())
		if err != nil {
			return nil, err
		}
	}

	before := models.ModelRegistry[typeString].BeforeInsert
	after := models.ModelRegistry[typeString].AfterInsert
	j := batchOpJob{
		mapper:       mapper,
		db:           db,
		oid:          oid,
		scope:        scope,
		typeString:   typeString,
		oldmodelObjs: nil,
		modelObjs:    modelObjs,
	}
	return batchOpCore(j, before, after, createOneCoreBasic)
}

// GetOneWithID get one model object based on its type and its id string
func (mapper *LinkTableMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	// anyone permission can read as long as you are linked on db
	// If you can  read your role to this model_id (YOUR role in the row, not in the row currently fetching)
	// Then you have the link to this model, you can read it
	modelObj, role, err := loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, nil, id, []models.UserRole{models.UserRoleAny})
	if err != nil {
		return nil, models.Invalid, err
	}

	if m, ok := modelObj.(models.IAfterRead); ok {
		hpdata := models.HookPointData{DB: db, OID: oid, Scope: scope, TypeString: typeString, Role: &role}
		if err := m.AfterReadDB(hpdata); err != nil {
			return nil, 0, err
		}
	}

	return modelObj, role, err
}

// GetAll obtains a slice of models.DomainModel
// options can be string "offset" and "limit", both of type int
// This is very Javascript-esque. I would have liked Python's optional parameter more.
// Alas, no such feature in Go. https://stackoverflow.com/questions/2032149/optional-parameters-in-go
// How does Gorm do the following? Might want to check out its source code.
// Cancel offset condition with -1
//  db.Offset(10).Find(&users1).Offset(-1).Find(&users2)
func (mapper *LinkTableMapper) GetAll(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, options map[URLParam]interface{}) ([]models.IModel, []models.UserRole, *int, error) {
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
func (mapper *LinkTableMapper) UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID) (models.IModel, error) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, modelObj, id, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	var before func(hpdata models.HookPointData) error
	var after func(hpdata models.HookPointData) error
	if v, ok := modelObj.(models.IBeforeUpdate); ok {
		before = v.BeforeUpdateDB
	}
	if v, ok := modelObj.(models.IAfterUpdate); ok {
		after = v.AfterUpdateDB
	}

	j := opJob{
		mapper:      mapper,
		db:          db,
		oid:         oid,
		scope:       scope,
		typeString:  typeString,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,
	}
	return opCore(before, after, j, updateOneCore)
}

// UpdateMany updates multiple models
func (mapper *LinkTableMapper) UpdateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	// load old model data
	ids := make([]*datatypes.UUID, len(modelObjs))
	for i, modelObj := range modelObjs {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		id := modelObj.GetID()
		if id == nil || id.String() == "" {
			return nil, errIDEmpty
		}
		ids[i] = id
	}

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper, db, oid, scope, typeString, ids, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	before := models.ModelRegistry[typeString].BeforeUpdate
	after := models.ModelRegistry[typeString].AfterUpdate
	j := batchOpJob{
		mapper:       mapper,
		db:           db,
		oid:          oid,
		scope:        scope,
		typeString:   typeString,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
	}
	return batchOpCore(j, before, after, updateOneCore)
}

// PatchOneWithID updates model based on this json
func (mapper *LinkTableMapper) PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonPatch []byte, id *datatypes.UUID) (models.IModel, error) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, nil, id, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	// Apply patch operations
	modelObj, err := applyPatchCore(typeString, oldModelObj, jsonPatch)
	if err != nil {
		return nil, err
	}

	var before func(hpdata models.HookPointData) error
	var after func(hpdata models.HookPointData) error
	if v, ok := modelObj.(models.IBeforePatch); ok {
		before = v.BeforePatchDB
	}
	if v, ok := modelObj.(models.IAfterPatch); ok {
		after = v.AfterPatchDB
	}

	j := opJob{
		mapper:      mapper,
		db:          db,
		oid:         oid,
		scope:       scope,
		typeString:  typeString,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,
	}
	return opCore(before, after, j, updateOneCore)
}

// PatchMany updates models based on JSON
func (mapper *LinkTableMapper) PatchMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonIDPatches []models.JSONIDPatch) ([]models.IModel, error) {
	// Load data, patch it, then send it to the hookpoint
	// Load IDs
	ids := make([]*datatypes.UUID, len(jsonIDPatches))
	for i, jsonIDPatch := range jsonIDPatches {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		if jsonIDPatch.ID.String() == "" {
			return nil, errIDEmpty
		}
		ids[i] = jsonIDPatch.ID
	}

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper, db, oid, scope, typeString, ids, []models.UserRole{models.Admin})

	// Now patch it
	modelObjs := make([]models.IModel, len(oldModelObjs))
	for i, modelObj := range oldModelObjs {
		// Apply patch operations
		modelObjs[i], err = applyPatchCore(typeString, modelObj, jsonIDPatches[i].Patch)
		if err != nil {
			return nil, err
		}
	}

	before := models.ModelRegistry[typeString].BeforeUpdate
	after := models.ModelRegistry[typeString].AfterUpdate
	j := batchOpJob{
		mapper:       mapper,
		db:           db,
		oid:          oid,
		scope:        scope,
		typeString:   typeString,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
		// roles:        roles,
	}
	return batchOpCore(j, before, after, updateOneCore)
}

// DeleteOneWithID delete the model
func (mapper *LinkTableMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, error) {
	modelObj, _, err := loadAndCheckErrorBeforeModify(mapper, db, oid, scope, typeString, nil, id, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	var before func(hpdata models.HookPointData) error
	var after func(hpdata models.HookPointData) error
	if v, ok := modelObj.(models.IBeforeDelete); ok {
		before = v.BeforeDeleteDB
	}
	if v, ok := modelObj.(models.IAfterDelete); ok {
		after = v.AfterDeleteDB
	}

	j := opJob{
		mapper:     mapper,
		db:         db,
		oid:        oid,
		scope:      scope,
		typeString: typeString,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
	}
	return opCore(before, after, j, deleteOneCore)
}

// DeleteMany deletes multiple models
func (mapper *LinkTableMapper) DeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	// load old model data
	ids := make([]*datatypes.UUID, len(modelObjs))
	for i, modelObj := range modelObjs {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		id := modelObj.GetID()
		if id == nil || id.String() == "" {
			return nil, errIDEmpty
		}
		ids[i] = id
	}

	modelObjs, _, err := loadManyAndCheckBeforeModify(mapper, db, oid, scope, typeString, ids, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if len(modelObjs) > 0 && modelNeedsRealDelete(modelObjs[0]) {
		db = db.Unscoped() // hookpoint will inherit this though
	}

	before := models.ModelRegistry[typeString].BeforeDelete
	after := models.ModelRegistry[typeString].AfterDelete

	j := batchOpJob{
		mapper:     mapper,
		db:         db,
		oid:        oid,
		scope:      scope,
		typeString: typeString,
		modelObjs:  modelObjs,
		// roles:      roles,
	}
	return batchOpCore(j, before, after, deleteOneCore)
}

// ----------------------------------------------------------------------------------------

// getOneWithIDCore get one model object based on its type and its id string
// since this is organizationMapper, need to make sure it's the same organization
func (mapper *LinkTableMapper) getOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
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

	// The role for this role is determined on the role of the row where the user_id is YOU
	type result struct {
		Role models.UserRole
	}
	res := result{}
	if err := db.Table(rtable).Where("user_id = ? and role = ? and model_id = ?",
		oid, models.Admin, id).Select("role").Scan(&res).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, models.Invalid, errPermission
		}
		return nil, models.Invalid, err // some other error
	}

	return modelObj, res.Role, err
}

func (mapper *LinkTableMapper) getManyWithIDsCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	rtable := models.GetTableNameFromTypeString(typeString)
	subquery := fmt.Sprintf("model_id IN (select model_id from %s where user_id = ?)", rtable)
	db2 := db.Table(rtable).Where(subquery, oid).Where("id in (?)", ids)
	modelObjs, err := models.NewSliceFromDBByTypeString(typeString, db2.Set("gorm:auto_preload", true).Find)
	if err != nil {
		log.Println("calling NewSliceFromDBByTypeString err:", err)
		return nil, nil, err
	}

	// Just in case err didn't work (as in the case with IN clause NOT in the ID field, maybe Gorm bug)
	if len(modelObjs) == 0 {
		return nil, nil, fmt.Errorf("not found")
	}

	if len(modelObjs) != len(ids) {
		return nil, nil, errBatchUpdateOrPatchOneNotFound
	}

	// Role is more complicated. I need to find for each unique model_id, when I have a corresponding
	// link to it, what role is it?
	// So currently I only know how to fetch one by one
	// I probably can do it in one go, need extra time.
	// Probably using the query in the beginnign of the function but by selecting the Role column
	// TODO
	roles := make([]models.UserRole, len(modelObjs))
	for i, modelObj := range modelObjs {
		id := modelObj.GetID()
		_, roles[i], err = userHasPermissionToEdit(mapper, db, oid, scope, typeString, id)
		if err != nil {
			return nil, nil, err
		}
	}

	return modelObjs, roles, nil
}
