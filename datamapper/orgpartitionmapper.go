package datamapper

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper/gormfixes"
	"github.com/t2wu/betterrest/datamapper/hfetcher"
	"github.com/t2wu/betterrest/datamapper/service"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/betterrest/registry"
	"github.com/t2wu/qry"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

// ---------------------------------------

var (
	onceOrgPartition   sync.Once
	orgPartitionMapper IDataMapper
)

// SetOrganizationMapper allows one to mock OrganizationMapper for testing
func SetOrgPartition(mapper IDataMapper) {
	onceOrgPartition.Do(func() {
		orgPartitionMapper = mapper
	})
}

// SharedOrgPartition creats a singleton of Crud object
func SharedOrgPartition() IDataMapper {
	onceOrgPartition.Do(func() {
		orgPartitionMapper = &OrgPartition{Service: &service.OrgPartition{BaseServiceV2: service.BaseServiceV2{}}}
		// orgPartitionMapper = &DataMapperV2{Service: &service.OrgPartition{BaseServiceV2: service.BaseServiceV2{}}}
	})

	return orgPartitionMapper
}

type OrgPartition struct {
	Service service.IServiceV2
}

// CreateMany creates an instance of this model based on json and store it in db
func (mapper *OrgPartition) CreateMany(db *gorm.DB, modelObjs []mdl.IModel, ep *hook.EndPoint,
	cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {

	modelObjs, err := mapper.Service.HookBeforeCreateMany(db, ep.Who, ep.TypeString, modelObjs)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	roles := make([]userrole.UserRole, len(modelObjs))
	for i := range roles {
		roles[i] = userrole.UserRoleAdmin // has to be admin to create
	}

	oldBefore := registry.ModelRegistry[ep.TypeString].BeforeCreate
	oldAfter := registry.ModelRegistry[ep.TypeString].AfterCreate

	data := hook.Data{Ms: modelObjs, DB: db, Roles: roles, Cargo: cargo}
	initData := hook.InitData{Roles: roles, Ep: ep}

	j := batchOpJobV2{
		serv:         mapper.Service,
		oldmodelObjs: nil,
		modelObjs:    modelObjs,

		oldBefore: oldBefore,
		oldAfter:  oldAfter,

		fetcher: hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:    &data,
		ep:      ep,
	}
	return batchOpCoreV2(j, mapper.Service.CreateOneCore)
}

// CreateOne creates an instance of this model based on json and store it in db
func (mapper *OrgPartition) CreateOne(db *gorm.DB, modelObj mdl.IModel,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	modelObj, err := mapper.Service.HookBeforeCreateOne(db, ep.Who, ep.TypeString, modelObj)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(mdlutil.IBeforeCreate); ok {
		b := "BeforeCreateDB"
		beforeFuncName = &b
	}
	if _, ok := modelObj.(mdlutil.IAfterCreate); ok {
		a := "AfterCreateDB"
		afterFuncName = &a
	}

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Cargo: cargo}
	initData := hook.InitData{Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Ep: ep}

	j := opJobV2{
		serv: mapper.Service,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,

		beforeFuncName: beforeFuncName,
		afterFuncName:  afterFuncName,

		fetcher: hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:    &data,
		ep:      ep,
	}
	return opCoreV2(j, mapper.Service.CreateOneCore)
}

func (mapper *OrgPartition) ReadMany(db *gorm.DB, ep *hook.EndPoint,
	cargo *hook.Cargo) (*MapperRet, []userrole.UserRole, *int, *webrender.RetError) {
	dbClean := db
	db = db.Set("gorm:auto_preload", false)
	db2 := db

	offset, limit, cstart, cstop, _, _, latestn, latestnons, totalcount := urlparam.GetOptions(ep.URLParams)
	if cstart == nil || cstop == nil {
		err := fmt.Errorf("GET /%s needs cstart and cstop parameters", strings.ToLower(ep.TypeString))
		return nil, nil, nil, webrender.NewRetValWithRendererError(err, webrender.NewErrQueryParameter(err))
	}

	rtable := registry.GetTableNameFromTypeString(ep.TypeString)

	if cstart != nil && cstop != nil {
		db = db.Where(rtable+".created_at BETWEEN ? AND ?", time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))
	}

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

	var orderby *string
	order := "desc"
	db, err = constructOrderFieldQueries(db, ep.TypeString, rtable, orderby, &order)
	if err != nil {
		return nil, nil, nil, &webrender.RetError{Error: err}
	}
	db, err = mapper.Service.GetAllQueryContructCore(db, ep.Who, ep.TypeString)
	if err != nil {
		return nil, nil, nil, &webrender.RetError{Error: err}
	}

	var no *int
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
			db = db.Set("gorm:auto_preload", false)

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
		// this one set it to  preload false!
		db, err = qry.Q(db, builder).BuildQuery(registry.NewFromTypeString(ep.TypeString))
		if err != nil {
			return nil, nil, nil, &webrender.RetError{Error: err}
		}
		db = db.Set("gorm:auto_preload", false)
	}

	// Actual query in the following line
	outmodels, err := registry.NewSliceFromDBByTypeString(ep.TypeString, db.Find)
	if err != nil {
		return nil, nil, nil, &webrender.RetError{Error: err}
	}

	// Now need to recursively walks through outmodes and query by dates
	service.RecursivelyQueryAllPeggedModels(db2, outmodels, time.Unix(int64(*cstart), 0), time.Unix(int64(*cstop), 0))

	roles, err := mapper.Service.GetAllRolesCore(db, dbClean, ep.Who, ep.TypeString, outmodels)
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

	// use dbClean cuz it's not chained
	data := hook.Data{Ms: outmodels, DB: dbClean, Roles: roles, Cargo: cargo}
	initData := hook.InitData{Roles: roles, Ep: ep}

	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// Begin deprecated
	if !fetcher.HasAttemptRegisteringHandler() {
		oldGeneric := registry.ModelRegistry[ep.TypeString].AfterCRUPD
		oldSpecific := registry.ModelRegistry[ep.TypeString].AfterRead

		if err := callOldBatch(&data, ep, oldGeneric, oldSpecific); err != nil {
			return nil, nil, nil, &webrender.RetError{Error: err}
		}
	}
	// End deprecated

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
func (mapper *OrgPartition) ReadOne(db *gorm.DB, id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, userrole.UserRole, *webrender.RetError) {

	db = db.Set("gorm:auto_preload", false)

	_, _, cstart, cstop, _, _, _, _, _ := urlparam.GetOptions(ep.URLParams)
	if cstart == nil || cstop == nil {
		err := fmt.Errorf("GET /%s needs cstart and cstop parameters", strings.ToLower(ep.TypeString))
		return nil, userrole.UserRoleAny, webrender.NewRetValWithRendererError(err, webrender.NewErrQueryParameter(err))
	}

	if id == nil || id.UUID.String() == "" {
		// in case it's an empty string
		return nil, userrole.UserRoleInvalid, &webrender.RetError{Error: service.ErrIDEmpty}
	}

	// Yes we actually want to read here
	modelObj, role, err := loadAndCheckErrorBeforeModifyV2(mapper.Service, db, ep.Who, ep.TypeString, nil, id,
		[]userrole.UserRole{userrole.UserRoleAny}, ep.URLParams)
	if err != nil {
		return nil, userrole.UserRoleInvalid, &webrender.RetError{Error: err}
	}

	initData := hook.InitData{Roles: []userrole.UserRole{role}, Ep: ep}
	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// Deprecated
	if !fetcher.HasAttemptRegisteringHandler() {
		// old one
		modelCargo := mdlutil.ModelCargo{Payload: cargo.Payload}
		// After CRUPD hook
		if m, ok := modelObj.(mdlutil.IAfterCRUPD); ok {
			hpdata := mdlutil.HookPointData{DB: db, Who: ep.Who, TypeString: ep.TypeString, Cargo: &modelCargo, Role: &role, URLParams: ep.URLParams}
			if err := m.AfterCRUPDDB(hpdata, mdlutil.CRUPDOpRead); err != nil {
				return nil, 0, &webrender.RetError{Error: err}
			}
		}

		// AfterRead hook
		if m, ok := modelObj.(mdlutil.IAfterRead); ok {
			hpdata := mdlutil.HookPointData{DB: db, Who: ep.Who, TypeString: ep.TypeString, Cargo: &modelCargo, Role: &role, URLParams: ep.URLParams}
			if err := m.AfterReadDB(hpdata); err != nil {
				return nil, 0, &webrender.RetError{Error: err}
			}
		}
		cargo.Payload = modelCargo.Payload
		ret := MapperRet{
			Ms:      []mdl.IModel{modelObj},
			Fetcher: fetcher,
		}

		return &ret, role, nil
	}
	// End deprecated

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
func (mapper *OrgPartition) UpdateOne(db *gorm.DB, modelObj mdl.IModel, id *datatype.UUID,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	oldModelObj, _, err := loadAndCheckErrorBeforeModifyV2(mapper.Service, db, ep.Who, ep.TypeString, modelObj, id,
		[]userrole.UserRole{userrole.UserRoleAdmin}, ep.URLParams)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	// TODO: Huh? How do we do validation here?!
	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(mdlutil.IBeforeUpdate); ok {
		b := "BeforeUpdateDB"
		beforeFuncName = &b
	}
	if _, ok := modelObj.(mdlutil.IAfterUpdate); ok {
		a := "AfterUpdateDB"
		afterFuncName = &a
	}

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Cargo: cargo}
	initData := hook.InitData{Roles: []userrole.UserRole{userrole.UserRoleAdmin}, Ep: ep}

	j := opJobV2{
		serv:        mapper.Service,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,

		beforeFuncName: beforeFuncName,
		afterFuncName:  afterFuncName,

		fetcher: hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:    &data,
		ep:      ep,
	}
	return opCoreV2(j, mapper.Service.UpdateOneCore)
}

// UpdateMany updates multiple mdl
func (mapper *OrgPartition) UpdateMany(db *gorm.DB, modelObjs []mdl.IModel,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
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

	oldModelObjs, roles, err := loadManyAndCheckBeforeModifyV2(mapper.Service, db, ep.Who, ep.TypeString, ids, []userrole.UserRole{userrole.UserRoleAdmin})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	// load and check is not in the same order as modelobj
	oldModelObjs, roles = mapper.sortOldModelAndRolesByIds(oldModelObjs, roles, ids)

	data := hook.Data{Ms: modelObjs, DB: db, Roles: roles, Cargo: cargo}
	initData := hook.InitData{Roles: roles, Ep: ep}

	oldBefore := registry.ModelRegistry[ep.TypeString].BeforeUpdate
	oldAfter := registry.ModelRegistry[ep.TypeString].AfterUpdate
	j := batchOpJobV2{
		serv:         mapper.Service,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,

		oldBefore: oldBefore,
		oldAfter:  oldAfter,

		fetcher: hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:    &data,
		ep:      ep,
	}
	return batchOpCoreV2(j, mapper.Service.UpdateOneCore)
}

// PatchOne updates model based on this json
func (mapper *OrgPartition) PatchOne(db *gorm.DB, jsonPatch []byte,
	id *datatype.UUID, ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	oldModelObj, role, err := loadAndCheckErrorBeforeModifyV2(mapper.Service, db, ep.Who, ep.TypeString, nil, id,
		[]userrole.UserRole{userrole.UserRoleAdmin}, ep.URLParams)
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

	initData := hook.InitData{Roles: []userrole.UserRole{role}, Ep: ep}
	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	// Begin deprecated
	if !fetcher.HasAttemptRegisteringHandler() {
		if m, ok := oldModelObj.(mdlutil.IBeforePatchApply); ok {
			modelCargo := mdlutil.ModelCargo{Payload: cargo.Payload}
			hpdata := mdlutil.HookPointData{DB: db, Who: ep.Who, TypeString: ep.TypeString, Cargo: &modelCargo}
			if err := m.BeforePatchApplyDB(hpdata); err != nil {
				return nil, &webrender.RetError{Error: err}
			}
			cargo.Payload = modelCargo.Payload
		}
	}
	// End deprecated

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

	// TODO: Huh? How do we do validation here?!
	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(mdlutil.IBeforePatch); ok {
		b := "BeforePatchDB"
		beforeFuncName = &b
	}

	if _, ok := modelObj.(mdlutil.IAfterPatch); ok {
		a := "AfterPatchDB"
		afterFuncName = &a
	}

	data.Ms = []mdl.IModel{modelObj} // modify the one already in hook

	j := opJobV2{
		serv:        mapper.Service,
		oldModelObj: oldModelObj,
		modelObj:    modelObj,

		beforeFuncName: beforeFuncName,
		afterFuncName:  afterFuncName,

		fetcher: fetcher,
		data:    &data,
		ep:      ep,
	}
	return opCoreV2(j, mapper.Service.UpdateOneCore)
}

// PatchMany patches multiple mdl
func (mapper *OrgPartition) PatchMany(db *gorm.DB, jsonIDPatches []mdlutil.JSONIDPatch,
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

	oldModelObjs, roles, err := loadManyAndCheckBeforeModifyV2(mapper.Service, db, ep.Who, ep.TypeString, ids, []userrole.UserRole{userrole.UserRoleAdmin})
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

	// Begin Deprecated
	fetcher := hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData)

	if !fetcher.HasAttemptRegisteringHandler() {
		batchCargo := mdlutil.BatchHookCargo{Payload: cargo.Payload}
		// Hookpoint BEFORE BeforeCRUD and BeforePatch
		// This is called BEFORE the actual patch
		beforeApply := registry.ModelRegistry[ep.TypeString].BeforePatchApply
		if beforeApply != nil {
			bhpData := mdlutil.BatchHookPointData{Ms: oldModelObjs, DB: db, Who: ep.Who, TypeString: ep.TypeString,
				Cargo: &batchCargo, Roles: roles}
			if err := beforeApply(bhpData); err != nil {
				return nil, &webrender.RetError{Error: err}
			}
		}
		cargo.Payload = batchCargo.Payload
	}
	// End Deprecated

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
	oldBefore := registry.ModelRegistry[ep.TypeString].BeforePatch
	oldAfter := registry.ModelRegistry[ep.TypeString].AfterPatch
	j := batchOpJobV2{
		serv:         mapper.Service,
		oldmodelObjs: oldModelObjs,
		modelObjs:    modelObjs,

		oldBefore: oldBefore,
		oldAfter:  oldAfter,

		fetcher: fetcher,
		data:    &data,
		ep:      ep,
	}
	return batchOpCoreV2(j, mapper.Service.UpdateOneCore)
}

// DeleteOne delete the model
// TODO: delete the groups associated with this record?
func (mapper *OrgPartition) DeleteOne(db *gorm.DB, id *datatype.UUID, ep *hook.EndPoint,
	cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
	modelObj, role, err := loadAndCheckErrorBeforeModifyV2(mapper.Service, db, ep.Who, ep.TypeString, nil, id, []userrole.UserRole{userrole.UserRoleAdmin}, ep.URLParams)
	if err != nil {
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

	var beforeFuncName, afterFuncName *string
	if _, ok := modelObj.(mdlutil.IBeforeDelete); ok {
		b := "BeforeDeleteDB"
		beforeFuncName = &b
	}
	if _, ok := modelObj.(mdlutil.IAfterDelete); ok {
		a := "AfterDeleteDB"
		afterFuncName = &a
	}

	data := hook.Data{Ms: []mdl.IModel{modelObj}, DB: db, Roles: []userrole.UserRole{role}, Cargo: cargo}
	initData := hook.InitData{Roles: []userrole.UserRole{role}, Ep: ep}

	j := opJobV2{
		serv: mapper.Service,
		// oldModelObj: oldModelObj,
		modelObj: modelObj,

		beforeFuncName: beforeFuncName,
		afterFuncName:  afterFuncName,

		fetcher: hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:    &data,
		ep:      ep,
	}
	return opCoreV2(j, mapper.Service.DeleteOneCore)
}

// DeleteMany deletes multiple mdl
func (mapper *OrgPartition) DeleteMany(db *gorm.DB, modelObjs []mdl.IModel,
	ep *hook.EndPoint, cargo *hook.Cargo) (*MapperRet, *webrender.RetError) {
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

	modelObjs, roles, err := loadManyAndCheckBeforeModifyV2(mapper.Service, db, ep.Who, ep.TypeString, ids, []userrole.UserRole{userrole.UserRoleAdmin})
	if err != nil {
		return nil, &webrender.RetError{Error: err}
	}

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

	oldBefore := registry.ModelRegistry[ep.TypeString].BeforeDelete
	oldAfter := registry.ModelRegistry[ep.TypeString].AfterDelete

	j := batchOpJobV2{
		serv:      mapper.Service,
		modelObjs: modelObjs,

		oldBefore: oldBefore,
		oldAfter:  oldAfter,

		fetcher: hfetcher.NewHandlerFetcher(registry.ModelRegistry[ep.TypeString].HandlerMap, &initData),
		data:    &data,
		ep:      ep,
	}
	return batchOpCoreV2(j, mapper.Service.DeleteOneCore)
}

// ----------------------------------------------------------------------------------------
func (mapper *OrgPartition) sortOldModelAndRolesByIds(oldModelObjs []mdl.IModel, roles []userrole.UserRole, ids []*datatype.UUID) ([]mdl.IModel, []userrole.UserRole) {
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
