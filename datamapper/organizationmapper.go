package datamapper

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"

	"github.com/jinzhu/gorm"
)

// ---------------------------------------

func userHasRolesAccessToModelOrg(db *gorm.DB, oid *datatypes.UUID, typeString string, modelObj models.IModel, roles []models.UserRole) (bool, error) {
	var modelObjHavingOrganization models.IHasOrganizationLink
	var modelObjHavingOwnership models.IHasOwnershipLink
	var ok bool

	// Create one model (dock),
	// Make sure oid has admin access to the organization it refers to

	// (Maybe organization should be defined in the library)
	// And it's organizational type has a user which includes

	if modelObjHavingOrganization, ok = modelObj.(models.IHasOrganizationLink); !ok {
		return false, fmt.Errorf("Model %s does not comform to IHasOrganizationLink", typeString)
	}

	organizationTableName := models.GetOrganizationTableName(modelObjHavingOrganization)
	// organization := reflect.New(modelObj.OwnershipType()).Interface().(models.IModel)

	if modelObjHavingOwnership, ok = reflect.New(modelObjHavingOrganization.OrganizationType()).Interface().(models.IHasOwnershipLink); !ok {
		return false, fmt.Errorf("Model %s's organization table does not comform to IHasOwnershipLink", typeString)
	}

	// Get organization's join table name
	organizationJoinTableName := models.GetJoinTableName(modelObjHavingOwnership)

	rolesQuery := strconv.Itoa(int(roles[0]))
	for i := 1; i < len(roles); i++ {
		rolesQuery += "," + strconv.Itoa(int(roles[i]))
	}

	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id AND \"%s\".role IN (%s)", organizationJoinTableName, organizationTableName, organizationJoinTableName,
		organizationJoinTableName, rolesQuery)
	secondJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", organizationJoinTableName, organizationJoinTableName)
	db = db.Table(organizationTableName).Joins(firstJoin).Joins(secondJoin, oid.String())

	organizations, err := models.NewSliceFromDBByType(modelObjHavingOrganization.OrganizationType(), db.Find)
	if err != nil {
		return false, err
	}

	// Check that organizations is not an empty array, and one of the organization has an ID that
	// is specified in
	organizationID := modelObjHavingOrganization.GetOrganizationID()
	for _, org := range organizations {
		if organizationID == nil {
			return false, errors.New("missing organization link ID")

		}
		if org.GetID().String() == organizationID.String() {
			return true, nil
		}
	}

	return false, nil
}

// ---------------------------------------

var onceOrganizationMapper sync.Once
var organizationMapper *OrganizationMapper

// OrganizationMapper is a basic CRUD manager
type OrganizationMapper struct {
}

// SharedOrganizationMapper creats a singleton of Crud object
func SharedOrganizationMapper() *OrganizationMapper {
	onceOrganizationMapper.Do(func() {
		organizationMapper = &OrganizationMapper{}
	})

	return organizationMapper
}

// CreateOne creates an instance of this model based on json and store it in db
// when creating, need to put yourself in OrganizationUser as well.
// Well check this!!
func (mapper *OrganizationMapper) CreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	if modelObj.GetID() == nil {
		modelObj.SetID(datatypes.NewUUID())
	}

	// Make sure oid has admin access to this organization
	hasAdminAccess, err := userHasRolesAccessToModelOrg(db, oid, typeString, modelObj, []models.UserRole{models.Admin})
	if err != nil {
		return nil, err
	} else if !hasAdminAccess {
		return nil, errors.New("user does not have admin access to the organization")
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
func (mapper *OrganizationMapper) CreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	// Check ownership permission
	for _, modelObj := range modelObjs {
		if modelObj.GetID() == nil {
			modelObj.SetID(datatypes.NewUUID())
		}

		// Make sure oid has admin access to this organization
		hasAdminAccess, err := userHasRolesAccessToModelOrg(db, oid, typeString, modelObj, []models.UserRole{models.Admin})
		if err != nil {
			return nil, err
		} else if !hasAdminAccess {
			return nil, errors.New("user does not have admin access to the organization")
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
func (mapper *OrganizationMapper) GetOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
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
func (mapper *OrganizationMapper) GetAll(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, options map[URLParam]interface{}) ([]models.IModel, []models.UserRole, *int, error) {
	db2 := db
	db = db.Set("gorm:auto_preload", true)

	offset, limit, cstart, cstop, order, latestn, totalcount := getOptions(options)

	var ok bool
	var modelObjHavingOrganization models.IHasOrganizationLink
	// (Maybe organization should be defined in the library)
	// And it's organizational type has a user which includes
	if modelObjHavingOrganization, ok = models.NewFromTypeString(typeString).(models.IHasOrganizationLink); !ok {
		return nil, nil, nil, fmt.Errorf("Model %s does not comform to IHasOrganizationLink", typeString)
	}

	// Graphically:
	// Model -- Org -- Join Table -- User
	orgTableName := models.GetOrganizationTableName(modelObjHavingOrganization)
	orgTable := reflect.New(modelObjHavingOrganization.OrganizationType()).Interface()
	joinTableName := models.GetJoinTableName(orgTable.(models.IHasOwnershipLink))
	orgFieldName := strcase.SnakeCase(modelObjHavingOrganization.GetOrganizationIDFieldName())

	rtable := models.GetTableNameFromTypeString(typeString)

	// e.g. INNER JOIN \"organization\" ON \"dock\".\"OrganizationID\" = \"organization\".id
	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".\"%s\" = \"%s\".id", orgTableName, rtable, orgFieldName, orgTableName)
	// e.g. INNER JOIN \"user_owns_organization\" ON \"organization\".id = \"user_owns_organization\".model_id
	secondJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id", joinTableName, orgTableName, joinTableName)
	thirdJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)

	if cstart != nil && cstop != nil {
		db = db.Where(rtable+".created_at BETWEEN ? AND ?", time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))
	}

	var err error

	db, err = constructInnerFieldParamQueries(db, typeString, options, latestn)
	if err != nil {
		return nil, nil, nil, err
	}

	db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin).Joins(thirdJoin, oid.String())

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

	// Now we need to fill in roles for each model. With regular ownershipmapper, the link table
	// itself has role values and we need to query that
	// But with OrganizationMapper, the role the user has to the organization is the role the user
	// has to the model. We do the following:
	// 1. We query all the organization this user belongs to, get the roles
	// 2. With the models we have, we find the organization id, and match it to the organization in step 1,
	// then fetch the role, which becomes the role for the model

	// Get the roles for the organizations this user has access to
	// stmt := fmt.Sprintf("SELECT model_id, role FROM %s WHERE user_id = ?", joinTableName)
	rows, err := db2.Table(joinTableName).Select("model_id, role").Where("user_id = ?", oid.String()).Rows()
	if err != nil {
		return nil, nil, nil, err
	}

	thisRole := models.Guest              // just some default
	organizationID := datatypes.NewUUID() // just some default
	orgIDToRoleMap := make(map[string]models.UserRole)
	for rows.Next() {
		if err = rows.Scan(organizationID, &thisRole); err != nil {
			return nil, nil, nil, err
		}

		orgIDToRoleMap[organizationID.String()] = thisRole
	}

	roles := make([]models.UserRole, 0)
	for _, outmodel := range outmodels {
		o := outmodel.(models.IHasOrganizationLink)
		orgID := o.GetOrganizationID().String()

		role := orgIDToRoleMap[orgID]
		roles = append(roles, role)
	}

	// safeguard, Must be coded wrongly
	if len(outmodels) != len(roles) {
		return nil, nil, nil, errors.New("unknown query error")
	}

	// make many to many tag works
	for _, m := range outmodels {
		err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db2, m)
		if err != nil {
			return nil, nil, nil, err
		}
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
func (mapper *OrganizationMapper) UpdateOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID) (models.IModel, error) {
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
func (mapper *OrganizationMapper) UpdateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
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
func (mapper *OrganizationMapper) PatchOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonPatch []byte, id *datatypes.UUID) (models.IModel, error) {
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
func (mapper *OrganizationMapper) PatchMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, jsonIDPatches []models.JSONIDPatch) ([]models.IModel, error) {
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
	for i, jsonIDPatch := range jsonIDPatches {
		// Apply patch operations
		modelObjs[i], err = applyPatchCore(typeString, oldModelObjs[i], []byte(jsonIDPatch.Patch))
		if err != nil {
			log.Println("patch error: ", err, string(jsonIDPatch.Patch))
			return nil, err
		}
	}

	before := models.ModelRegistry[typeString].BeforePatch
	after := models.ModelRegistry[typeString].AfterPatch
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

// DeleteOneWithID delete the model
// TODO: delete the groups associated with this record?
func (mapper *OrganizationMapper) DeleteOneWithID(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, error) {
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
func (mapper *OrganizationMapper) DeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
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
		db = db.Unscoped() // hookpoint wil inherit this though
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
	}
	return batchOpCore(j, before, after, deleteOneCore)
}

// ----------------------------------------------------------------------------------------

// getOneWithIDCore get one model object based on its type and its id string
// since this is organizationMapper, need to make sure it's the same organization
func (mapper *OrganizationMapper) getOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	modelObj := models.NewFromTypeString(typeString)

	db = db.Set("gorm:auto_preload", true)

	var ok bool
	var modelObjHavingOrganization models.IHasOrganizationLink
	// (Maybe organization should be defined in the library)
	// And it's organizational type has a user which includes
	if modelObjHavingOrganization, ok = models.NewFromTypeString(typeString).(models.IHasOrganizationLink); !ok {
		return nil, models.Guest, fmt.Errorf("Model %s does not comform to IHasOrganizationLink", typeString)
	}

	// Graphically:
	// Model -- Org -- Join Table -- User
	orgTableName := models.GetOrganizationTableName(modelObjHavingOrganization)
	orgTable := reflect.New(modelObjHavingOrganization.OrganizationType()).Interface()
	joinTableName := models.GetJoinTableName(orgTable.(models.IHasOwnershipLink))
	orgFieldName := strcase.SnakeCase(modelObjHavingOrganization.GetOrganizationIDFieldName())

	rtable := models.GetTableNameFromIModel(modelObj)

	// e.g. INNER JOIN \"organization\" ON \"dock\".\"OrganizationID\" = \"organization\".id
	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".\"%s\" = \"%s\".id AND \"%s\".\"id\" = ?", orgTableName, rtable, orgFieldName, orgTableName, rtable)
	// e.g. INNER JOIN \"user_owns_organization\" ON \"organization\".id = \"user_owns_organization\".model_id
	secondJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id", joinTableName, orgTableName, joinTableName)
	thirdJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)

	err := db.Table(rtable).Joins(firstJoin, id.String()).Joins(secondJoin).Joins(thirdJoin, oid.String()).Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	joinTable := reflect.New(orgTable.(models.IHasOwnershipLink).OwnershipType()).Interface()
	role := models.Guest // just some default

	orgID := modelObj.(models.IHasOrganizationLink).GetOrganizationID().String()

	// Get the roles for the organizations this user has access to
	if err2 := db.Table(joinTableName).Select("model_id, role").Where("user_id = ? AND model_id = ?", oid.String(),
		orgID).Scan(joinTable).Error; err2 != nil {
		return nil, 0, err2
	}

	if m, ok := joinTable.(models.IOwnership); ok {
		role = m.GetRole()
	}

	err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db, modelObj)
	if err != nil {
		return nil, 0, err
	}

	return modelObj, role, err
}

func (mapper *OrganizationMapper) getManyWithIDsCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	var ok bool
	var modelObjHavingOrganization models.IHasOrganizationLink
	if modelObjHavingOrganization, ok = models.NewFromTypeString(typeString).(models.IHasOrganizationLink); !ok {
		return nil, nil, fmt.Errorf("Model %s does not comform to IHasOrganizationLink", typeString)
	}

	// Graphically:
	// Model -- Org -- Join Table -- User
	rtable := models.GetTableNameFromTypeString(typeString)
	orgTableName := models.GetOrganizationTableName(modelObjHavingOrganization)
	orgTable := reflect.New(modelObjHavingOrganization.OrganizationType()).Interface()
	joinTableName := models.GetJoinTableName(orgTable.(models.IHasOwnershipLink))
	orgFieldName := strcase.SnakeCase(modelObjHavingOrganization.GetOrganizationIDFieldName())

	// e.g. INNER JOIN \"organization\" ON \"dock\".\"OrganizationID\" = \"organization\".id
	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".\"%s\" = \"%s\".id AND \"%s\".\"id\" IN (?)", orgTableName, rtable, orgFieldName, orgTableName, rtable)
	// e.g. INNER JOIN \"user_owns_organization\" ON \"organization\".id = \"user_owns_organization\".model_id
	secondJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id", joinTableName, orgTableName, joinTableName)
	thirdJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)

	db2 := db.Table(rtable).Joins(firstJoin, ids).Joins(secondJoin).Joins(thirdJoin, oid) // .Find(modelObj).Error

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

	// Check error
	// Load the roles and check if they're admin
	roles := make([]models.UserRole, 0)
	if err := db2.Select(fmt.Sprintf("\"%s\".\"role\"", joinTableName)).Scan(&roles).Error; err != nil {
		log.Printf("err getting roles")
		return nil, nil, err
	}

	return modelObjs, roles, nil
}
