package datamapper

import (
	"log"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/rest"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/utils/transact"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/betterrest/registry"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

// ----------------------------------------------------------------------------------------------

// Who is the information about the client or the user
type WhoMock struct {
	Oid *datatype.UUID // ownerid
}

func (w *WhoMock) GetUserID() *datatype.UUID {
	return w.Oid
}

type Car struct {
	mdl.BaseModel

	Name string `json:"name"`

	Ownerships []mdlutil.OwnershipModelWithIDBase `gorm:"PRELOAD:false" json:"-" betterrest:"ownership"`
}

// ----------------------------------------------------------------------------------------------

// These have to be out there because out-style in-model hooks are not called in a single
// object, before and after can be different.

var guardAPIEntryCalled bool
var guardAPIEntryWho mdlutil.UserIDFetchable
var guardAPIEntryHTTP mdlutil.HTTP

var beforeCUPDDBCalled bool
var beforeCUPDDBHpdata mdlutil.HookPointData
var beforeCUPDDBOp mdlutil.CRUPDOp

var beforeCreateDBCalled bool
var beforeCreateDBHpdata mdlutil.HookPointData

var beforeReadDBCalled bool
var beforeReadDBHpdata mdlutil.HookPointData

var beforeUpdateDBCalled bool
var beforeUpdateDBHpdata mdlutil.HookPointData

var beforePatchDBCalled bool
var beforePatchDBHpdata mdlutil.HookPointData

var beforeDeleteDBCalled bool
var beforeDeleteDBHpdata mdlutil.HookPointData

var afterCRUPDDBCalled bool
var afterCRUPDDBHpdata mdlutil.HookPointData
var afterCRUPDDBOp mdlutil.CRUPDOp

var afterCreateDBCalled bool
var afterCreateDBHpdata mdlutil.HookPointData

var afterReadDBCalled bool
var afterReadDBHpdata mdlutil.HookPointData

var afterUpdateDBCalled bool
var afterUpdateDBHpdata mdlutil.HookPointData

var afterPatchDBCalled bool
var afterPatchDBHpdata mdlutil.HookPointData

var afterDeleteDBCalled bool
var afterDeleteDBHpdata mdlutil.HookPointData

func resetGlobals() {
	guardAPIEntryCalled = false
	guardAPIEntryWho = nil
	guardAPIEntryHTTP = mdlutil.HTTP{}

	beforeCUPDDBCalled = false
	beforeCUPDDBHpdata = mdlutil.HookPointData{}
	beforeCUPDDBOp = mdlutil.CRUPDOpOther

	beforeCreateDBCalled = false
	beforeCreateDBHpdata = mdlutil.HookPointData{}

	beforeReadDBCalled = false
	beforeReadDBHpdata = mdlutil.HookPointData{}

	beforeUpdateDBCalled = false
	beforeUpdateDBHpdata = mdlutil.HookPointData{}

	beforePatchDBCalled = false
	beforePatchDBHpdata = mdlutil.HookPointData{}

	beforeDeleteDBCalled = false
	beforeDeleteDBHpdata = mdlutil.HookPointData{}

	afterCRUPDDBCalled = false
	afterCRUPDDBHpdata = mdlutil.HookPointData{}
	afterCRUPDDBOp = mdlutil.CRUPDOpOther

	afterCreateDBCalled = false
	afterCreateDBHpdata = mdlutil.HookPointData{}

	afterReadDBCalled = false
	afterReadDBHpdata = mdlutil.HookPointData{}

	afterUpdateDBCalled = false
	afterUpdateDBHpdata = mdlutil.HookPointData{}

	afterPatchDBCalled = false
	afterPatchDBHpdata = mdlutil.HookPointData{}

	afterDeleteDBCalled = false
	afterDeleteDBHpdata = mdlutil.HookPointData{}

}

type CarWithCallbacks struct {
	mdl.BaseModel

	Name string `json:"name"`

	Ownerships []mdlutil.OwnershipModelWithIDBase `gorm:"PRELOAD:false" json:"-" betterrest:"ownership"`
}

func (CarWithCallbacks) TableName() string {
	return "car"
}

// GuardAPIEntry guards denies api call based on scope
func (c *CarWithCallbacks) GuardAPIEntry(who mdlutil.UserIDFetchable, http mdlutil.HTTP) bool {
	guardAPIEntryCalled = true
	guardAPIEntryWho = who
	guardAPIEntryHTTP = http
	return true // true to pass
}

func (c *CarWithCallbacks) BeforeCUPDDB(hpdata mdlutil.HookPointData, op mdlutil.CRUPDOp) error {
	beforeCUPDDBCalled = true
	beforeCUPDDBHpdata = hpdata
	beforeCUPDDBOp = op
	return nil
}

func (c *CarWithCallbacks) BeforeCreateDB(hpdata mdlutil.HookPointData) error {
	beforeCreateDBCalled = true
	beforeCreateDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) BeforeReadDB(hpdata mdlutil.HookPointData) error {
	beforeReadDBCalled = true
	beforeReadDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) BeforeUpdateDB(hpdata mdlutil.HookPointData) error {
	beforeUpdateDBCalled = true
	beforeUpdateDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) BeforePatchDB(hpdata mdlutil.HookPointData) error {
	beforePatchDBCalled = true
	beforePatchDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) BeforeDeleteDB(hpdata mdlutil.HookPointData) error {
	beforeDeleteDBCalled = true
	beforeDeleteDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) AfterCRUPDDB(hpdata mdlutil.HookPointData, op mdlutil.CRUPDOp) error {
	afterCRUPDDBCalled = true
	afterCRUPDDBHpdata = hpdata
	afterCRUPDDBOp = op

	return nil
}

func (c *CarWithCallbacks) AfterCreateDB(hpdata mdlutil.HookPointData) error {
	afterCreateDBCalled = true
	afterCreateDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) AfterReadDB(hpdata mdlutil.HookPointData) error {
	afterReadDBCalled = true
	afterReadDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) AfterUpdateDB(hpdata mdlutil.HookPointData) error {
	afterUpdateDBCalled = true
	afterUpdateDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) AfterPatchDB(hpdata mdlutil.HookPointData) error {
	afterPatchDBCalled = true
	afterPatchDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) AfterDeleteDB(hpdata mdlutil.HookPointData) error {
	afterDeleteDBCalled = true
	afterDeleteDBHpdata = hpdata
	return nil
}

// ----------------------------------------------------------------------------------------------

func createBatchHookPoint(called *bool, bhpDataCalled *mdlutil.BatchHookPointData, opCalled *mdlutil.CRUPDOp) func(bhpData mdlutil.BatchHookPointData, op mdlutil.CRUPDOp) error {
	return func(bhpData mdlutil.BatchHookPointData, op mdlutil.CRUPDOp) error {
		*called = true

		deepCopyBHPData(&bhpData, bhpDataCalled)

		*opCalled = op
		return nil
	}
}

func createBatchSingleMethodHookPoint(called *bool, bhpDataCalled *mdlutil.BatchHookPointData) func(bhpData mdlutil.BatchHookPointData) error {
	return func(bhpData mdlutil.BatchHookPointData) error {
		*called = true

		deepCopyBHPData(&bhpData, bhpDataCalled)
		return nil
	}
}

// ----------------------------------------------------------------------------------------------

type CarControllerWithoutCallbacks struct {
}

func (c *CarControllerWithoutCallbacks) Init(data *hook.InitData, args ...interface{}) {
}

type CarHandlerJBT struct {
	// From init
	who        mdlutil.UserIDFetchable
	typeString string
	roles      []userrole.UserRole
	urlParams  map[urlparam.Param]interface{}
	info       *hook.EndPoint

	guardAPIEntryCalled bool
	guardAPIEntryData   *hook.Data
	guardAPIEntryInfo   *hook.EndPoint

	beforeApplyCalled bool
	beforeApplyData   *hook.Data
	beforeApplyInfo   *hook.EndPoint

	beforeCalled bool
	beforeData   *hook.Data
	beforeInfo   *hook.EndPoint

	afterCalled bool
	afterData   *hook.Data
	afterInfo   *hook.EndPoint
}

func (c *CarHandlerJBT) Init(data *hook.InitData, args ...interface{}) {
	c.who = data.Ep.Who
	c.typeString = data.Ep.TypeString
	c.roles = data.Roles
	c.urlParams = data.Ep.URLParams
	c.info = data.Ep
}

func (c *CarHandlerJBT) GuardAPIEntry(data *hook.Data, info *hook.EndPoint) *webrender.GuardRetVal {
	c.guardAPIEntryCalled = true
	c.guardAPIEntryData = &hook.Data{}
	deepCopyData(data, c.guardAPIEntryData)
	c.guardAPIEntryInfo = info
	return nil
}

func (c *CarHandlerJBT) BeforeApply(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	c.beforeApplyCalled = true
	c.beforeApplyData = &hook.Data{}
	deepCopyData(data, c.beforeApplyData)
	c.beforeApplyInfo = info
	return nil
}

func (c *CarHandlerJBT) Before(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	c.beforeCalled = true
	c.beforeData = &hook.Data{}
	deepCopyData(data, c.beforeData)
	c.beforeInfo = info
	return nil
}

func (c *CarHandlerJBT) After(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	c.afterCalled = true
	c.afterData = &hook.Data{}
	deepCopyData(data, c.afterData)
	c.afterInfo = info
	return nil
}

// ----------------------------------------------------------------------------------------------

type TestBaseMapperCreateSuite struct {
	suite.Suite
	db         *gorm.DB
	mock       sqlmock.Sqlmock
	who        mdlutil.UserIDFetchable
	typeString string
}

func (suite *TestBaseMapperCreateSuite) SetupTest() {
	sqldb, mock, _ := sqlmock.New() // db, mock, error. We're testing lifecycle here
	suite.db, _ = gorm.Open("postgres", sqldb)
	// suite.db.LogMode(true)
	suite.db.SingularTable(true)
	suite.mock = mock
	suite.who = &WhoMock{Oid: datatype.NewUUID()} // userid
	suite.typeString = "cars"

	// clear registry
	delete(registry.ModelRegistry, "cars")

	resetGlobals()
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestBaseMapperCreateSuite) TestCreateOne_WhenGiven_GotCar() {
	carID := datatype.NewUUID()
	carName := "DSM"
	var modelObj mdl.IModel = &Car{BaseModel: mdl.BaseModel{ID: carID}, Name: carName}

	suite.mock.ExpectBegin()
	stmt := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt3 := `SELECT * FROM "car"  WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	mapper := SharedOwnershipMapper()

	var retVal *MapperRet
	ep := hook.EndPoint{
		Op:          rest.OpCreate,
		Cardinality: rest.CardinalityOne,
		TypeString:  suite.typeString,
		URLParams:   options,
		Who:         suite.who,
	}
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		if retVal, retErr = mapper.CreateOne(tx, modelObj, &ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	if car, ok := retVal.Ms[0].(*Car); assert.True(suite.T(), ok) {
		assert.Equal(suite.T(), carName, car.Name)
		assert.Equal(suite.T(), carID, car.ID)
	}
}

func (suite *TestBaseMapperCreateSuite) TestCreateOne_WhenNoController_CallRelevantOldCallbacks() {
	carID := datatype.NewUUID()
	carName := "DSM"
	var modelObj mdl.IModel = &CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID}, Name: carName}

	suite.mock.ExpectBegin()
	stmt := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt3 := `SELECT * FROM "car"  WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	mapper := SharedOwnershipMapper()

	var retVal *MapperRet
	var tx2 *gorm.DB
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		ep := hook.EndPoint{
			Op:          rest.OpCreate,
			Cardinality: rest.CardinalityOne,
			TypeString:  suite.typeString,
			URLParams:   options,
			Who:         suite.who,
		}
		if retVal, retErr = mapper.CreateOne(tx, modelObj, &ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	role := userrole.UserRoleAdmin
	hpdata := mdlutil.HookPointData{DB: tx2, Who: suite.who, TypeString: suite.typeString,
		Cargo: &mdlutil.ModelCargo{Payload: cargo.Payload}, Role: &role, URLParams: options}

	if _, ok := retVal.Ms[0].(*CarWithCallbacks); assert.True(suite.T(), ok) {
		assert.False(suite.T(), guardAPIEntryCalled) // not called when going through mapper
		// Create has no before callback since haven't been read
		if assert.True(suite.T(), beforeCUPDDBCalled) {
			assert.Equal(suite.T(), beforeCUPDDBOp, mdlutil.CRUPDOpCreate)
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &beforeCUPDDBHpdata))
		}

		if assert.True(suite.T(), beforeCreateDBCalled) {
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &beforeCreateDBHpdata))
		}

		if assert.True(suite.T(), afterCRUPDDBCalled) {
			assert.Equal(suite.T(), afterCRUPDDBOp, mdlutil.CRUPDOpCreate)
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &afterCRUPDDBHpdata))
		}
		if assert.True(suite.T(), afterCreateDBCalled) {
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &afterCRUPDDBHpdata))
		}
	}
}

func (suite *TestBaseMapperCreateSuite) TestCreateOne_WhenHavingController_NotCallOldCallbacks() {
	carID := datatype.NewUUID()
	carName := "DSM"
	var modelObj mdl.IModel = &CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID}, Name: carName}

	suite.mock.ExpectBegin()
	stmt := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt3 := `SELECT * FROM "car"  WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	hdlr := CarControllerWithoutCallbacks{}
	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt).Hook(&hdlr, "CRUPD")

	mapper := SharedOwnershipMapper()

	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		ep := hook.EndPoint{
			Op:          rest.OpCreate,
			Cardinality: rest.CardinalityOne,
			TypeString:  suite.typeString,
			URLParams:   options,
			Who:         suite.who,
		}
		if retVal, retErr = mapper.CreateOne(tx, modelObj, &ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	if _, ok := modelObj.(*CarWithCallbacks); assert.True(suite.T(), ok) {
		// None of the model callback should be called when there is hook
		assert.False(suite.T(), guardAPIEntryCalled) // not called when going through mapper
		assert.False(suite.T(), beforeCUPDDBCalled)
		assert.False(suite.T(), beforeReadDBCalled)
		assert.False(suite.T(), afterCRUPDDBCalled)
		assert.False(suite.T(), afterReadDBCalled)
	}

	if _, ok := retVal.Ms[0].(*CarWithCallbacks); assert.True(suite.T(), ok) {
		// None of the model callback should be called when there is hook
		assert.False(suite.T(), guardAPIEntryCalled) // not called when going through mapper
		assert.False(suite.T(), beforeCUPDDBCalled)
		assert.False(suite.T(), beforeReadDBCalled)
		assert.False(suite.T(), afterCRUPDDBCalled)
		assert.False(suite.T(), afterReadDBCalled)
	}
}

func (suite *TestBaseMapperCreateSuite) TestCreateOne_WhenHavingController_CallRelevantControllerCallbacks() {
	carID := datatype.NewUUID()
	carName := "DSM"
	var modelObj mdl.IModel = &CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID}, Name: carName}

	suite.mock.ExpectBegin()
	stmt := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt3 := `SELECT * FROM "car"  WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt).Hook(&CarHandlerJBT{}, "CRUPD")

	mapper := SharedOwnershipMapper()

	// var modelObj2 mdl.IModel
	var tx2 *gorm.DB
	var retVal *MapperRet
	ep := hook.EndPoint{
		Op:          rest.OpCreate,
		Cardinality: rest.CardinalityOne,
		TypeString:  suite.typeString,
		URLParams:   options,
		Who:         suite.who,
	}
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		if retVal, retErr = mapper.CreateOne(tx, modelObj, &ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	role := userrole.UserRoleAdmin
	data := hook.Data{Ms: []mdl.IModel{&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID}, Name: carName}}, DB: tx2,
		Roles: []userrole.UserRole{role}, Cargo: &cargo}

	ctrls := retVal.Fetcher.GetAllInstantiatedHanders()
	if !assert.Len(suite.T(), ctrls, 1) {
		return
	}

	hdlr, ok := ctrls[0].(*CarHandlerJBT)
	if !assert.True(suite.T(), ok) {
		return
	}

	assert.False(suite.T(), hdlr.guardAPIEntryCalled) // Not called when going through mapper (or lifecycle for that matter)
	assert.False(suite.T(), hdlr.beforeApplyCalled)   // not patch, not called

	if assert.True(suite.T(), hdlr.beforeCalled) {
		assert.Condition(suite.T(), dataComparison(&data, hdlr.beforeData))
		assert.Equal(suite.T(), ep, *hdlr.beforeInfo)
	}

	// After hook has made some modification to data (this is harder to test)
	// data.Ms[0].(*Car).Name = data.Ms[0].(*Car).Name + "-after"

	if assert.True(suite.T(), hdlr.afterCalled) {
		assert.Condition(suite.T(), dataComparison(&data, hdlr.afterData))
		assert.Equal(suite.T(), ep, *hdlr.afterInfo)
	}
}

func (suite *TestBaseMapperCreateSuite) TestCreateMany_WhenGiven_GotCars() {
	carID1 := datatype.NewUUID()
	carName1 := "DSM"
	carID2 := datatype.NewUUID()
	carName2 := "DSM4Life"
	carID3 := datatype.NewUUID()
	carName3 := "Eclipse"
	modelObjs := []mdl.IModel{
		&Car{BaseModel: mdl.BaseModel{ID: carID1}, Name: carName1},
		&Car{BaseModel: mdl.BaseModel{ID: carID2}, Name: carName2},
		&Car{BaseModel: mdl.BaseModel{ID: carID3}, Name: carName3},
	}

	suite.mock.ExpectBegin()
	// Gorm v1 insert 3 times
	// (Also Gorm v2 support modifying returning: https://gorm.io/docs/update.html#Returning-Data-From-Modified-Rows
	stmt1 := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	stmt3 := `SELECT * FROM "car" WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))
	// actually it might not be possible to fetch the id gorm gives
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatype.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatype.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatype.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	var retVal *MapperRet
	ep := hook.EndPoint{
		Op:          rest.OpCreate,
		Cardinality: rest.CardinalityMany,
		TypeString:  suite.typeString,
		URLParams:   options,
		Who:         suite.who,
	}
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		mapper := SharedOwnershipMapper()
		retVal, retErr = mapper.CreateMany(tx, modelObjs, &ep, &cargo)
		return retErr
	}, "lifecycle.CreateMany")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	if assert.Len(suite.T(), retVal.Ms, 3) {
		assert.Equal(suite.T(), carID1.String(), retVal.Ms[0].GetID().String())
		assert.Equal(suite.T(), carID2.String(), retVal.Ms[1].GetID().String())
		assert.Equal(suite.T(), carID3.String(), retVal.Ms[2].GetID().String())
		assert.Equal(suite.T(), carName1, retVal.Ms[0].(*Car).Name)
		assert.Equal(suite.T(), carName2, retVal.Ms[1].(*Car).Name)
		assert.Equal(suite.T(), carName3, retVal.Ms[2].(*Car).Name)
	}
}

func (suite *TestBaseMapperCreateSuite) TestCreateMany_WhenNoController_CallRelevantOldCallbacks() {
	carID1 := datatype.NewUUID()
	carName1 := "DSM"
	carID2 := datatype.NewUUID()
	carName2 := "DSM4Life"
	carID3 := datatype.NewUUID()
	carName3 := "Eclipse"
	modelObjs := []mdl.IModel{
		&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID1}, Name: carName1},
		&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID2}, Name: carName2},
		&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID3}, Name: carName3},
	}

	suite.mock.ExpectBegin()
	// Gorm v1 insert 3 times
	// (Also Gorm v2 support modifying returning: https://gorm.io/docs/update.html#Returning-Data-From-Modified-Rows
	stmt1 := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	stmt3 := `SELECT * FROM "car" WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))
	// actually it might not be possible to fetch the id gorm gives
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatype.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatype.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatype.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	var beforeCalled bool
	var beforeData mdlutil.BatchHookPointData
	var beforeOp mdlutil.CRUPDOp
	before := createBatchHookPoint(&beforeCalled, &beforeData, &beforeOp)

	var afterCalled bool
	var afterData mdlutil.BatchHookPointData
	var afterOp mdlutil.CRUPDOp
	after := createBatchHookPoint(&afterCalled, &afterData, &afterOp)

	var beforeCreateCalled bool
	var beforeCreateData mdlutil.BatchHookPointData
	beforeCreate := createBatchSingleMethodHookPoint(&beforeCreateCalled, &beforeCreateData)

	var afterCreateCalled bool
	var afterCreateData mdlutil.BatchHookPointData
	afterCreate := createBatchSingleMethodHookPoint(&afterCreateCalled, &afterCreateData)

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).
		BatchCRUPDHooks(before, after).BatchCreateHooks(beforeCreate, afterCreate)

	var tx2 *gorm.DB
	ep := hook.EndPoint{
		Op:          rest.OpCreate,
		Cardinality: rest.CardinalityMany,
		TypeString:  suite.typeString,
		URLParams:   options,
		Who:         suite.who,
	}
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		_, retErr = mapper.CreateMany(tx2, modelObjs, &ep, &cargo)
		return retErr
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}
	roles := []userrole.UserRole{userrole.UserRoleAdmin, userrole.UserRoleAdmin, userrole.UserRoleAdmin}

	// Expected
	expectedData := mdlutil.BatchHookPointData{
		Ms: []mdl.IModel{
			&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID1}, Name: carName1},
			&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID2}, Name: carName2},
			&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID3}, Name: carName3},
		},
		DB: tx2, Who: suite.who, TypeString: suite.typeString, Roles: roles, URLParams: options,
		Cargo: &mdlutil.BatchHookCargo{Payload: cargo.Payload},
	}

	if assert.True(suite.T(), beforeCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &beforeData))
		// It won't be null because it ain't pointers
		// assert.Nil(suite.T(), beforeData.Ms[0].GetCreatedAt())
		// assert.Nil(suite.T(), beforeData.Ms[0].GetUpdatedAt())
		assert.Equal(suite.T(), beforeOp, mdlutil.CRUPDOpCreate)
	}

	if assert.True(suite.T(), beforeCreateCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &beforeCreateData))
		// 	assert.Nil(suite.T(), beforeCreateData.Ms[0].GetCreatedAt())
		// 	assert.Nil(suite.T(), beforeCreateData.Ms[0].GetUpdatedAt())
	}

	if assert.True(suite.T(), afterCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &afterData))
		assert.Equal(suite.T(), afterOp, mdlutil.CRUPDOpCreate)
		// assert.NotNil(suite.T(), afterData.Ms[0].GetCreatedAt())
		// assert.NotNil(suite.T(), afterData.Ms[0].GetUpdatedAt())
	}

	if assert.True(suite.T(), afterCreateCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &afterCreateData))
		// assert.NotNil(suite.T(), afterData.Ms[0].GetCreatedAt())
		// assert.NotNil(suite.T(), afterData.Ms[0].GetUpdatedAt())
	}
}

func (suite *TestBaseMapperCreateSuite) TestCreateMany_WhenHavingController_NotCallOldCallbacks() {
	carID1 := datatype.NewUUID()
	carName1 := "DSM"
	carID2 := datatype.NewUUID()
	carName2 := "DSM4Life"
	carID3 := datatype.NewUUID()
	carName3 := "Eclipse"
	modelObjs := []mdl.IModel{
		&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID1}, Name: carName1},
		&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID2}, Name: carName2},
		&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID3}, Name: carName3},
	}

	suite.mock.ExpectBegin()
	// Gorm v1 insert 3 times
	// (Also Gorm v2 support modifying returning: https://gorm.io/docs/update.html#Returning-Data-From-Modified-Rows
	stmt1 := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	stmt3 := `SELECT * FROM "car" WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))
	// actually it might not be possible to fetch the id gorm gives
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatype.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatype.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatype.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	var beforeCalled bool
	var beforeData mdlutil.BatchHookPointData
	var beforeOp mdlutil.CRUPDOp
	before := createBatchHookPoint(&beforeCalled, &beforeData, &beforeOp)

	var afterCalled bool
	var afterData mdlutil.BatchHookPointData
	var afterOp mdlutil.CRUPDOp
	after := createBatchHookPoint(&afterCalled, &afterData, &afterOp)

	var beforeCreateCalled bool
	var beforeCreateData mdlutil.BatchHookPointData
	beforeCreate := createBatchSingleMethodHookPoint(&beforeCreateCalled, &beforeCreateData)

	var afterCreateCalled bool
	var afterCreateData mdlutil.BatchHookPointData
	afterCreate := createBatchSingleMethodHookPoint(&afterCreateCalled, &afterCreateData)

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).
		BatchCRUPDHooks(before, after).BatchCreateHooks(beforeCreate, afterCreate).Hook(&CarControllerWithoutCallbacks{}, "CRUPD")

	ep := hook.EndPoint{
		Op:          rest.OpCreate,
		Cardinality: rest.CardinalityMany,
		TypeString:  suite.typeString,
		URLParams:   options,
		Who:         suite.who,
	}
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		mapper := SharedOwnershipMapper()
		_, retErr = mapper.CreateMany(tx, modelObjs, &ep, &cargo)
		return retErr
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	assert.False(suite.T(), beforeCalled)
	assert.False(suite.T(), afterCalled)
	assert.False(suite.T(), beforeCreateCalled)
	assert.False(suite.T(), afterCreateCalled)
}

func (suite *TestBaseMapperCreateSuite) TestCreateMany_WhenHavingController_CallRelevantControllerCallbacks() {
	carID1 := datatype.NewUUID()
	carName1 := "DSM"
	carID2 := datatype.NewUUID()
	carName2 := "DSM4Life"
	carID3 := datatype.NewUUID()
	carName3 := "Eclipse"
	modelObjs := []mdl.IModel{
		&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID1}, Name: carName1},
		&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID2}, Name: carName2},
		&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID3}, Name: carName3},
	}

	suite.mock.ExpectBegin()
	// Gorm v1 insert 3 times
	// (Also Gorm v2 support modifying returning: https://gorm.io/docs/update.html#Returning-Data-From-Modified-Rows
	stmt1 := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	stmt3 := `SELECT * FROM "car" WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))
	// actually it might not be possible to fetch the id gorm gives
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatype.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatype.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatype.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).Hook(&CarHandlerJBT{}, "CRUPD")

	var tx2 *gorm.DB
	var retVal *MapperRet
	ep := hook.EndPoint{
		Op:          rest.OpCreate,
		Cardinality: rest.CardinalityMany,
		TypeString:  suite.typeString,
		URLParams:   options,
		Who:         suite.who,
	}
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		mapper := SharedOwnershipMapper()
		tx2 = tx
		retVal, retErr = mapper.CreateMany(tx2, modelObjs, &ep, &cargo)
		return retErr
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	roles := []userrole.UserRole{userrole.UserRoleAdmin, userrole.UserRoleAdmin, userrole.UserRoleAdmin}
	data := hook.Data{Ms: []mdl.IModel{
		&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID1}, Name: carName1},
		&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID2}, Name: carName2},
		&CarWithCallbacks{BaseModel: mdl.BaseModel{ID: carID3}, Name: carName3},
	}, DB: tx2, Roles: roles, Cargo: &cargo}

	ctrls := retVal.Fetcher.GetAllInstantiatedHanders()
	if !assert.Len(suite.T(), ctrls, 1) { //testthis
		return
	}

	hdlr, ok := ctrls[0].(*CarHandlerJBT)
	if !assert.True(suite.T(), ok) {
		return
	}

	assert.False(suite.T(), hdlr.guardAPIEntryCalled) // not called when call createMany directly
	assert.False(suite.T(), hdlr.beforeApplyCalled)
	if assert.True(suite.T(), hdlr.beforeCalled) {
		assert.Condition(suite.T(), dataComparison(&data, hdlr.beforeData))
		assert.Equal(suite.T(), ep.Op, hdlr.beforeInfo.Op)
		assert.Equal(suite.T(), ep.Cardinality, hdlr.beforeInfo.Cardinality)
	}
	if assert.True(suite.T(), hdlr.afterCalled) {
		assert.Condition(suite.T(), dataComparison(&data, hdlr.afterData))
		assert.Equal(suite.T(), ep.Op, hdlr.afterInfo.Op)
		assert.Equal(suite.T(), ep.Cardinality, hdlr.afterInfo.Cardinality)
	}
}

func dataComparison(expected *hook.Data, actual *hook.Data) func() (success bool) {
	return func() (success bool) {
		if expected.DB != actual.DB {
			log.Println("dataComparison 1")
			return false
		}

		if (expected.Cargo.Payload != nil && actual.Cargo.Payload == nil) ||
			(expected.Cargo.Payload == nil && actual.Cargo.Payload != nil) {
			log.Println("dataComparison 3")
			return false
		}

		if expected.Cargo.Payload != nil && actual.Cargo.Payload != nil &&
			expected.Cargo != actual.Cargo {
			log.Println("dataComparison 4")
			return false
		}

		if len(expected.Ms) != len(actual.Ms) {
			log.Println("dataComparison 5")
			return false
		}

		// for i := 0; i < len(expected.Ms); i++ {
		// 	if !assert.ObjectsAreEqual(expected.Ms[i], actual.Ms[i]) {
		// 		log.Println("dataComparison 6")
		// 		return false
		// 	}
		// }
		for i := 0; i < len(expected.Ms); i++ {
			// I think it look the same and yet it failed
			// if !assert.ObjectsAreEqual(expected.Ms[i], actual.Ms[i]) {
			// 	return false
			// }
			if c, ok := expected.Ms[i].(*Car); ok {
				if c.ID.String() != actual.Ms[i].(*Car).ID.String() {
					log.Println(".............5.1")
					return false
				}
				if c.Name != actual.Ms[i].(*Car).Name {
					log.Println(".............5.2")
					return false
				}
			}
			if c, ok := expected.Ms[i].(*CarWithCallbacks); ok {
				if c.ID.String() != actual.Ms[i].(*CarWithCallbacks).ID.String() {
					log.Println(".............5.3")
					return false
				}
				if c.Name != actual.Ms[i].(*CarWithCallbacks).Name {
					log.Println(".............5.4")
					return false
				}
			}
		}

		if len(expected.Roles) != len(actual.Roles) {
			log.Println("dataComparison 8")
			return false
		}

		for i := 0; i < len(expected.Roles); i++ {
			if expected.Roles[i] != actual.Roles[i] {
				log.Println("dataComparison 9")
				return false
			}
		}

		return true
	}
}

func dataComparisonNoDB(expected *hook.Data, actual *hook.Data) func() (success bool) {
	return func() (success bool) {
		if (expected.Cargo.Payload != nil && actual.Cargo.Payload == nil) ||
			(expected.Cargo.Payload == nil && actual.Cargo.Payload != nil) {
			log.Println("dataComparison 3")
			return false
		}

		if expected.Cargo.Payload != nil && actual.Cargo.Payload != nil &&
			expected.Cargo != actual.Cargo {
			log.Println("dataComparison 4")
			return false
		}

		if len(expected.Ms) != len(actual.Ms) {
			log.Println("dataComparison 5")
			return false
		}

		// for i := 0; i < len(expected.Ms); i++ {
		// 	if !assert.ObjectsAreEqual(expected.Ms[i], actual.Ms[i]) {
		// 		log.Println("dataComparison 6")
		// 		return false
		// 	}
		// }
		for i := 0; i < len(expected.Ms); i++ {
			// I think it look the same and yet it failed
			// if !assert.ObjectsAreEqual(expected.Ms[i], actual.Ms[i]) {
			// 	return false
			// }
			if c, ok := expected.Ms[i].(*Car); ok {
				if c.ID.String() != actual.Ms[i].(*Car).ID.String() {
					log.Println(".............5.1")
					return false
				}
				if c.Name != actual.Ms[i].(*Car).Name {
					log.Println(".............5.2")
					return false
				}
			}
			if c, ok := expected.Ms[i].(*CarWithCallbacks); ok {
				if c.ID.String() != actual.Ms[i].(*CarWithCallbacks).ID.String() {
					log.Println(".............5.3")
					return false
				}
				if c.Name != actual.Ms[i].(*CarWithCallbacks).Name {
					log.Println(".............5.4")
					return false
				}
			}
		}

		// if expected.TypeString != actual.TypeString {
		// 	log.Println("dataComparison 7")
		// 	return false
		// }

		if len(expected.Roles) != len(actual.Roles) {
			log.Println("dataComparison 8")
			return false
		}

		for i := 0; i < len(expected.Roles); i++ {
			if expected.Roles[i] != actual.Roles[i] {
				log.Println("dataComparison 9")
				return false
			}
		}

		// if len(expected.URLParams) != 0 && len(actual.URLParams) != 0 {
		// 	if !assert.ObjectsAreEqualValues(expected.URLParams, actual.URLParams) {
		// 		log.Println("dataComparison 10")
		// 		return false
		// 	}
		// }

		return true
	}
}

func hpDataComparison(expected *mdlutil.HookPointData, actual *mdlutil.HookPointData) func() (success bool) {
	return func() (success bool) {
		if expected.DB != actual.DB {
			return false
		}

		if expected.Who != actual.Who {
			log.Println("return false 2")
			return false
		}

		if (expected.Cargo.Payload != nil && actual.Cargo.Payload == nil) ||
			(expected.Cargo.Payload == nil && actual.Cargo.Payload != nil) {
			log.Println("return false 3")
			return false
		}

		if expected.Cargo.Payload != nil && actual.Cargo.Payload != nil &&
			expected.Cargo != actual.Cargo {
			log.Println("return false 4")
			return false
		}

		if expected.TypeString != actual.TypeString {
			log.Println("return false 5")
			return false
		}

		if *expected.Role != *actual.Role {
			log.Println("return false 6")
			return false
		}

		if len(expected.URLParams) != 0 && len(actual.URLParams) != 0 {
			if !assert.ObjectsAreEqualValues(expected.URLParams, actual.URLParams) {
				log.Println("return false 7")
				return false
			}
		}

		return true
	}
}

// For delete calls, since unscope is called
func hpDataComparisonNoDB(expected *mdlutil.HookPointData, actual *mdlutil.HookPointData) func() (success bool) {
	return func() (success bool) {
		if expected.Who != actual.Who {
			log.Println("return false 2")
			return false
		}

		if (expected.Cargo.Payload != nil && actual.Cargo.Payload == nil) ||
			(expected.Cargo.Payload == nil && actual.Cargo.Payload != nil) {
			log.Println("return false 3")
			return false
		}

		if expected.Cargo.Payload != nil && actual.Cargo.Payload != nil &&
			expected.Cargo != actual.Cargo {
			log.Println("return false 4")
			return false
		}

		if expected.TypeString != actual.TypeString {
			log.Println("return false 5")
			return false
		}

		if *expected.Role != *actual.Role {
			log.Println("return false 6")
			return false
		}

		if len(expected.URLParams) != 0 && len(actual.URLParams) != 0 {
			if !assert.ObjectsAreEqualValues(expected.URLParams, actual.URLParams) {
				log.Println("return false 7")
				return false
			}
		}

		return true
	}
}

func bhpDataComparison(expected *mdlutil.BatchHookPointData, actual *mdlutil.BatchHookPointData) func() (success bool) {
	return func() (success bool) {
		if expected.DB != actual.DB {
			log.Println(".............1", actual.DB)
			return false
		}

		if expected.Who != actual.Who {
			log.Println(".............2")
			return false
		}

		if (expected.Cargo.Payload != nil && actual.Cargo.Payload == nil) ||
			(expected.Cargo.Payload == nil && actual.Cargo.Payload != nil) {
			log.Println(".............3")
			return false
		}

		if expected.Cargo.Payload != nil && actual.Cargo.Payload != nil &&
			expected.Cargo != actual.Cargo {
			log.Println(".............4")
			return false
		}

		if len(expected.Ms) != len(actual.Ms) {
			log.Println(".............5")
			return false
		}

		for i := 0; i < len(expected.Ms); i++ {
			// I think it look the same and yet it failed
			// if !assert.ObjectsAreEqual(expected.Ms[i], actual.Ms[i]) {
			// 	return false
			// }
			if c, ok := expected.Ms[i].(*Car); ok {
				if c.ID.String() != actual.Ms[i].(*Car).ID.String() {
					log.Println(".............5.1")
					return false
				}
				if c.Name != actual.Ms[i].(*Car).Name {
					log.Println(".............5.2")
					return false
				}
			}

			if c, ok := expected.Ms[i].(*CarWithCallbacks); ok {
				if c.ID.String() != actual.Ms[i].(*CarWithCallbacks).ID.String() {
					log.Println(".............5.3")
					return false
				}
				if c.Name != actual.Ms[i].(*CarWithCallbacks).Name {
					log.Println(".............5.4")
					return false
				}
			}
		}

		if expected.TypeString != actual.TypeString {
			log.Println(".............7")
			return false
		}

		if len(expected.Roles) != len(actual.Roles) {
			log.Println(".............8")
			return false
		}

		for i := 0; i < len(expected.Roles); i++ {
			if expected.Roles[i] != actual.Roles[i] {
				log.Println(".............9")
				return false
			}
		}

		if len(expected.URLParams) != 0 && len(actual.URLParams) != 0 {
			if !assert.ObjectsAreEqualValues(expected.URLParams, actual.URLParams) {
				log.Println(".............10")
				return false
			}
		}

		return true
	}
}

func bhpDataComparisonNoDB(expected *mdlutil.BatchHookPointData, actual *mdlutil.BatchHookPointData) func() (success bool) {
	return func() (success bool) {
		if expected.Who != actual.Who {
			log.Println(".............2")
			return false
		}

		if (expected.Cargo.Payload != nil && actual.Cargo.Payload == nil) ||
			(expected.Cargo.Payload == nil && actual.Cargo.Payload != nil) {
			log.Println(".............3")
			return false
		}

		if expected.Cargo.Payload != nil && actual.Cargo.Payload != nil &&
			expected.Cargo != actual.Cargo {
			log.Println(".............4")
			return false
		}

		if len(expected.Ms) != len(actual.Ms) {
			log.Println(".............5")
			return false
		}

		for i := 0; i < len(expected.Ms); i++ {
			// I think it look the same and yet it failed
			// if !assert.ObjectsAreEqual(expected.Ms[i], actual.Ms[i]) {
			// 	return false
			// }
			if c, ok := expected.Ms[i].(*Car); ok {
				if c.ID.String() != actual.Ms[i].(*Car).ID.String() {
					log.Println(".............5.1")
					return false
				}
				if c.Name != actual.Ms[i].(*Car).Name {
					log.Println(".............5.2")
					return false
				}
			}

			if c, ok := expected.Ms[i].(*CarWithCallbacks); ok {
				if c.ID.String() != actual.Ms[i].(*CarWithCallbacks).ID.String() {
					log.Println(".............5.3")
					return false
				}
				if c.Name != actual.Ms[i].(*CarWithCallbacks).Name {
					log.Println(".............5.4")
					return false
				}
			}
		}

		if expected.TypeString != actual.TypeString {
			log.Println(".............7")
			return false
		}

		if len(expected.Roles) != len(actual.Roles) {
			log.Println(".............8")
			return false
		}

		for i := 0; i < len(expected.Roles); i++ {
			if expected.Roles[i] != actual.Roles[i] {
				log.Println(".............9")
				return false
			}
		}

		if len(expected.URLParams) != 0 && len(actual.URLParams) != 0 {
			if !assert.ObjectsAreEqualValues(expected.URLParams, actual.URLParams) {
				log.Println(".............10")
				return false
			}
		}

		return true
	}
}

func deepCopyBHPData(src *mdlutil.BatchHookPointData, dst *mdlutil.BatchHookPointData) {
	dst.DB = src.DB
	dst.Who = src.Who
	dst.TypeString = src.TypeString
	dst.Cargo = &mdlutil.BatchHookCargo{Payload: src.Cargo.Payload}
	dst.Roles = src.Roles
	dst.URLParams = src.URLParams

	// https://stackoverflow.com/questions/56355212/deep-copying-data-structures-in-golang
	dst.Ms = make([]mdl.IModel, len(src.Ms))
	for i, model := range src.Ms {
		v := reflect.ValueOf(model).Elem()
		vp2 := reflect.New(v.Type())
		vp2.Elem().Set(v)
		dst.Ms[i] = vp2.Interface().(mdl.IModel)
	}
}

func deepCopyData(src *hook.Data, dst *hook.Data) {
	dst.DB = src.DB
	// dst.TypeString = src.TypeString
	dst.Cargo = &hook.Cargo{Payload: src.Cargo.Payload}
	dst.Roles = src.Roles
	// dst.URLParams = src.URLParams

	// https://stackoverflow.com/questions/56355212/deep-copying-data-structures-in-golang
	dst.Ms = make([]mdl.IModel, len(src.Ms))
	for i, model := range src.Ms {
		v := reflect.ValueOf(model).Elem()
		vp2 := reflect.New(v.Type())
		vp2.Elem().Set(v)
		dst.Ms[i] = vp2.Interface().(mdl.IModel)
	}
}

func TestBaseMappingCreateSuite(t *testing.T) {
	suite.Run(t, new(TestBaseMapperCreateSuite))
}
