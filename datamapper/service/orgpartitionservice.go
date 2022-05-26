package service

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/stoewer/go-strcase"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry"
)

// OrgPartition handles all the ownership specific db calls
type OrgPartition struct {
	BaseServiceV2
}

func (serv *OrgPartition) HookBeforeCreateOne(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel) (models.IModel, error) {
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

func (serv *OrgPartition) HookBeforeCreateMany(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
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

func (serv *OrgPartition) HookBeforeDeleteOne(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel) (models.IModel, error) {
	return modelObj, nil
}

func (serv *OrgPartition) HookBeforeDeleteMany(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObjs []models.IModel) ([]models.IModel, error) {
	return modelObjs, nil
}

// getOneWithIDCore get one model object based on its type and its id string
// since this is organizationMapper, need to make sure it's the same organization
func (serv *OrgPartition) ReadOneCore(db *gorm.DB, who models.UserIDFetchable, typeString string, id *datatypes.UUID, options map[urlparam.Param]interface{}) (models.IModel, models.UserRole, error) {
	modelObj := registry.NewFromTypeString(typeString)

	db2 := db

	// Graphically:
	// Model -- Org -- Join Table -- User
	orgTableName := registry.OrgModelNameFromOrgResourceTypeString(typeString)

	joinTableName := orgJoinTableName(typeString)
	orgIDFieldName := *models.GetFieldNameFromModelByTagKey(registry.NewFromTypeString(typeString), "org")
	orgFieldName := strcase.SnakeCase(orgIDFieldName)
	rtable := models.GetTableNameFromIModel(modelObj)

	// e.g. INNER JOIN \"organization\" ON \"dock\".\"OrganizationID\" = \"organization\".id
	firstJoin := fmt.Sprintf(`INNER JOIN "%s" ON "%s"."%s" = "%s".id AND "%s"."id" = ?`, orgTableName, rtable, orgFieldName, orgTableName, rtable)
	// e.g. INNER JOIN \"user_owns_organization\" ON \"organization\".id = \"user_owns_organization\".model_id AND "user_owns_organization".user_id = ?
	secondJoin := fmt.Sprintf(`INNER JOIN "%s" ON "%s".id = "%s".model_id AND "%s".user_id = ?`, joinTableName, orgTableName, joinTableName, joinTableName)

	// For Org partition date is required
	_, _, cstart, cstop, _, _, _, _ := urlparam.GetOptions(options)
	if cstart == nil || cstop == nil {
		return nil, models.UserRoleInvalid, fmt.Errorf("GET /%s needs cstart and cstop parameters", strings.ToLower(typeString))
	}
	db = db.Where(rtable+".created_at BETWEEN ? AND ?", time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))
	err := db.Table(rtable).Joins(firstJoin, id.String()).Joins(secondJoin, who.GetUserID().String()).Take(modelObj).Error
	if err != nil {
		return nil, models.UserRoleInvalid, err
	}

	// Now need to find query for all subtables within this table...with date all the way down

	// Now we have to traverse this, get all the pegged fields, and get date as well
	err = RecursivelyQueryAllPeggedModels(db2, []models.IModel{modelObj}, time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))
	if err != nil {
		return nil, models.UserRoleInvalid, err
	}

	role, err := serv.getOrgRole(db2, who, typeString, modelObj, joinTableName)

	// Partition table has to be all partition within
	// Let's not worry about many-to-many table here with partitioned table, they needs to be all "pegged", all the way down

	return modelObj, role, err
}

// func (serv *OrgPartition) recursivelyQueryAllPeggedModels(db *gorm.DB, modelObj models.IModel, begin time.Time, end time.Time) error {
// 	// result := models.PeggedIDSearch{
// 	// 	Peg:      make(map[string]models.ModelAndIds),
// 	// 	PegAssoc: make(map[string]models.ModelAndIds),
// 	// }
// 	// models.FindAllBetterRestPeggOrPegAssocIDs(modelObj, &result)

// 	rtable := models.GetTableNameFromIModel(modelObj)
// 	fieldNameAndTypeArr := models.GetPeggedFieldNumAndType(modelObj)
// 	for _, data := range fieldNameAndTypeArr {

// 		innerTableName := models.GetTableNameFromType(data.ObjType)
// 		db3 := db.Where(innerTableName+".created_at BETWEEN ? AND ?", begin, end)

// 		if data.IsSlice {
// 			slice := reflect.New(reflect.SliceOf(data.ObjType)).Interface()
// 			if err := db3.Where(fmt.Sprintf("%s_id = ?", rtable), modelObj.GetID()).Find(slice).Error; err != nil {
// 				return err
// 			}

// 			models.SetSliceAtFieldNum(modelObj, data.FieldNum, slice)
// 			// Loop the slice, But then I would need to query recursively (not very efficient!! =o=) // TODO
// 		} else {
// 			m := reflect.New(data.ObjType).Interface()
// 			if err := db3.Where(fmt.Sprintf("%s_id = ?", rtable), modelObj.GetID()).Take(m).Error; err != nil {
// 				return err
// 			}

// 			if data.IsStruct {
// 				models.SetStructAtFieldNum(modelObj, data.FieldNum, m)
// 			} else if data.IsPtr {
// 				models.SetStructPtrAtFieldNum(modelObj, data.FieldNum, m)
// 			}

// 			// tagVal := v.Type().Field(i).Tag.Get("betterrest")
// 			// gotag.TagValueHasPrefix(tagVal, "peg")
// 			// need to check if it's pegged field, if it is, may need to traverse further
// 			tagVal := reflect.TypeOf(modelObj).Field(data.FieldNum).Tag.Get("betterrest")
// 			if gotag.TagValueHasPrefix(tagVal, "peg") {
// 				nextObj := reflect.ValueOf(modelObj).Field(data.FieldNum)
// 				if data.IsPtr {
// 					nextObj = nextObj.Elem()
// 				}
// 				err := serv.recursivelyQueryAllPeggedModels(db, nextObj.Interface().(models.IModel), begin, end)
// 				if err != nil {
// 					return err
// 				}
// 			}
// 		}
// 	}
// 	return nil
// }

// If gorm like xorm could select from multiple models we would not need this
func (serv *OrgPartition) getOrgRole(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel, joinTableName string) (models.UserRole, error) {
	joinTable := registry.NewOrgOwnershipModelFromOrgResourceTypeString(typeString)
	role := models.UserRoleGuest // just some default

	orgID := models.GetFieldValueFromModelByTagKeyBetterRestAndValueKey(modelObj, "org").(*datatypes.UUID)
	// orgID := modelObj.(models.IHasOrganizationLink).GetOrganizationID().String()

	// Get the roles for the organizations this user has access to
	if err2 := db.Table(joinTableName).Select("model_id, role").Where("user_id = ? AND model_id = ?", who.GetUserID().String(),
		orgID).Scan(joinTable).Error; err2 != nil {
		return role, err2
	}

	if m, ok := joinTable.(models.IOwnership); ok {
		role = m.GetRole()
	}

	return role, nil
}

func (serv *OrgPartition) GetManyCore(db *gorm.DB, who models.UserIDFetchable, typeString string, ids []*datatypes.UUID) ([]models.IModel, []models.UserRole, error) {
	// var ok bool
	// var modelObjHavingOrganization models.IHasOrganizationLink
	// if modelObjHavingOrganization, ok = registry.NewFromTypeString(typeString).(models.IHasOrganizationLink); !ok {
	// 	return nil, nil, fmt.Errorf("Model %s does not comform to IHasOrganizationLink", typeString)
	// }

	// Graphically:
	// Model -- Org -- Join Table -- User
	rtable := registry.GetTableNameFromTypeString(typeString)
	// orgTableName := models.GetOrganizationTableName(modelObjHavingOrganization)
	// orgTable := reflect.New(modelObjHavingOrganization.OrganizationType()).Interface()
	orgTableName := registry.OrgModelNameFromOrgResourceTypeString(typeString)
	joinTableName := orgJoinTableName(typeString)

	// orgTableName := models.GetOrganizationalTableNameFromModelTypeString(typeString)
	// orgTable, _ := reflect.New(models.GetOrganizationTypeFromTypeString(typeString)).Interface().(models.IModel)
	// joinTableName := models.GetJoinTableName(orgTable)

	ofield := *models.GetFieldNameFromModelByTagKey(registry.NewFromTypeString(typeString), "org")
	orgFieldName := strcase.SnakeCase(ofield)
	// orgFieldName := strcase.SnakeCase(modelObjHavingOrganization.GetOrganizationIDFieldName())

	// e.g. INNER JOIN \"organization\" ON \"dock\".\"OrganizationID\" = \"organization\".id
	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".\"%s\" = \"%s\".id AND \"%s\".\"id\" IN (?)", orgTableName, rtable, orgFieldName, orgTableName, rtable)
	// e.g. INNER JOIN \"user_owns_organization\" ON \"organization\".id = \"user_owns_organization\".model_id
	secondJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id", joinTableName, orgTableName, joinTableName)
	thirdJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)

	db2 := db.Table(rtable).Joins(firstJoin, ids).Joins(secondJoin).Joins(thirdJoin, who.GetUserID()) // .Find(modelObj).Error

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
func (serv *OrgPartition) GetAllQueryContructCore(db *gorm.DB, who models.UserIDFetchable, typeString string) (*gorm.DB, error) {
	// Graphically:
	// Model -- Org -- Join Table -- User
	// (Maybe organization should be defined in the library)
	// And it's organizational type has a user which includes
	// modelObjHavingOrganization, ok := registry.NewFromTypeString(typeString).(models.IHasOrganizationLink)
	// if !ok {
	// 	return nil, fmt.Errorf("Model %s does not comform to IHasOrganizationLink", typeString)
	// }

	rtable := registry.GetTableNameFromTypeString(typeString)
	// orgTableName := models.GetOrganizationTableName(modelObjHavingOrganization)
	orgTableName := registry.OrgModelNameFromOrgResourceTypeString(typeString)

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

	ofield := *models.GetFieldNameFromModelByTagKey(registry.NewFromTypeString(typeString), "org")
	orgFieldName := strcase.SnakeCase(ofield)

	// e.g. INNER JOIN \"organization\" ON \"dock\".\"OrganizationID\" = \"organization\".id
	firstJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".\"%s\" = \"%s\".id", orgTableName, rtable, orgFieldName, orgTableName)
	// e.g. INNER JOIN \"user_owns_organization\" ON \"organization\".id = \"user_owns_organization\".model_id
	secondJoin := fmt.Sprintf("INNER JOIN \"%s\" ON \"%s\".id = \"%s\".model_id", joinTableName, orgTableName, joinTableName)
	thirdJoin := fmt.Sprintf("INNER JOIN \"user\" ON \"user\".id = \"%s\".user_id AND \"%s\".user_id = ?", joinTableName, joinTableName)
	db = db.Table(rtable).Joins(firstJoin).Joins(secondJoin).Joins(thirdJoin, who.GetUserID().String())
	return db, nil
}

// GetAllRolesCore gets all roles according to the criteria
func (serv *OrgPartition) GetAllRolesCore(dbChained *gorm.DB, dbClean *gorm.DB, who models.UserIDFetchable, typeString string, modelObjs []models.IModel) ([]models.UserRole, error) {
	// modelObjHavingOrganization, _ := registry.NewFromTypeString(typeString).(models.IHasOrganizationLink)
	// orgTable := reflect.New(modelObjHavingOrganization.OrganizationType()).Interface()
	// orgTable := reflect.New(models.GetOrganizationTypeFromTypeString(typeString)).Interface().(models.IModel)
	// joinTableName := models.GetJoinTableName(orgTable)
	joinTableName := orgJoinTableName(typeString)

	rows, err := db.Shared().Table(joinTableName).Select("model_id, role").Where("user_id = ?", who.GetUserID().String()).Rows()
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

// UpdateOneCore one, permission should already be checked
// called for patch operation as well (after patch has already applied)
// Fuck, repeat the following code for now (you can't call the overriding method from the non-overriding one)
func (serv *OrgPartition) UpdateOneCore(db *gorm.DB, who models.UserIDFetchable, typeString string, modelObj models.IModel, id *datatypes.UUID, oldModelObj models.IModel) (modelObj2 models.IModel, err error) {
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
