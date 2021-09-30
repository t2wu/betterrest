package service

import (
	"errors"
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// OrganizationService handles all the ownership specific db calls
type OrganizationService struct {
	BaseService
}

func (serv *OrganizationService) HookBeforeCreateOne(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel) (models.IModel, error) {
	if modelObj.GetID() == nil {
		modelObj.SetID(datatypes.NewUUID())
	}

	// Make sure oid has admin access to this organization
	hasAdminAccess, err := userHasRolesAccessToModelOrg(db, who, typeString, modelObj, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	} else if !hasAdminAccess {
		return nil, errors.New("user does not have admin access to the organization")
	}

	return modelObj, nil
}

func (serv *OrganizationService) HookBeforeCreateMany(db *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	// Check ownership permission
	for _, modelObj := range modelObjs {
		// Make sure oid has admin access to this organization
		hasAdminAccess, err := userHasRolesAccessToModelOrg(db, who, typeString, modelObj, []models.UserRole{models.UserRoleAdmin})
		if err != nil {
			return nil, err
		} else if !hasAdminAccess {
			return nil, errors.New("user does not have admin access to the organization")
		}
	}
	return modelObjs, nil
}

func (serv *OrganizationService) HookBeforeDeleteOne(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel) (models.IModel, error) {
	return modelObj, nil
}

func (serv *OrganizationService) HookBeforeDeleteMany(db *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return modelObjs, nil
}

// getOneWithIDCore get one model object based on its type and its id string
// since this is organizationMapper, need to make sure it's the same organization
func (serv *OrganizationService) ReadOneCore(db *gorm.DB, who models.Who, typeString string, id *datatypes.UUID) (models.IModel, models.UserRole, error) {
	log.Println("organization ReadOneCore called")
	modelObj := models.NewFromTypeString(typeString)

	db = db.Set("gorm:auto_preload", true)

	// var ok bool
	// var modelObjHavingOrganization models.IHasOrganizationLink
	// // (Maybe organization should be defined in the library)
	// // And it's organizational type has a user which includes
	// if modelObjHavingOrganization, ok = models.NewFromTypeString(typeString).(models.IHasOrganizationLink); !ok {
	// 	return nil, models.UserRoleGuest, fmt.Errorf("Model %s does not comform to IHasOrganizationLink", typeString)
	// }

	// Graphically:
	// Model -- Org -- Join Table -- User
	// orgTableName := models.GetOrganizationTableName(modelObjHavingOrganization)
	// orgTable := reflect.New(modelObjHavingOrganization.OrganizationType()).Interface()
	orgTableName := models.OrgModelNameFromOrgResourceTypeString(typeString)
	// orgTableName := models.GetOrganizationalTableNameFromModelTypeString(typeString)

	joinTableName := orgJoinTableName(typeString)
	// orgTable, _ := reflect.New(models.GetOrganizationTypeFromTypeString(typeString)).Interface().(models.IModel)
	// joinTableName := models.GetJoinTableName(orgTable)

	orgIDFieldName := *models.GetFieldNameFromModelByTagKey(models.NewFromTypeString(typeString), "org")
	orgFieldName := strcase.SnakeCase(orgIDFieldName)
	// orgFieldName := strcase.SnakeCase(modelObjHavingOrganization.GetOrganizationIDFieldName())

	rtable := models.GetTableNameFromIModel(modelObj)

	// e.g. INNER JOIN \"organization\" ON \"dock\".\"OrganizationID\" = \"organization\".id
	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".\"%s\" = \"%s\".id AND \"%s\".\"id\" = ?", orgTableName, rtable, orgFieldName, orgTableName, rtable)
	// e.g. INNER JOIN \"user_owns_organization\" ON \"organization\".id = \"user_owns_organization\".model_id
	secondJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id", joinTableName, orgTableName, joinTableName)
	thirdJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)

	err := db.Table(rtable).Joins(firstJoin, id.String()).Joins(secondJoin).Joins(thirdJoin, who.Oid.String()).Find(modelObj).Error
	if err != nil {
		return nil, 0, err
	}

	joinTable := models.NewOrgOwnershipModelFromOrgResourceTypeString(typeString)
	role := models.UserRoleGuest // just some default

	orgID := models.GetFieldValueFromModelByTagKeyBetterRestAndValueKey(modelObj, "org").(*datatypes.UUID)
	// orgID := modelObj.(models.IHasOrganizationLink).GetOrganizationID().String()

	// Get the roles for the organizations this user has access to
	if err2 := db.Table(joinTableName).Select("model_id, role").Where("user_id = ? AND model_id = ?", who.Oid.String(),
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

func (serv *OrganizationService) GetManyCore(db *gorm.DB, who models.Who, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	// var ok bool
	// var modelObjHavingOrganization models.IHasOrganizationLink
	// if modelObjHavingOrganization, ok = models.NewFromTypeString(typeString).(models.IHasOrganizationLink); !ok {
	// 	return nil, nil, fmt.Errorf("Model %s does not comform to IHasOrganizationLink", typeString)
	// }

	// Graphically:
	// Model -- Org -- Join Table -- User
	rtable := models.GetTableNameFromTypeString(typeString)
	// orgTableName := models.GetOrganizationTableName(modelObjHavingOrganization)
	// orgTable := reflect.New(modelObjHavingOrganization.OrganizationType()).Interface()
	orgTableName := models.OrgModelNameFromOrgResourceTypeString(typeString)
	joinTableName := orgJoinTableName(typeString)

	// orgTableName := models.GetOrganizationalTableNameFromModelTypeString(typeString)
	// orgTable, _ := reflect.New(models.GetOrganizationTypeFromTypeString(typeString)).Interface().(models.IModel)
	// joinTableName := models.GetJoinTableName(orgTable)

	ofield := *models.GetFieldNameFromModelByTagKey(models.NewFromTypeString(typeString), "org")
	orgFieldName := strcase.SnakeCase(ofield)
	// orgFieldName := strcase.SnakeCase(modelObjHavingOrganization.GetOrganizationIDFieldName())

	// e.g. INNER JOIN \"organization\" ON \"dock\".\"OrganizationID\" = \"organization\".id
	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".\"%s\" = \"%s\".id AND \"%s\".\"id\" IN (?)", orgTableName, rtable, orgFieldName, orgTableName, rtable)
	// e.g. INNER JOIN \"user_owns_organization\" ON \"organization\".id = \"user_owns_organization\".model_id
	secondJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id", joinTableName, orgTableName, joinTableName)
	thirdJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)

	db2 := db.Table(rtable).Joins(firstJoin, ids).Joins(secondJoin).Joins(thirdJoin, who.Oid) // .Find(modelObj).Error

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

// GetAllQueryContructCore construct query core
func (serv *OrganizationService) GetAllQueryContructCore(db *gorm.DB, who models.Who, typeString string) (*gorm.DB, error) {
	// Graphically:
	// Model -- Org -- Join Table -- User
	// (Maybe organization should be defined in the library)
	// And it's organizational type has a user which includes
	// modelObjHavingOrganization, ok := models.NewFromTypeString(typeString).(models.IHasOrganizationLink)
	// if !ok {
	// 	return nil, fmt.Errorf("Model %s does not comform to IHasOrganizationLink", typeString)
	// }

	rtable := models.GetTableNameFromTypeString(typeString)
	// orgTableName := models.GetOrganizationTableName(modelObjHavingOrganization)
	orgTableName := models.OrgModelNameFromOrgResourceTypeString(typeString)

	joinTableName := orgJoinTableName(typeString)

	// This is the go to class for join. So if they use this it's a different
	// join table name from main resource name (org table)
	if joinTableName == "ownership_model_with_id_base" {
		joinTableName = "user_owns_" + orgTableName
	}

	// orgTableName := models.GetOrganizationalTableNameFromModelTypeString(typeString)
	// // orgTable := reflect.New(modelObjHavingOrganization.OrganizationType()).Interface()
	// orgTable := reflect.New(models.GetOrganizationTypeFromTypeString(typeString)).Interface().(models.IModel)
	// joinTableName := models.GetJoinTableName(orgTable)

	ofield := *models.GetFieldNameFromModelByTagKey(models.NewFromTypeString(typeString), "org")
	orgFieldName := strcase.SnakeCase(ofield)

	// e.g. INNER JOIN \"organization\" ON \"dock\".\"OrganizationID\" = \"organization\".id
	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".\"%s\" = \"%s\".id", orgTableName, rtable, orgFieldName, orgTableName)
	// e.g. INNER JOIN \"user_owns_organization\" ON \"organization\".id = \"user_owns_organization\".model_id
	secondJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id", joinTableName, orgTableName, joinTableName)
	thirdJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)
	db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin).Joins(thirdJoin, who.Oid.String())
	return db, nil
}

// GetAllRolesCore gets all roles according to the criteria
func (serv *OrganizationService) GetAllRolesCore(dbChained *gorm.DB, dbClean *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel) ([]models.UserRole, error) {
	// modelObjHavingOrganization, _ := models.NewFromTypeString(typeString).(models.IHasOrganizationLink)
	// orgTable := reflect.New(modelObjHavingOrganization.OrganizationType()).Interface()
	// orgTable := reflect.New(models.GetOrganizationTypeFromTypeString(typeString)).Interface().(models.IModel)
	// joinTableName := models.GetJoinTableName(orgTable)
	joinTableName := orgJoinTableName(typeString)

	rows, err := db.Shared().Table(joinTableName).Select("model_id, role").Where("user_id = ?", who.Oid.String()).Rows()
	// that's weird, Gorm says [0 rows affected or returned ] but in fact it did return something.
	if err != nil {
		return nil, err
	}

	thisRole := models.UserRoleGuest      // just some default
	organizationID := datatypes.NewUUID() // just some default
	orgIDToRoleMap := make(map[string]models.UserRole)
	for rows.Next() {
		if err = rows.Scan(organizationID, &thisRole); err != nil {
			return nil, err
		}

		orgIDToRoleMap[organizationID.String()] = thisRole
	}

	roles := make([]models.UserRole, 0)
	for _, outmodel := range modelObjs {
		// o := outmodel.(models.IHasOrganizationLink)

		orgID := models.GetFieldValueFromModelByTagKeyBetterRestAndValueKey(outmodel, "org").(*datatypes.UUID).String()
		// orgID := o.GetOrganizationID().String()

		role := orgIDToRoleMap[orgID]
		roles = append(roles, role)
	}

	return roles, nil
}

// ---------------------------------------
// The model object should have link to the ownership object which has a linking table to the user
func userHasRolesAccessToModelOrg(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel, roles []models.UserRole) (bool, error) {
	organizationTableName := models.OrgModelNameFromOrgResourceTypeString(typeString)
	organizationJoinTableName := orgJoinTableName(typeString)

	organizationID := models.GetFieldValueFromModelByTagKeyBetterRestAndValueKey(modelObj, "org").(*datatypes.UUID)

	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id AND \"%s\".role IN (?)", organizationJoinTableName, organizationTableName, organizationJoinTableName,
		organizationJoinTableName)
	secondJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", organizationJoinTableName, organizationJoinTableName)
	whereStmt := fmt.Sprintf("\"%s\".model_id = ?", organizationJoinTableName)
	db = db.Table(organizationTableName).Joins(firstJoin, roles).Joins(secondJoin, who.Oid.String()).Where(whereStmt, organizationID)

	organizations, err := models.NewSliceFromDBByType(models.OrgModelTypeFromOrgResourceTypeString(typeString), db.Find)
	if err != nil {
		return false, err
	}

	if len(organizations) != 1 {
		return false, fmt.Errorf("search should have exactly one organization")
	}

	// Check that organizations is not an empty array, and one of the organization has an ID that
	// is specified in
	org := organizations[0]
	if organizationID == nil {
		return false, errors.New("missing organization link ID")

	}

	if org.GetID().String() == organizationID.String() {
		return true, nil
	}

	return false, nil
}

// UpdateOneCore one, permission should already be checked
// called for patch operation as well (after patch has already applied)
// Fuck, repeat the following code for now (you can't call the overriding method from the non-overriding one)
func (serv *OrganizationService) UpdateOneCore(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (modelObj2 models.IModel, err error) {
	if modelNeedsRealDelete(oldModelObj) { // parent model
		db = db.Unscoped()
	}

	err = gormfixes.UpdatePeggedFields(db, oldModelObj, modelObj)
	if err != nil {
		return nil, err
	}

	if err = db.Save(modelObj).Error; err != nil { // save updates all fields (FIXME: need to check for required)
		log.Println("Error updating:", err)
		return nil, err
	}

	// This loads the IDs
	// This so we have the preloading.
	modelObj2, _, err = serv.ReadOneCore(db, who, typeString, id)
	if err != nil { // Error is "record not found" when not found
		log.Println("Error:", err)
		return nil, err
	}

	// ouch! for many to many we need to remove it again!!
	// because it's in a transaction so it will load up again
	gormfixes.FixManyToMany(modelObj, modelObj2)

	return modelObj2, nil
}

// ----------------------------------------

func orgJoinTableName(typeString string) string {
	joinTableName := models.OrgOwnershipModelNameFromOrgResourceTypeString(typeString)

	// This is the go to class for join. So if they use this it's a different
	// join table name from main resource name (org table)
	if joinTableName == "ownership_model_with_id_base" {
		orgTableName := models.OrgModelNameFromOrgResourceTypeString(typeString)
		joinTableName = "user_owns_" + orgTableName
	}

	return joinTableName
}
