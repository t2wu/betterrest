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
	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// OrganizationService handles all the ownership specific db calls
type OrganizationService struct {
	BaseService
}

func (serv *OrganizationService) HookBeforeCreateOne(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel) (models.IModel, error) {
	if modelObj.GetID() == nil {
		modelObj.SetID(datatypes.NewUUID())
	}

	// Make sure oid has admin access to this organization
	hasAdminAccess, err := userHasRolesAccessToModelOrg(db, oid, typeString, modelObj, []models.UserRole{models.UserRoleAdmin})
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
		hasAdminAccess, err := userHasRolesAccessToModelOrg(db, oid, typeString, modelObj, []models.UserRole{models.UserRoleAdmin})
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
	log.Println("organization GetOneWithIDCore called")
	modelObj := models.NewFromTypeString(typeString)

	db = db.Set("gorm:auto_preload", true)

	var ok bool
	var modelObjHavingOrganization models.IHasOrganizationLink
	// (Maybe organization should be defined in the library)
	// And it's organizational type has a user which includes
	if modelObjHavingOrganization, ok = models.NewFromTypeString(typeString).(models.IHasOrganizationLink); !ok {
		return nil, models.UserRoleGuest, fmt.Errorf("Model %s does not comform to IHasOrganizationLink", typeString)
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
	role := models.UserRoleGuest // just some default

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

// GetAllQueryContructCore construct query core
func (serv *OrganizationService) GetAllQueryContructCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string) (*gorm.DB, error) {
	// Graphically:
	// Model -- Org -- Join Table -- User
	// (Maybe organization should be defined in the library)
	// And it's organizational type has a user which includes
	modelObjHavingOrganization, ok := models.NewFromTypeString(typeString).(models.IHasOrganizationLink)
	if !ok {
		return nil, fmt.Errorf("Model %s does not comform to IHasOrganizationLink", typeString)
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
	return db, nil
}

// GetAllRolesCore gets all roles according to the criteria
func (serv *OrganizationService) GetAllRolesCore(dbChained *gorm.DB, dbClean *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObjs []models.IModel) ([]models.UserRole, error) {
	modelObjHavingOrganization, _ := models.NewFromTypeString(typeString).(models.IHasOrganizationLink)
	orgTable := reflect.New(modelObjHavingOrganization.OrganizationType()).Interface()
	joinTableName := models.GetJoinTableName(orgTable.(models.IHasOwnershipLink))

	rows, err := db.Shared().Table(joinTableName).Select("model_id, role").Where("user_id = ?", oid.String()).Rows()
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
		o := outmodel.(models.IHasOrganizationLink)
		orgID := o.GetOrganizationID().String()

		role := orgIDToRoleMap[orgID]
		roles = append(roles, role)
	}

	return roles, nil
}

// ---------------------------------------
// The model object should have link to the ownership object which has a linking table to the user
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
	organizationJoinTableName := models.GetJoinTableName(modelObjHavingOwnership) // join table (link table)

	rolesQuery := strconv.Itoa(int(roles[0]))
	for i := 1; i < len(roles); i++ {
		rolesQuery += "," + strconv.Itoa(int(roles[i]))
	}

	organizationID := modelObjHavingOrganization.GetOrganizationID()
	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id AND \"%s\".role IN (%s)", organizationJoinTableName, organizationTableName, organizationJoinTableName,
		organizationJoinTableName, rolesQuery)
	secondJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", organizationJoinTableName, organizationJoinTableName)
	whereStmt := fmt.Sprintf("\"%s\".model_id = ?", organizationJoinTableName)
	db = db.Table(organizationTableName).Joins(firstJoin).Joins(secondJoin, oid.String()).Where(whereStmt, organizationID)

	organizations, err := models.NewSliceFromDBByType(modelObjHavingOrganization.OrganizationType(), db.Find)
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

// UpdateOneCore one, permissin should already be checked
// called for patch operation as well (after patch has already applied)
// Fuck, repeat the following code for now (you can't call the overriding method from the non-overriding one)
func (serv *OrganizationService) UpdateOneCore(db *gorm.DB, oid *datatypes.UUID, scope *string, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (modelObj2 models.IModel, err error) {
	if modelNeedsRealDelete(oldModelObj) { // parent model
		db = db.Unscoped()
	}

	err = gormfixes.UpdatePeggedFields(db, oldModelObj, modelObj)
	if err != nil {
		return nil, err
	}

	// For some unknown reason
	// insert many-to-many works cuz Gorm does and works???
	// [2020-05-22 18:50:17]  [1.63ms]  INSERT INTO \"dock_group\" (\"group_id\",\"dock_id\") SELECT '<binary>','<binary>' FROM DUAL WHERE NOT EXISTS (SELECT * FROM \"dock_group\" WHERE \"group_id\" = '<binary>' AND \"dock_id\" = '<binary>')
	// [0 rows affected or returned ]

	// (/Users/t2wu/Documents/Go/pkg/mod/github.com/t2wu/betterrest@v0.1.19/datamapper/modulelibs.go:62)
	// [2020-05-22 18:50:17]  [1.30ms]  UPDATE \"dock\" SET \"updated_at\" = '2020-05-22 18:50:17', \"deleted_at\" = NULL, \"name\" = '', \"model\" = '', \"serial_no\" = '', \"mac\" = '', \"hub_id\" = NULL, \"is_online\" = false, \"room_id\" = NULL  WHERE \"dock\".\"deleted_at\" IS NULL AND \"dock\".\"id\" = '{2920e86e-33b1-4848-a773-e68e5bde4fc0}'
	// [1 rows affected or returned ]

	// (/Users/t2wu/Documents/Go/pkg/mod/github.com/t2wu/betterrest@v0.1.19/datamapper/modulelibs.go:62)
	// [2020-05-22 18:50:17]  [2.84ms]  INSERT INTO \"dock_group\" (\"dock_id\",\"group_id\") SELECT ') �n3�HH�s�[�O�','<binary>' FROM DUAL WHERE NOT EXISTS (SELECT * FROM \"dock_group\" WHERE \"dock_id\" = ') �n3�HH�s�[�O�' AND \"group_id\" = '<binary>')
	// [1 rows affected or returned ]
	if err = db.Save(modelObj).Error; err != nil { // save updates all fields (FIXME: need to check for required)
		log.Println("Error updating:", err)
		return nil, err
	}

	// This loads the IDs
	// This so we have the preloading.
	modelObj2, _, err = serv.GetOneWithIDCore(db, oid, scope, typeString, id)
	if err != nil { // Error is "record not found" when not found
		log.Println("Error:", err)
		return nil, err
	}

	// ouch! for many to many we need to remove it again!!
	// because it's in a transaction so it will load up again
	gormfixes.FixManyToMany(modelObj, modelObj2)

	return modelObj2, nil
}
