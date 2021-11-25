package datamapper

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/models"
	qry "github.com/t2wu/betterrest/query"

	"github.com/jinzhu/gorm"
)

//------------------------------------
// User model only
//------------------------------------

// IChangeEmailPasswordMapper changes email and password
type IChangeEmailPasswordMapper interface {
	ChangeEmailPasswordWithID(db *gorm.DB, who models.Who,
		typeString string, modelobj models.IModel, id *datatypes.UUID) (models.IModel, error)
}

// IEmailVerificationMapper does verification email
type IEmailVerificationMapper interface {
	SendEmailVerification(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel) error
	VerifyEmail(db *gorm.DB, typeString string, id *datatypes.UUID, code string) error
}

// IResetPasswordMapper reset password in-case user forgotten it.
type IResetPasswordMapper interface {
	SendEmailResetPassword(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel) error
	ResetPassword(db *gorm.DB, typeString string, modelObj models.IModel, id *datatypes.UUID, code string) error
}

// -----------------------------------
// Base mapper
// -----------------------------------

// BaseMapper is a basic CRUD manager
type BaseMapper struct {
	Service service.IService
}

// CreateOne creates an instance of this model based on json and store it in db
func (mapper *BaseMapper) CreateOne(db *gorm.DB, who models.Who, typeString string, modelObj models.IModel,
	options *map[urlparam.Param]interface{}, cargo *models.ModelCargo) (models.IModel, error) {
	modelObj, err := mapper.Service.HookBeforeCreateOne(db, who, typeString, modelObj)
	if err != nil {
		return nil, err
	}

	var before, after *string
	if _, ok := modelObj.(models.IBeforeCreate); ok {
		b := "BeforeCreateDB"
		before = &b
	}
	if _, ok := modelObj.(models.IAfterCreate); ok {
		a := "AfterCreateDB"
		after = &a
	}

	j := opJob{
		serv:       mapper.Service,
		db:         db,
		who:        who,
		typeString: typeString,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
		crupdOp:  models.CRUPDOpCreate,
		cargo:    cargo,
		options:  options,
	}
	return opCore(before, after, j, mapper.Service.CreateOneCore)
}

// CreateMany creates an instance of this model based on json and store it in db
func (mapper *BaseMapper) CreateMany(db *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel,
	options *map[urlparam.Param]interface{}, cargo *models.BatchHookCargo) ([]models.IModel, error) {
	modelObjs, err := mapper.Service.HookBeforeCreateMany(db, who, typeString, modelObjs)
	if err != nil {
		return nil, err
	}

	before := models.ModelRegistry[typeString].BeforeCreate
	after := models.ModelRegistry[typeString].AfterCreate
	j := batchOpJob{
		serv:         mapper.Service,
		db:           db,
		who:          who,
		typeString:   typeString,
		oldmodelObjs: nil,
		modelObjs:    modelObjs,
		crupdOp:      models.CRUPDOpCreate,
		cargo:        cargo,
		options:      options,
	}
	return batchOpCore(j, before, after, mapper.Service.CreateOneCore)
}

// ReadOne get one model object based on its type and its id string
func (mapper *BaseMapper) ReadOne(db *gorm.DB, who models.Who, typeString string, id *datatypes.UUID,
	options *map[urlparam.Param]interface{}, cargo *models.ModelCargo) (models.IModel, models.UserRole, error) {
	// anyone permission can read as long as you are linked on db
	modelObj, role, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, nil, id, []models.UserRole{models.UserRoleAny})
	if err != nil {
		return nil, models.UserRoleInvalid, err
	}

	// After CRUPD hook
	if m, ok := modelObj.(models.IAfterCRUPD); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: cargo, Role: &role, URLParams: options}
		m.AfterCRUPDDB(hpdata, models.CRUPDOpRead)
	}

	// AfterRead hook
	if m, ok := modelObj.(models.IAfterRead); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: cargo, Role: &role, URLParams: options}
		if err := m.AfterReadDB(hpdata); err != nil {
			return nil, 0, err
		}
	}

	return modelObj, role, err
}

func createBuilderFromQueryParameters(urlParams url.Values, typeString string) (*qry.PredicateRelationBuilder, error) {
	var builder *qry.PredicateRelationBuilder
	for urlQueryKey, urlQueryVals := range urlParams {
		model := models.NewFromTypeString(typeString)
		fieldName, err := qry.JSONKeysToFieldName(model, urlQueryKey)
		if err != nil {
			continue // field name doesn't exists
		}

		// urlQueryKeys can be the same, or can be different field
		// But between urlQueryKeys it is always an AND relationship
		// AND relationship is outer, OR relationship is inner

		var innerBuilder *qry.PredicateRelationBuilder
		for _, urlQueryVal := range urlQueryVals {
			// query value can have ">30;<20" and it's an OR relationship
			// since query key can be different fields, or multiple same fields
			// it's AND relationship on the outer keys
			urlQueryVal = strings.TrimSpace(urlQueryVal)
			conditions := strings.Split(urlQueryVal, ";")
			predicate, value := getPredicateAndValueFromFieldValue2(conditions[0])
			predicate = strings.TrimSpace(predicate)
			value = strings.TrimSpace(value)
			innerBuilder = qry.C(fieldName+" "+predicate, value)

			for _, condition := range conditions[1:] {
				predicate, value := getPredicateAndValueFromFieldValue2(condition)
				predicate = strings.TrimSpace(predicate)
				value = strings.TrimSpace(value)
				innerBuilder = innerBuilder.Or(fieldName+" "+predicate, value)
				// qry.Q(db, qry.C(fieldName+" "+predicate, value))
			}
		}

		if builder == nil {
			builder = qry.C(innerBuilder)
		} else {
			builder = builder.And(innerBuilder) // outer is AND
		}
	}
	return builder, nil
}

// ReadMany obtains a slice of models.DomainModel
// options can be string "offset" and "limit", both of type int
// This is very Javascript-esque. I would have liked Python's optional parameter more.
// Alas, no such feature in Go. https://stackoverflow.com/questions/2032149/optional-parameters-in-go
// How does Gorm do the following? Might want to check out its source code.
// Cancel offset condition with -1
//  db.Offset(10).Find(&users1).Offset(-1).Find(&users2)
// func (mapper *BaseMapper) ReadMany(db *gorm.DB, who models.Who, typeString string,
// 	options *map[urlparam.Param]interface{}, cargo *models.BatchHookCargo) ([]models.IModel, []models.UserRole, *int, error) {
// 	dbClean := db
// 	db = db.Set("gorm:auto_preload", true)

// 	offset, limit, cstart, cstop, order, latestn, latestnons, totalcount := urlparam.GetOptions(*options)
// 	rtable := models.GetTableNameFromTypeString(typeString)

// 	var builder *qry.PredicateRelationBuilder
// 	if urlParams, ok := (*options)[urlparam.ParamOtherQueries].(url.Values); ok && len(urlParams) != 0 {
// 		builder = createBuilderFromQueryParameters(urlParams, typeString)
// 	}

// 	builder = builder.C("CreatedAt BETWEEN", time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))

// 	var err error
// 	// db, err = constructInnerFieldParamQueries(db, typeString, options, latestn, latestnons)
// 	// if err != nil {
// 	// 	return nil, nil, nil, err
// 	// }

// 	db, err = mapper.Service.GetAllQueryContructCore(db, who, typeString)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	if order != nil && *order == "asc" {
// 		qry.Q(db, builder).Order("CreatedAt ASC")
// 	} else {
// 		qry.Q(db, builder).Order("CreatedAt DESC")
// 	}

// 	var no *int
// 	if totalcount {
// 		no = new(int)
// 		// Query for total count, without offset and limit (all)
// 		if err := db.Count(no).Error; err != nil {
// 			log.Println("count error:", err)
// 			return nil, nil, nil, err
// 		}
// 	}

// 	// chain offset and limit
// 	if offset != nil && limit != nil {
// 		db = db.Offset(*offset).Limit(*limit)
// 	} else if cstart == nil && cstop == nil { // default to 100 maximum unless time is specified
// 		db = db.Offset(0).Limit(100)
// 	}

// 	// Actual quer in the following line
// 	outmodels, err := models.NewSliceFromDBByTypeString(typeString, db.Find)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	roles, err := mapper.Service.GetAllRolesCore(db, dbClean, who, typeString, outmodels)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}

// 	// safeguard, Must be coded wrongly
// 	if len(outmodels) != len(roles) {
// 		return nil, nil, nil, errors.New("unknown query error")
// 	}

// 	// make many to many tag works
// 	for _, m := range outmodels {
// 		err = gormfixes.LoadManyToManyBecauseGormFailsWithID(dbClean, m)
// 		if err != nil {
// 			return nil, nil, nil, err
// 		}
// 	}

// 	// the AfterCRUPD hookpoint
// 	// use dbClean cuz it's not chained
// 	if after := models.ModelRegistry[typeString].AfterCRUPD; after != nil {
// 		bhpData := models.BatchHookPointData{Ms: outmodels, DB: dbClean, Who: who,
// 			TypeString: typeString, Roles: roles, Cargo: cargo, URLParams: options}
// 		if err = after(bhpData, models.CRUPDOpRead); err != nil {
// 			return nil, nil, nil, err
// 		}
// 	}

// 	// AfterRead hookpoint
// 	// use dbClean cuz it's not chained
// 	if after := models.ModelRegistry[typeString].AfterRead; after != nil {
// 		bhpData := models.BatchHookPointData{Ms: outmodels, DB: dbClean, Who: who,
// 			TypeString: typeString, Roles: roles, Cargo: cargo, URLParams: options}
// 		if err = after(bhpData); err != nil {
// 			return nil, nil, nil, err
// 		}
// 	}

// 	return outmodels, roles, no, err
// }

func (mapper *BaseMapper) ReadMany(db *gorm.DB, who models.Who, typeString string,
	options *map[urlparam.Param]interface{}, cargo *models.BatchHookCargo) ([]models.IModel, []models.UserRole, *int, error) {
	dbClean := db
	db = db.Set("gorm:auto_preload", true)

	offset, limit, cstart, cstop, order, latestn, latestnons, totalcount := urlparam.GetOptions(*options)
	rtable := models.GetTableNameFromTypeString(typeString)

	if cstart != nil && cstop != nil {
		db = db.Where(rtable+".created_at BETWEEN ? AND ?", time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))
	}

	var err error
	var builder *qry.PredicateRelationBuilder
	if latestn != nil { // query module currently don't handle latestn, use old if so
		db, err = constructInnerFieldParamQueries(db, typeString, options, latestn, latestnons)
		if err != nil {
			return nil, nil, nil, err
		}
	} else {
		if urlParams, ok := (*options)[urlparam.ParamOtherQueries].(url.Values); ok && len(urlParams) != 0 {
			builder, err = createBuilderFromQueryParameters(urlParams, typeString)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}

	db = constructOrderFieldQueries(db, rtable, order)
	db, err = mapper.Service.GetAllQueryContructCore(db, who, typeString)
	if err != nil {
		return nil, nil, nil, err
	}

	var no *int
	if totalcount {
		no = new(int)
		if builder == nil {
			// Query for total count, without offset and limit (all)
			if err := db.Count(no).Error; err != nil {
				log.Println("count error:", err)
				return nil, nil, nil, err
			}
		} else {
			q := qry.Q(db, builder)
			if err := q.Count(models.NewFromTypeString(typeString), no).Error(); err != nil {
				log.Println("count error:", err)
				return nil, nil, nil, err
			}

			// Fetch it back, so the builder stuff is in there
			// And resort back to Gorm.
			// db = q.GetDB()
		}
	}

	// chain offset and limit
	if offset != nil && limit != nil {
		db = db.Offset(*offset).Limit(*limit)
	} else if cstart == nil && cstop == nil { // default to 100 maximum unless time is specified
		db = db.Offset(0).Limit(100)
	}

	if builder != nil {
		db, err = qry.Q(db, builder).BuildQuery(models.NewFromTypeString(typeString))
		if err != nil {
			return nil, nil, nil, err
		}
	}

	// Actual quer in the following line
	outmodels, err := models.NewSliceFromDBByTypeString(typeString, db.Find)
	if err != nil {
		return nil, nil, nil, err
	}

	roles, err := mapper.Service.GetAllRolesCore(db, dbClean, who, typeString, outmodels)
	if err != nil {
		return nil, nil, nil, err
	}

	// safeguard, Must be coded wrongly
	if len(outmodels) != len(roles) {
		return nil, nil, nil, errors.New("unknown query error")
	}

	// make many to many tag works
	for _, m := range outmodels {
		err = gormfixes.LoadManyToManyBecauseGormFailsWithID(dbClean, m)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	// the AfterCRUPD hookpoint
	// use dbClean cuz it's not chained
	if after := models.ModelRegistry[typeString].AfterCRUPD; after != nil {
		bhpData := models.BatchHookPointData{Ms: outmodels, DB: dbClean, Who: who,
			TypeString: typeString, Roles: roles, Cargo: cargo, URLParams: options}
		if err = after(bhpData, models.CRUPDOpRead); err != nil {
			return nil, nil, nil, err
		}
	}

	// AfterRead hookpoint
	// use dbClean cuz it's not chained
	if after := models.ModelRegistry[typeString].AfterRead; after != nil {
		bhpData := models.BatchHookPointData{Ms: outmodels, DB: dbClean, Who: who,
			TypeString: typeString, Roles: roles, Cargo: cargo, URLParams: options}
		if err = after(bhpData); err != nil {
			return nil, nil, nil, err
		}
	}

	return outmodels, roles, no, err
}

// UpdateOne updates model based on this json
func (mapper *BaseMapper) UpdateOne(db *gorm.DB, who models.Who, typeString string,
	modelObj models.IModel, id *datatypes.UUID, options *map[urlparam.Param]interface{}, cargo *models.ModelCargo) (models.IModel, error) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, modelObj, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	// TODO: Huh? How do we do validation here?!
	var before, after *string
	if _, ok := modelObj.(models.IBeforeUpdate); ok {
		b := "BeforeUpdateDB"
		before = &b
	}
	if _, ok := modelObj.(models.IAfterUpdate); ok {
		a := "AfterUpdateDB"
		after = &a
	}

	j := opJob{
		serv:        mapper.Service,
		db:          db,
		who:         who,
		typeString:  typeString,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,
		crupdOp:     models.CRUPDOpUpdate,
		cargo:       cargo,
		options:     options,
	}
	return opCore(before, after, j, mapper.Service.UpdateOneCore)
}

// UpdateMany updates multiple models
func (mapper *BaseMapper) UpdateMany(db *gorm.DB, who models.Who, typeString string,
	modelObjs []models.IModel, options *map[urlparam.Param]interface{},
	cargo *models.BatchHookCargo) ([]models.IModel, error) {
	// load old model data
	ids := make([]*datatypes.UUID, len(modelObjs))
	for i, modelObj := range modelObjs {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		id := modelObj.GetID()
		if id == nil || id.String() == "" {
			return nil, service.ErrIDEmpty
		}
		ids[i] = id
	}

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper.Service, db, who, typeString, ids, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	// load and check is not in the same order as modelobj
	oldModelObjs = mapper.sortOldModelByIds(oldModelObjs, ids)

	before := models.ModelRegistry[typeString].BeforeUpdate
	after := models.ModelRegistry[typeString].AfterUpdate
	j := batchOpJob{
		serv:         mapper.Service,
		db:           db,
		who:          who,
		typeString:   typeString,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
		crupdOp:      models.CRUPDOpUpdate,
		cargo:        cargo,
		options:      options,
	}
	return batchOpCore(j, before, after, mapper.Service.UpdateOneCore)
}

// PatchOne updates model based on this json
func (mapper *BaseMapper) PatchOne(db *gorm.DB, who models.Who, typeString string, jsonPatch []byte,
	id *datatypes.UUID, options *map[urlparam.Param]interface{}, cargo *models.ModelCargo) (models.IModel, error) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, nil, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	if m, ok := oldModelObj.(models.IBeforePatchApply); ok {
		hpdata := models.HookPointData{DB: db, Who: who, TypeString: typeString, Cargo: cargo}
		if err := m.BeforePatchApplyDB(hpdata); err != nil {
			return nil, err
		}
	}

	// Apply patch operations
	modelObj, err := applyPatchCore(typeString, oldModelObj, jsonPatch)
	if err != nil {
		return nil, err
	}

	// TODO: Huh? How do we do validation here?!
	var before, after *string
	if _, ok := modelObj.(models.IBeforePatch); ok {
		b := "BeforePatchDB"
		before = &b
	}

	if _, ok := modelObj.(models.IAfterPatch); ok {
		a := "AfterPatchDB"
		after = &a
	}

	j := opJob{
		serv:        mapper.Service,
		db:          db,
		who:         who,
		typeString:  typeString,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,
		crupdOp:     models.CRUPDOpPatch,
		cargo:       cargo,
		options:     options,
	}
	return opCore(before, after, j, mapper.Service.UpdateOneCore)
}

// PatchMany patches multiple models
func (mapper *BaseMapper) PatchMany(db *gorm.DB, who models.Who, typeString string,
	jsonIDPatches []models.JSONIDPatch, options *map[urlparam.Param]interface{},
	cargo *models.BatchHookCargo) ([]models.IModel, error) {
	// Load data, patch it, then send it to the hookpoint
	// Load IDs
	ids := make([]*datatypes.UUID, len(jsonIDPatches))
	for i, jsonIDPatch := range jsonIDPatches {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		if jsonIDPatch.ID.String() == "" {
			return nil, service.ErrIDEmpty
		}
		ids[i] = jsonIDPatch.ID
	}

	oldModelObjs, _, err := loadManyAndCheckBeforeModify(mapper.Service, db, who, typeString, ids, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	// load and check is not in the same order as modelobj
	oldModelObjs = mapper.sortOldModelByIds(oldModelObjs, ids)

	// Hookpoint BEFORE BeforeCRUD and BeforePatch
	// This is called BEFORE the actual patch
	beforeApply := models.ModelRegistry[typeString].BeforePatchApply
	if beforeApply != nil {
		bhpData := models.BatchHookPointData{Ms: oldModelObjs, DB: db, Who: who, TypeString: typeString,
			Cargo: cargo}
		if err := beforeApply(bhpData); err != nil {
			return nil, err
		}
	}

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

	// Finally update them
	before := models.ModelRegistry[typeString].BeforePatch
	after := models.ModelRegistry[typeString].AfterPatch
	j := batchOpJob{
		serv:         mapper.Service,
		db:           db,
		who:          who,
		typeString:   typeString,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
		crupdOp:      models.CRUPDOpPatch,
		cargo:        cargo,
		options:      options,
	}
	return batchOpCore(j, before, after, mapper.Service.UpdateOneCore)
}

// DeleteOne delete the model
// TODO: delete the groups associated with this record?
func (mapper *BaseMapper) DeleteOne(db *gorm.DB, who models.Who, typeString string,
	id *datatypes.UUID, options *map[urlparam.Param]interface{}, cargo *models.ModelCargo) (models.IModel, error) {
	modelObj, _, err := loadAndCheckErrorBeforeModify(mapper.Service, db, who, typeString, nil, id, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	modelObj, err = mapper.Service.HookBeforeDeleteOne(db, who, typeString, modelObj)
	if err != nil {
		return nil, err
	}

	var before, after *string
	if _, ok := modelObj.(models.IBeforeDelete); ok {
		b := "BeforeDeleteDB"
		before = &b
	}
	if _, ok := modelObj.(models.IAfterDelete); ok {
		a := "AfterDeleteDB"
		after = &a
	}

	j := opJob{
		serv:       mapper.Service,
		db:         db,
		who:        who,
		typeString: typeString,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
		crupdOp:  models.CRUPDOpDelete,
		cargo:    cargo,
		options:  options,
	}
	return opCore(before, after, j, mapper.Service.DeleteOneCore)
}

// DeleteMany deletes multiple models
func (mapper *BaseMapper) DeleteMany(db *gorm.DB, who models.Who, typeString string, modelObjs []models.IModel,
	options *map[urlparam.Param]interface{}, cargo *models.BatchHookCargo) ([]models.IModel, error) {
	// load old model data
	ids := make([]*datatypes.UUID, len(modelObjs))
	for i, modelObj := range modelObjs {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		id := modelObj.GetID()
		if id == nil || id.String() == "" {
			return nil, service.ErrIDEmpty
		}
		ids[i] = id
	}

	modelObjs, _, err := loadManyAndCheckBeforeModify(mapper.Service, db, who, typeString, ids, []models.UserRole{models.UserRoleAdmin})
	if err != nil {
		return nil, err
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if len(modelObjs) > 0 && modelNeedsRealDelete(modelObjs[0]) {
		db = db.Unscoped() // hookpoint will inherit this though
	}

	modelObjs, err = mapper.Service.HookBeforeDeleteMany(db, who, typeString, modelObjs)
	if err != nil {
		return nil, err
	}

	before := models.ModelRegistry[typeString].BeforeDelete
	after := models.ModelRegistry[typeString].AfterDelete

	j := batchOpJob{
		serv:       mapper.Service,
		db:         db,
		who:        who,
		typeString: typeString,
		modelObjs:  modelObjs,
		crupdOp:    models.CRUPDOpDelete,
		cargo:      cargo,
		options:    options,
	}
	return batchOpCore(j, before, after, mapper.Service.DeleteOneCore)
}

// ----------------------------------------------------------------------------------------

func (mapper *BaseMapper) sortOldModelByIds(oldModelObjs []models.IModel, ids []*datatypes.UUID) []models.IModel {
	// build dictionary of old model objs
	mapping := make(map[string]models.IModel)
	for _, oldModelObj := range oldModelObjs {
		mapping[oldModelObj.GetID().String()] = oldModelObj
	}

	oldModelObjSorted := make([]models.IModel, 0)
	for _, id := range ids {
		oldModelObjSorted = append(oldModelObjSorted, mapping[id.String()])
	}
	return oldModelObjSorted
}

func constructOrderFieldQueries(db *gorm.DB, tableName string, order *string) *gorm.DB {
	if order != nil && *order == "asc" {
		db = db.Order(fmt.Sprintf("\"%s\".created_at ASC", tableName))
	} else {
		db = db.Order(fmt.Sprintf("\"%s\".created_at DESC", tableName)) // descending by default
	}
	return db
}
