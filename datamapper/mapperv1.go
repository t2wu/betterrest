package datamapper

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/datamapper/hfetcher"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/utils/letters"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/betterrest/registry"
	"github.com/t2wu/qry"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"

	"github.com/jinzhu/gorm"
)

// -----------------------------------
// Base mapper
// -----------------------------------

// DataMapper is a basic CRUD manager
type DataMapper struct {
	Service service.IServiceV1
}

// CreateMany creates an instance of this model based on json and store it in db
func (mapper *DataMapper) CreateMany(db *gorm.DB, modelObjs []mdl.IModel,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	modelObjs, err := mapper.Service.HookBeforeCreateMany(db, ep.Who, ep.TypeString, modelObjs)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	roles := make([]userrole.UserRole, len(modelObjs))
	for i := range roles {
		roles[i] = userrole.UserRoleAdmin // has to be admin to create
	}

	data := hook.Data{Ms: modelObjs, DB: db, Roles: roles, Cargo: cargo}
	initData := hook.InitData{Roles: roles, Ep: ep}

	j := batchOpJobV1{
		serv:         mapper.Service,
		oldmodelObjs: nil,
		modelObjs:    modelObjs,

		fetcher: hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:    &data,
		ep:      ep,
	}
	return batchOpCoreV1(j, mapper.Service.CreateOneCore)
}

// CreateOne creates an instance of this model based on json and store it in db
func (mapper *DataMapper) CreateOne(db *gorm.DB, modelObj mdl.IModel,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	modelObj, err := mapper.Service.HookBeforeCreateOne(db, ep.Who, ep.TypeString, modelObj)

	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Cargo: cargo}
	initData := hook.InitData{Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Ep: ep}

	j := opJobV1{
		serv:     mapper.Service,
		modelObj: modelObj,
		fetcher:  hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:     &data,
		ep:       ep,
	}
	return opCoreV1(j, mapper.Service.CreateOneCore)
}

func (mapper *DataMapper) ReadMany(db *gorm.DB, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, []userrole.UserRole, *int, *webrender.RetError) {
	dbClean := db
	db = db.Set("gorm:auto_preload", true)

	initData := hook.InitData{Roles: nil, Ep: ep}
	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// New after hooks
	// fetch all handlers with before hooks
	var outmodels []mdl.IModel
	var roles []userrole.UserRole
	var no *int

	cacheMiss := true
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "C") { // C for cache
		var retErr *webrender.RetError
		var handled bool
		handled, _, outmodels, roles, no, retErr = hdlr.(hook.ICache).GetFromCache(ep)
		if retErr != nil {
			return nil, nil, nil, retErr
		}

		if handled {
			cacheMiss = false
			break
		}
	}

	offset, limit, cstart, cstop, orderby, order, latestn, latestnons, totalcount := urlparam.GetOptions(ep.URLParams)
	rtable := registry.GetTableNameFromTypeString(ep.TypeString)

	if cstart != nil && cstop != nil {
		db = db.Where(rtable+`.created_at BETWEEN ? AND ?`, time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))
	}

	if cacheMiss {
		var err error
		var builder *qry.PredicateRelationBuilder
		if latestn != nil { // query module currently don't handle latestn, use old if so
			db, err = constructInnerFieldParamQueries(db, ep.TypeString, ep.URLParams, latestn, latestnons)
			if err != nil {
				return nil, nil, nil, &webrender.RetError{Error: err}
			}
		} else {
			if urlParams, ok := ep.URLParams[urlparam.ParamOtherQueries].(url.Values); ok && len(urlParams) != 0 {
				builder, err = createBuilderFromQueryParameters(urlParams, ep.TypeString)
				if err != nil {
					return nil, nil, nil, &webrender.RetError{Error: err}
				}
			}
		}

		db, err = constructOrderFieldQueries(db, ep.TypeString, rtable, orderby, order)
		if err != nil {
			return nil, nil, nil, &webrender.RetError{Error: err}
		}
		db, err = mapper.Service.GetAllQueryContructCore(db, ep.Who, ep.TypeString)
		if err != nil {
			return nil, nil, nil, &webrender.RetError{Error: err}
		}

		if totalcount {
			no = new(int)
			if builder == nil {
				// Query for total count, without offset and limit (all)
				if err := db.Count(no).Error; err != nil {
					return nil, nil, nil, &webrender.RetError{Error: err}
				}
			} else {
				q := qry.Q(db, builder)
				if err := q.Count(registry.NewFromTypeString(ep.TypeString), no).Error(); err != nil {
					return nil, nil, nil, &webrender.RetError{Error: err}
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
			db, err = qry.Q(db, builder).BuildQuery(registry.NewFromTypeString(ep.TypeString))
			if err != nil {
				return nil, nil, nil, &webrender.RetError{Error: err}
			}
		}

		// Actual query in the following line
		outmodels, err = registry.NewSliceFromDBByTypeString(ep.TypeString, db.Find)
		if err != nil {
			return nil, nil, nil, &webrender.RetError{Error: err}
		}

		roles, err = mapper.Service.GetAllRolesCore(db, dbClean, ep.Who, ep.TypeString, outmodels)
		if err != nil {
			return nil, nil, nil, &webrender.RetError{Error: err}
		}

		// safeguard, Must be coded wrongly
		if len(outmodels) != len(roles) {
			return nil, nil, nil, &webrender.RetError{Error: errors.New("unknown query error")}
		}

		// make many to many tag works
		for _, m := range outmodels {
			err = gormfixes.LoadManyToManyBecauseGormFailsWithID(dbClean, m)
			if err != nil {
				return nil, nil, nil, &webrender.RetError{Error: err}
			}
		}

		// Add to cache if any defined
		for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "C") { // C for cache
			var retErr *webrender.RetError
			var handled bool
			found := false // for ReadMany this isn't used
			handled, retErr = hdlr.(hook.ICache).AddToCache(ep, found, outmodels, roles, no)
			if retErr != nil {
				return nil, nil, nil, retErr
			}

			if handled {
				break
			}
		}
	}

	// use dbClean cuz it's not chained
	data := hook.Data{Ms: outmodels, DB: dbClean, Roles: roles, Cargo: cargo}
	// initData := hook.InitData{Roles: roles, Ep: ep}
	// fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// New after hooks
	// fetch all handlers with before hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "A") {
		if retErr := hdlr.(hook.IAfter).After(&data, ep); retErr != nil {
			return nil, nil, nil, retErr
		}
	}

	retval := &MapperRet{
		Ms:      outmodels,
		Fetcher: fetcher,
	}

	return retval, roles, no, nil
}

// ReadOne get one model object based on its type and its id string
func (mapper *DataMapper) ReadOne(db *gorm.DB, id *datatype.UUID, ep *hook.EndPoint,
	cargo *hook.Cargo) (*MapperRet, userrole.UserRole, *webrender.RetError) {
	var modelObj mdl.IModel
	var role userrole.UserRole
	var err error

	// cache time needs to be small, otherwise if permission removed it's not as immediate
	cacheMiss := true
	initData := hook.InitData{Roles: nil, Ep: ep}
	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "C") { // C for cache
		// var retErr *webrender.RetError
		// var handled bool
		// var outmodels []mdl.IModel
		// var roles []userrole.UserRole
		// var found bool
		handled, found, outmodels, roles, _, retErr := hdlr.(hook.ICache).GetFromCache(ep)
		if retErr != nil {
			return nil, userrole.UserRoleInvalid, retErr
		}

		if handled {
			cacheMiss = false

			if !found { // cache result is found, but the result is "not found"
				return nil, userrole.UserRoleInvalid, webrender.NewRetValWithRendererError(err, webrender.NewErrNotFound(err))
			}

			modelObj = outmodels[0]
			role = roles[0]
			break
		}
	}

	if cacheMiss {
		// anyone permission can read as long as you are linked on db
		modelObj, role, err = loadAndCheckErrorBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, nil, id, []userrole.UserRole{userrole.UserRoleAny})
		var found bool = true
		if err != nil {
			if err.Error() != "record not found" { // real error
				return nil, userrole.UserRoleInvalid, &webrender.RetError{Error: err}
			}
			found = false
		}

		// Add to cache if hook defined
		for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "C") { // C for cache
			var retErr *webrender.RetError
			var handled bool
			if found {
				handled, retErr = hdlr.(hook.ICache).AddToCache(ep, found, []mdl.IModel{modelObj}, []userrole.UserRole{role}, nil)
			} else { // not found
				handled, retErr = hdlr.(hook.ICache).AddToCache(ep, found, nil, nil, nil)
			}

			if retErr != nil {
				return nil, userrole.UserRoleInvalid, &webrender.RetError{Error: err}
			}

			if handled {
				break
			}
		}

		if !found {
			// not found
			return nil, userrole.UserRoleInvalid, webrender.NewRetValWithRendererError(err, webrender.NewErrNotFound(err))
		}
	}
	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{role}, Cargo: cargo}

	// fetch all handlers with before hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "A") {
		if retErr := hdlr.(hook.IAfter).After(&data, ep); retErr != nil {
			return nil, role, retErr
		}
	}
	ret := MapperRet{
		Ms:      []mdl.IModel{modelObj},
		Fetcher: fetcher,
	}

	return &ret, role, nil
}

// UpdateOne updates model based on this json
func (mapper *DataMapper) UpdateOne(db *gorm.DB, modelObj mdl.IModel, id *datatype.UUID,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, modelObj, id, []userrole.UserRole{userrole.UserRoleAdmin, userrole.UserRolePublic})
	if err != nil {
		if err.Error() == "record not found" {
			return nil, webrender.NewRetValWithRendererError(err, webrender.NewErrNotFound(err))
		}

		return nil, &webrender.RetError{Error: err}
	}

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Cargo: cargo}
	initData := hook.InitData{Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Ep: ep}

	j := opJobV1{
		serv:        mapper.Service,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,

		fetcher: hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:    &data,
		ep:      ep,
	}
	return opCoreV1(j, mapper.Service.UpdateOneCore)
}

// UpdateMany updates multiple mdl
func (mapper *DataMapper) UpdateMany(db *gorm.DB, modelObjs []mdl.IModel, ep *hook.EndPoint,
	cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	// load old model data
	ids := make([]*datatype.UUID, len(modelObjs))
	for i, modelObj := range modelObjs {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		id := modelObj.GetID()
		if id == nil || id.String() == "" {
			return nil, &webrender.RetError{Error: service.ErrIDEmpty}
		}
		ids[i] = id
	}

	oldModelObjs, roles, err := loadManyAndCheckBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, ids, []userrole.UserRole{userrole.UserRoleAdmin, userrole.UserRolePublic})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	// load and check is not in the same order as modelobj
	oldModelObjs, roles = mapper.sortOldModelAndRolesByIds(oldModelObjs, roles, ids)

	data := hook.Data{Ms: modelObjs, DB: db, Roles: roles, Cargo: cargo}
	initData := hook.InitData{Roles: roles, Ep: ep}

	j := batchOpJobV1{
		serv:         mapper.Service,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
		fetcher:      hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:         &data,
		ep:           ep,
	}
	return batchOpCoreV1(j, mapper.Service.UpdateOneCore)
}

// PatchOne updates model based on this json
func (mapper *DataMapper) PatchOne(db *gorm.DB, jsonPatch []byte,
	id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	oldModelObj, role, err := loadAndCheckErrorBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, nil, id, []userrole.UserRole{userrole.UserRoleAdmin})
	if err != nil {
		if err.Error() == "record not found" {
			return nil, webrender.NewRetValWithRendererError(err, webrender.NewErrNotFound(err))
		}

		return nil, &webrender.RetError{Error: err}
	}

	initData := hook.InitData{Roles: []userrole.UserRole{role}, Ep: ep}
	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	data := hook.Data{Ms: []mdl.IModel{oldModelObj}, DB: db, Roles: []userrole.UserRole{role}, Cargo: cargo}

	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "J") {
		if retErr := hdlr.(hook.IBeforeApply).BeforeApply(&data, ep); retErr != nil {
			return nil, retErr
		}
	}

	// Apply patch operations
	modelObj, err := applyPatchCore(ep.TypeString, oldModelObj, jsonPatch)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	// VALIDATION, TODO: What is this?
	err = mdl.Validate.Struct(modelObj)
	if errs, ok := err.(validator.ValidationErrors); ok {
		s, err2 := mdl.TranslateValidationErrorMessage(errs, modelObj)
		if err2 != nil {
			log.Println("error translating validaiton message:", err)
		}
		err = errors.New(s)
		return nil, webrender.NewRetValWithError(err)
	}

	data.Ms = []mdl.IModel{modelObj} // modify the one already in hook

	j := opJobV1{
		serv:        mapper.Service,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,
		fetcher:     fetcher,
		data:        &data,
		ep:          ep,
	}
	return opCoreV1(j, mapper.Service.UpdateOneCore)
}

// PatchMany patches multiple mdl
func (mapper *DataMapper) PatchMany(db *gorm.DB, jsonIDPatches []mdlutil.JSONIDPatch,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	// Load data, patch it, then send it to the hookpoint
	// Load IDs
	ids := make([]*datatype.UUID, len(jsonIDPatches))
	for i, jsonIDPatch := range jsonIDPatches {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		if jsonIDPatch.ID.String() == "" {
			return nil, &webrender.RetError{Error: service.ErrIDEmpty}
		}
		ids[i] = jsonIDPatch.ID
	}

	oldModelObjs, roles, err := loadManyAndCheckBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, ids, []userrole.UserRole{userrole.UserRoleAdmin})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	// load and check is not in the same order as modelobj
	oldModelObjs, roles = mapper.sortOldModelAndRolesByIds(oldModelObjs, roles, ids)

	// roles := make([]userrole.UserRole, len(jsonIDPatches))
	// for i := range roles {
	// 	roles[i] = userrole.UserRoleAdmin // has to be admin to patch
	// }

	initData := hook.InitData{Roles: roles, Ep: ep}
	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// here we put the oldModelObjs, not patched yet
	data := hook.Data{Ms: oldModelObjs, DB: db, Roles: roles, Cargo: cargo}

	// fetch all handlers with before hooks
	for _, hdlr := range fetcher.FetchHandlersForOpAndHook(ep.Op, "J") {
		if retErr := hdlr.(hook.IBeforeApply).BeforeApply(&data, ep); retErr != nil {
			return nil, retErr
		}
	}

	// Now patch it
	modelObjs := make([]mdl.IModel, len(oldModelObjs))
	for i, jsonIDPatch := range jsonIDPatches {
		// Apply patch operations
		modelObjs[i], err = applyPatchCore(ep.TypeString, oldModelObjs[i], []byte(jsonIDPatch.Patch))
		if err != nil {
			return nil, &webrender.RetError{Error: err}
		}
	}

	// here we put the new model objs (this should modify the one already in hook)
	data.Ms = modelObjs

	// Finally update them
	j := batchOpJobV1{
		serv:         mapper.Service,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,
		fetcher:      fetcher,
		data:         &data,
		ep:           ep,
	}
	return batchOpCoreV1(j, mapper.Service.UpdateOneCore)
}

// DeleteOne delete the model
// TODO: delete the groups associated with this record?
func (mapper *DataMapper) DeleteOne(db *gorm.DB, id *datatype.UUID, ep *hook.EndPoint,
	cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	modelObj, role, err := loadAndCheckErrorBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, nil, id, []userrole.UserRole{userrole.UserRoleAdmin})
	if err != nil {
		if err.Error() == "record not found" {
			return nil, webrender.NewRetValWithRendererError(err, webrender.NewErrNotFound(err))
		}
		return nil, &webrender.RetError{Error: err}
	}

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if modelNeedsRealDelete(modelObj) {
		db = db.Unscoped()
	}

	modelObj, err = mapper.Service.HookBeforeDeleteOne(db, ep.Who, ep.TypeString, modelObj)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{role}, Cargo: cargo}
	initData := hook.InitData{Roles: []userrole.UserRole{role}, Ep: ep}

	j := opJobV1{
		serv: mapper.Service,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,
		fetcher:  hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:     &data,
		ep:       ep,
	}
	return opCoreV1(j, mapper.Service.DeleteOneCore)
}

// DeleteMany deletes multiple mdl
func (mapper *DataMapper) DeleteMany(db *gorm.DB, modelObjs []mdl.IModel,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {

	// Delete many with limit and order
	// https://stackoverflow.com/questions/5170546/how-do-i-delete-a-fixed-number-of-rows-with-sorting-in-postgresql

	// load old model data
	ids := make([]*datatype.UUID, len(modelObjs))
	for i, modelObj := range modelObjs {
		// Check error, make sure it has an id and not empty string (could potentially update all records!)
		id := modelObj.GetID()
		if id == nil || id.String() == "" {
			return nil, &webrender.RetError{Error: service.ErrIDEmpty}
		}
		ids[i] = id
	}

	// log.Println("load and check before modify v1 1")
	modelObjs, roles, err := loadManyAndCheckBeforeModifyV1(mapper.Service, db, ep.Who, ep.TypeString, ids, []userrole.UserRole{userrole.UserRoleAdmin, userrole.UserRolePublic})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}
	// log.Println("load and check before modify v1 2")

	// Unscoped() for REAL delete!
	// Foreign key constraint works only on real delete
	// Soft delete will take more work, have to verify myself manually
	if len(modelObjs) > 0 && modelNeedsRealDelete(modelObjs[0]) {
		db = db.Unscoped() // hookpoint will inherit this though
	}

	modelObjs, err = mapper.Service.HookBeforeDeleteMany(db, ep.Who, ep.TypeString, modelObjs)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	data := hook.Data{Ms: modelObjs, DB: db, Roles: roles, Cargo: cargo}
	initData := hook.InitData{Roles: roles, Ep: ep}

	j := batchOpJobV1{
		serv:      mapper.Service,
		modelObjs: modelObjs,
		fetcher:   hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:      &data,
		ep:        ep,
	}
	return batchOpCoreV1(j, mapper.Service.DeleteOneCore)
}

// ----------------------------------------------------------------------------------------

func (mapper *DataMapper) sortOldModelAndRolesByIds(oldModelObjs []mdl.IModel, roles []userrole.UserRole, ids []*datatype.UUID) ([]mdl.IModel, []userrole.UserRole) {
	mapping := make(map[string]int) // stores index
	for i, oldModelObj := range oldModelObjs {
		mapping[oldModelObj.GetID().String()] = i
	}

	oldModelObjSorted := make([]mdl.IModel, 0)
	rolesSorted := make([]userrole.UserRole, 0)
	for _, id := range ids {
		idx := mapping[id.String()]
		oldModelObjSorted = append(oldModelObjSorted, oldModelObjs[idx])
		rolesSorted = append(rolesSorted, roles[idx])
	}

	return oldModelObjSorted, rolesSorted
}

func constructOrderFieldQueries(db *gorm.DB, typeString string, tableName string, orderby *string, order *string) (*gorm.DB, error) {
	if orderby != nil {
		modelObj := registry.NewFromTypeString(typeString)
		// Make sure orderby is within the field
		if _, err := datatype.GetModelFieldTypeIfValid(modelObj, letters.CamelCaseToPascalCase(*orderby)); err != nil {
			return nil, err
		}
	}

	if orderby == nil {
		orderbyField := `created_at`
		orderby = &orderbyField
	}

	if order != nil && *order == "asc" {
		db = db.Order(fmt.Sprintf(`"%s"."%s" ASC`, tableName, *orderby))
	} else {
		db = db.Order(fmt.Sprintf(`"%s"."%s" DESC`, tableName, *orderby)) // descending by default
	}
	return db, nil
}

func createBuilderFromQueryParameters(urlParams url.Values, typeString string) (*qry.PredicateRelationBuilder, error) {
	var builder *qry.PredicateRelationBuilder
	for urlQueryKey, urlQueryVals := range urlParams {
		model := registry.NewFromTypeString(typeString)
		fieldName, err := mdl.JSONKeysToFieldName(model, urlQueryKey)
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

			if innerBuilder == nil {
				innerBuilder = qry.C(fieldName+" "+predicate, value)
			} else {
				innerBuilder.Or(fieldName+" "+predicate, value)
			}

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
