package service

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"strconv"

	"github.com/jinzhu/gorm"
	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// OrganizationService handles all the ownership specific db calls
type OrganizationService struct {
}

func (serv *OrganizationService) HookBeforeCreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
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

	return modelObj, nil
}

func (serv *OrganizationService) HookBeforeCreateMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
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
	return modelObjs, nil
}

func (serv *OrganizationService) HookBeforeDeleteOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	return modelObj, nil
}

func (serv *OrganizationService) HookBeforeDeleteMany(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return modelObjs, nil
}

// getOneWithIDCore get one model object based on its type and its id string
// since this is organizationMapper, need to make sure it's the same organization
func (serv *OrganizationService) GetOneWithIDCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
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

func (serv *OrganizationService) GetManyWithIDsCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
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
		return nil, nil, ErrBatchUpdateOrPatchOneNotFound
	}

	// Check error
	// Load the roles and check if they're admin
	roles := make([]models.UserRole, 0)
	if err := db2.Select(fmt.Sprintf("\"%s\".\"role\"", joinTableName)).Scan(&roles).Error; err != nil {
		log.Printf("err getting roles")
		return nil, nil, err
	}

	for _, modelObj := range modelObjs {
		err = gormfixes.LoadManyToManyBecauseGormFailsWithID(db, modelObj)
		if err != nil {
			return nil, nil, err
		}
	}

	return modelObjs, roles, nil
}

func (serv *OrganizationService) GetAllCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string) ([]models.IModel, []models.UserRole, error) {
	// Graphically:
	// Model -- Org -- Join Table -- User
	// (Maybe organization should be defined in the library)
	// And it's organizational type has a user which includes
	modelObjHavingOrganization, ok := models.NewFromTypeString(typeString).(models.IHasOrganizationLink)
	if !ok {
		return nil, nil, fmt.Errorf("Model %s does not comform to IHasOrganizationLink", typeString)
	}

	rtable := models.GetTableNameFromTypeString(typeString)
	orgTableName := models.GetOrganizationTableName(modelObjHavingOrganization)
	orgTable := reflect.New(modelObjHavingOrganization.OrganizationType()).Interface()
	joinTableName := models.GetJoinTableName(orgTable.(models.IHasOwnershipLink))
	orgFieldName := strcase.SnakeCase(modelObjHavingOrganization.GetOrganizationIDFieldName())

	// e.g. INNER JOIN \"organization\" ON \"dock\".\"OrganizationID\" = \"organization\".id
	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".\"%s\" = \"%s\".id", orgTableName, rtable, orgFieldName, orgTableName)
	// e.g. INNER JOIN \"user_owns_organization\" ON \"organization\".id = \"user_owns_organization\".model_id
	secondJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id", joinTableName, orgTableName, joinTableName)
	thirdJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)
	db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin).Joins(thirdJoin, oid.String())

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
	rows, err := db.Table(joinTableName).Select("model_id, role").Where("user_id = ?", oid.String()).Rows()
	if err != nil {
		return nil, nil, err
	}

	thisRole := models.Guest              // just some default
	organizationID := datatypes.NewUUID() // just some default
	orgIDToRoleMap := make(map[string]models.UserRole)
	for rows.Next() {
		if err = rows.Scan(organizationID, &thisRole); err != nil {
			return nil, nil, err
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

	return outmodels, roles, nil
}

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
