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
	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/utils/transact"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry"
)

// ----------------------------------------------------------------------------------------------

// Who is the information about the client or the user
type WhoMock struct {
	Oid *datatypes.UUID // ownerid
}

func (w *WhoMock) GetUserID() *datatypes.UUID {
	return w.Oid
}

type Car struct {
	models.BaseModel

	Name string `json:"name"`

	Ownerships []models.OwnershipModelWithIDBase `gorm:"PRELOAD:false" json:"-" betterrest:"ownership"`
}

// ----------------------------------------------------------------------------------------------

// These have to be out there because out-style in-model hooks are not called in a single
// object, before and after can be different.

var guardAPIEntryCalled bool
var guardAPIEntryWho models.UserIDFetchable
var guardAPIEntryHTTP models.HTTP

var beforeCUPDDBCalled bool
var beforeCUPDDBHpdata models.HookPointData
var beforeCUPDDBOp models.CRUPDOp

var beforeCreateDBCalled bool
var beforeCreateDBHpdata models.HookPointData

var beforeReadDBCalled bool
var beforeReadDBHpdata models.HookPointData

var beforeUpdateDBCalled bool
var beforeUpdateDBHpdata models.HookPointData

var beforePatchDBCalled bool
var beforePatchDBHpdata models.HookPointData

var beforeDeleteDBCalled bool
var beforeDeleteDBHpdata models.HookPointData

var afterCRUPDDBCalled bool
var afterCRUPDDBHpdata models.HookPointData
var afterCRUPDDBOp models.CRUPDOp

var afterCreateDBCalled bool
var afterCreateDBHpdata models.HookPointData

var afterReadDBCalled bool
var afterReadDBHpdata models.HookPointData

var afterUpdateDBCalled bool
var afterUpdateDBHpdata models.HookPointData

var afterPatchDBCalled bool
var afterPatchDBHpdata models.HookPointData

var afterDeleteDBCalled bool
var afterDeleteDBHpdata models.HookPointData

func resetGlobals() {
	guardAPIEntryCalled = false
	guardAPIEntryWho = nil
	guardAPIEntryHTTP = models.HTTP{}

	beforeCUPDDBCalled = false
	beforeCUPDDBHpdata = models.HookPointData{}
	beforeCUPDDBOp = models.CRUPDOpOther

	beforeCreateDBCalled = false
	beforeCreateDBHpdata = models.HookPointData{}

	beforeReadDBCalled = false
	beforeReadDBHpdata = models.HookPointData{}

	beforeUpdateDBCalled = false
	beforeUpdateDBHpdata = models.HookPointData{}

	beforePatchDBCalled = false
	beforePatchDBHpdata = models.HookPointData{}

	beforeDeleteDBCalled = false
	beforeDeleteDBHpdata = models.HookPointData{}

	afterCRUPDDBCalled = false
	afterCRUPDDBHpdata = models.HookPointData{}
	afterCRUPDDBOp = models.CRUPDOpOther

	afterCreateDBCalled = false
	afterCreateDBHpdata = models.HookPointData{}

	afterReadDBCalled = false
	afterReadDBHpdata = models.HookPointData{}

	afterUpdateDBCalled = false
	afterUpdateDBHpdata = models.HookPointData{}

	afterPatchDBCalled = false
	afterPatchDBHpdata = models.HookPointData{}

	afterDeleteDBCalled = false
	afterDeleteDBHpdata = models.HookPointData{}

}

type CarWithCallbacks struct {
	models.BaseModel

	Name string `json:"name"`

	Ownerships []models.OwnershipModelWithIDBase `gorm:"PRELOAD:false" json:"-" betterrest:"ownership"`

	// GuardAPIEntryCalled bool                   `gorm:"-" json:"-"`
	// GuardAPIEntryWho    models.UserIDFetchable `gorm:"-" json:"-" betterrest:"peg-ignore"`
	// GuardAPIEntryHTTP   models.HTTP            `gorm:"-" json:"-" betterrest:"peg-ignore"`

	// BeforeCUPDDBCalled bool                 `gorm:"-" json:"-"`
	// BeforeCUPDDBHpdata models.HookPointData `gorm:"-" json:"-" betterrest:"peg-ignore"`
	// BeforeCUPDDBOp     models.CRUPDOp       `gorm:"-" json:"-" betterrest:"peg-ignore"`

	// BeforeCreateDBCalled bool                 `gorm:"-" json:"-"`
	// BeforeCreateDBHpdata models.HookPointData `gorm:"-" json:"-" betterrest:"peg-ignore"`

	// BeforeReadDBCalled bool                 `gorm:"-" json:"-"`
	// BeforeReadDBHpdata models.HookPointData `gorm:"-" json:"-" betterrest:"peg-ignore"`

	// BeforeUpdateDBCalled bool                 `gorm:"-" json:"-"`
	// BeforeUpdateDBHpdata models.HookPointData `gorm:"-" json:"-" betterrest:"peg-ignore"`

	// AfterCRUPDDBCalled bool                 `gorm:"-" json:"-"`
	// AfterCRUPDDBHpdata models.HookPointData `gorm:"-" json:"-" betterrest:"peg-ignore"`
	// AfterCRUPDDBOp     models.CRUPDOp       `gorm:"-" json:"-" betterrest:"peg-ignore"`

	// AfterCreateDBCalled bool                 `gorm:"-" json:"-"`
	// AfterCreateDBHpdata models.HookPointData `gorm:"-" json:"-" betterrest:"peg-ignore"`

	// AfterReadDBCalled bool                 `gorm:"-" json:"-"`
	// AfterReadDBHpdata models.HookPointData `gorm:"-" json:"-" betterrest:"peg-ignore"`
}

func (CarWithCallbacks) TableName() string {
	return "car"
}

// GuardAPIEntry guards denies api call based on scope
func (c *CarWithCallbacks) GuardAPIEntry(who models.UserIDFetchable, http models.HTTP) bool {
	guardAPIEntryCalled = true
	guardAPIEntryWho = who
	guardAPIEntryHTTP = http
	return true // true to pass
}

func (c *CarWithCallbacks) BeforeCUPDDB(hpdata models.HookPointData, op models.CRUPDOp) error {
	beforeCUPDDBCalled = true
	beforeCUPDDBHpdata = hpdata
	beforeCUPDDBOp = op
	return nil
}

func (c *CarWithCallbacks) BeforeCreateDB(hpdata models.HookPointData) error {
	beforeCreateDBCalled = true
	beforeCreateDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) BeforeReadDB(hpdata models.HookPointData) error {
	beforeReadDBCalled = true
	beforeReadDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) BeforeUpdateDB(hpdata models.HookPointData) error {
	beforeUpdateDBCalled = true
	beforeUpdateDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) BeforePatchDB(hpdata models.HookPointData) error {
	beforePatchDBCalled = true
	beforePatchDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) BeforeDeleteDB(hpdata models.HookPointData) error {
	beforeDeleteDBCalled = true
	beforeDeleteDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) AfterCRUPDDB(hpdata models.HookPointData, op models.CRUPDOp) error {
	afterCRUPDDBCalled = true
	afterCRUPDDBHpdata = hpdata
	afterCRUPDDBOp = op

	return nil
}

func (c *CarWithCallbacks) AfterCreateDB(hpdata models.HookPointData) error {
	afterCreateDBCalled = true
	afterCreateDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) AfterReadDB(hpdata models.HookPointData) error {
	afterReadDBCalled = true
	afterReadDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) AfterUpdateDB(hpdata models.HookPointData) error {
	afterUpdateDBCalled = true
	afterUpdateDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) AfterPatchDB(hpdata models.HookPointData) error {
	afterPatchDBCalled = true
	afterPatchDBHpdata = hpdata
	return nil
}

func (c *CarWithCallbacks) AfterDeleteDB(hpdata models.HookPointData) error {
	afterDeleteDBCalled = true
	afterDeleteDBHpdata = hpdata
	return nil
}

// ----------------------------------------------------------------------------------------------

func createBatchHookPoint(called *bool, bhpDataCalled *models.BatchHookPointData, opCalled *models.CRUPDOp) func(bhpData models.BatchHookPointData, op models.CRUPDOp) error {
	return func(bhpData models.BatchHookPointData, op models.CRUPDOp) error {
		*called = true

		deepCopyBHPData(&bhpData, bhpDataCalled)

		*opCalled = op
		return nil
	}
}

func createBatchSingleMethodHookPoint(called *bool, bhpDataCalled *models.BatchHookPointData) func(bhpData models.BatchHookPointData) error {
	return func(bhpData models.BatchHookPointData) error {
		*called = true

		deepCopyBHPData(&bhpData, bhpDataCalled)
		return nil
	}
}

// ----------------------------------------------------------------------------------------------

type CarControllerWithoutCallbacks struct {
}

func (c *CarControllerWithoutCallbacks) Init(data *hookhandler.InitData) {
}

type CarHandlerJBT struct {
	// From init
	who        models.UserIDFetchable
	typeString string
	roles      []models.UserRole
	urlParams  map[urlparam.Param]interface{}
	info       *hookhandler.EndPointInfo

	guardAPIEntryCalled bool
	guardAPIEntryData   *hookhandler.Data
	guardAPIEntryInfo   *hookhandler.EndPointInfo

	beforeApplyCalled bool
	beforeApplyData   *hookhandler.Data
	beforeApplyInfo   *hookhandler.EndPointInfo

	beforeCalled bool
	beforeData   *hookhandler.Data
	beforeInfo   *hookhandler.EndPointInfo

	afterCalled bool
	afterData   *hookhandler.Data
	afterInfo   *hookhandler.EndPointInfo
}

func (c *CarHandlerJBT) Init(data *hookhandler.InitData) {
	c.who = data.Who
	c.typeString = data.TypeString
	c.roles = data.Roles
	c.urlParams = data.URLParams
	c.info = data.Info
}

func (c *CarHandlerJBT) GuardAPIEntry(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.GuardRetVal {
	c.guardAPIEntryCalled = true
	c.guardAPIEntryData = &hookhandler.Data{}
	deepCopyData(data, c.guardAPIEntryData)
	c.guardAPIEntryInfo = info
	return nil
}

func (c *CarHandlerJBT) BeforeApply(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	c.beforeApplyCalled = true
	c.beforeApplyData = &hookhandler.Data{}
	deepCopyData(data, c.beforeApplyData)
	c.beforeApplyInfo = info
	return nil
}

func (c *CarHandlerJBT) Before(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	c.beforeCalled = true
	c.beforeData = &hookhandler.Data{}
	deepCopyData(data, c.beforeData)
	c.beforeInfo = info
	return nil
}

func (c *CarHandlerJBT) After(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	c.afterCalled = true
	c.afterData = &hookhandler.Data{}
	deepCopyData(data, c.afterData)
	c.afterInfo = info
	return nil
}

// ----------------------------------------------------------------------------------------------

type TestBaseMapperCreateSuite struct {
	suite.Suite
	db         *gorm.DB
	mock       sqlmock.Sqlmock
	who        models.UserIDFetchable
	typeString string
}

func (suite *TestBaseMapperCreateSuite) SetupTest() {
	sqldb, mock, _ := sqlmock.New() // db, mock, error. We're testing lifecycle here
	suite.db, _ = gorm.Open("postgres", sqldb)
	// suite.db.LogMode(true)
	suite.db.SingularTable(true)
	suite.mock = mock
	suite.who = &WhoMock{Oid: datatypes.NewUUID()} // userid
	suite.typeString = "cars"

	// clear registry
	delete(registry.ModelRegistry, "cars")

	resetGlobals()
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestBaseMapperCreateSuite) TestCreateOne_WhenGiven_GotCar() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	var modelObj models.IModel = &Car{BaseModel: models.BaseModel{ID: carID}, Name: carName}

	suite.mock.ExpectBegin()
	stmt := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt3 := `SELECT * FROM "car"  WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	mapper := SharedOwnershipMapper()

	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		if retVal, retErr = mapper.CreateOne(tx, suite.who, suite.typeString, modelObj, options, &cargo); retErr != nil {
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
	carID := datatypes.NewUUID()
	carName := "DSM"
	var modelObj models.IModel = &CarWithCallbacks{BaseModel: models.BaseModel{ID: carID}, Name: carName}

	suite.mock.ExpectBegin()
	stmt := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt3 := `SELECT * FROM "car"  WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	mapper := SharedOwnershipMapper()

	var retVal *MapperRet
	var tx2 *gorm.DB
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		if retVal, retErr = mapper.CreateOne(tx, suite.who, suite.typeString, modelObj, options, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	role := models.UserRoleAdmin
	hpdata := models.HookPointData{DB: tx2, Who: suite.who, TypeString: suite.typeString,
		Cargo: &models.ModelCargo{Payload: cargo.Payload}, Role: &role, URLParams: options}

	if _, ok := retVal.Ms[0].(*CarWithCallbacks); assert.True(suite.T(), ok) {
		assert.False(suite.T(), guardAPIEntryCalled) // not called when going through mapper
		// Create has no before callback since haven't been read
		if assert.True(suite.T(), beforeCUPDDBCalled) {
			assert.Equal(suite.T(), beforeCUPDDBOp, models.CRUPDOpCreate)
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &beforeCUPDDBHpdata))
		}

		if assert.True(suite.T(), beforeCreateDBCalled) {
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &beforeCreateDBHpdata))
		}

		if assert.True(suite.T(), afterCRUPDDBCalled) {
			assert.Equal(suite.T(), afterCRUPDDBOp, models.CRUPDOpCreate)
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &afterCRUPDDBHpdata))
		}
		if assert.True(suite.T(), afterCreateDBCalled) {
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &afterCRUPDDBHpdata))
		}
	}
}

func (suite *TestBaseMapperCreateSuite) TestCreateOne_WhenHavingController_NotCallOldCallbacks() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	var modelObj models.IModel = &CarWithCallbacks{BaseModel: models.BaseModel{ID: carID}, Name: carName}

	suite.mock.ExpectBegin()
	stmt := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt3 := `SELECT * FROM "car"  WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	hdlr := CarControllerWithoutCallbacks{}
	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt).HookHandler(&hdlr, "CRUPD")

	mapper := SharedOwnershipMapper()

	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		if retVal, retErr = mapper.CreateOne(tx, suite.who, suite.typeString, modelObj, options, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	if _, ok := modelObj.(*CarWithCallbacks); assert.True(suite.T(), ok) {
		// None of the model callback should be called when there is hookhandler
		assert.False(suite.T(), guardAPIEntryCalled) // not called when going through mapper
		assert.False(suite.T(), beforeCUPDDBCalled)
		assert.False(suite.T(), beforeReadDBCalled)
		assert.False(suite.T(), afterCRUPDDBCalled)
		assert.False(suite.T(), afterReadDBCalled)
	}

	if _, ok := retVal.Ms[0].(*CarWithCallbacks); assert.True(suite.T(), ok) {
		// None of the model callback should be called when there is hookhandler
		assert.False(suite.T(), guardAPIEntryCalled) // not called when going through mapper
		assert.False(suite.T(), beforeCUPDDBCalled)
		assert.False(suite.T(), beforeReadDBCalled)
		assert.False(suite.T(), afterCRUPDDBCalled)
		assert.False(suite.T(), afterReadDBCalled)
	}
}

func (suite *TestBaseMapperCreateSuite) TestCreateOne_WhenHavingController_CallRelevantControllerCallbacks() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	var modelObj models.IModel = &CarWithCallbacks{BaseModel: models.BaseModel{ID: carID}, Name: carName}

	suite.mock.ExpectBegin()
	stmt := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	stmt3 := `SELECT * FROM "car"  WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt).HookHandler(&CarHandlerJBT{}, "CRUPD")

	mapper := SharedOwnershipMapper()

	// var modelObj2 models.IModel
	var tx2 *gorm.DB
	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		if retVal, retErr = mapper.CreateOne(tx, suite.who, suite.typeString, modelObj, options, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	role := models.UserRoleAdmin
	data := hookhandler.Data{Ms: []models.IModel{&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID}, Name: carName}}, DB: tx2, Who: suite.who, TypeString: suite.typeString, Roles: []models.UserRole{role}, Cargo: &cargo}
	info := hookhandler.EndPointInfo{Op: hookhandler.RESTOpCreate, Cardinality: hookhandler.APICardinalityOne}

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
		assert.Equal(suite.T(), info, *hdlr.beforeInfo)
	}

	// After hookhandler has made some modification to data (this is harder to test)
	// data.Ms[0].(*Car).Name = data.Ms[0].(*Car).Name + "-after"

	if assert.True(suite.T(), hdlr.afterCalled) {
		assert.Condition(suite.T(), dataComparison(&data, hdlr.afterData))
		assert.Equal(suite.T(), info, *hdlr.afterInfo)
	}
}

func (suite *TestBaseMapperCreateSuite) TestCreateMany_WhenGiven_GotCars() {
	carID1 := datatypes.NewUUID()
	carName1 := "DSM"
	carID2 := datatypes.NewUUID()
	carName2 := "DSM4Life"
	carID3 := datatypes.NewUUID()
	carName3 := "Eclipse"
	modelObjs := []models.IModel{
		&Car{BaseModel: models.BaseModel{ID: carID1}, Name: carName1},
		&Car{BaseModel: models.BaseModel{ID: carID2}, Name: carName2},
		&Car{BaseModel: models.BaseModel{ID: carID3}, Name: carName3},
	}

	suite.mock.ExpectBegin()
	// Gorm v1 insert 3 times
	// (Also Gorm v2 support modifying returning: https://gorm.io/docs/update.html#Returning-Data-From-Modified-Rows
	stmt1 := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	stmt3 := `SELECT * FROM "car" WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))
	// actually it might not be possible to fetch the id gorm gives
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatypes.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatypes.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatypes.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		mapper := SharedOwnershipMapper()
		retVal, retErr = mapper.CreateMany(tx, suite.who, suite.typeString, modelObjs, options, &cargo)
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
	carID1 := datatypes.NewUUID()
	carName1 := "DSM"
	carID2 := datatypes.NewUUID()
	carName2 := "DSM4Life"
	carID3 := datatypes.NewUUID()
	carName3 := "Eclipse"
	modelObjs := []models.IModel{
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carName1},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carName2},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carName3},
	}

	suite.mock.ExpectBegin()
	// Gorm v1 insert 3 times
	// (Also Gorm v2 support modifying returning: https://gorm.io/docs/update.html#Returning-Data-From-Modified-Rows
	stmt1 := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	stmt3 := `SELECT * FROM "car" WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))
	// actually it might not be possible to fetch the id gorm gives
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatypes.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatypes.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatypes.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	var beforeCalled bool
	var beforeData models.BatchHookPointData
	var beforeOp models.CRUPDOp
	before := createBatchHookPoint(&beforeCalled, &beforeData, &beforeOp)

	var afterCalled bool
	var afterData models.BatchHookPointData
	var afterOp models.CRUPDOp
	after := createBatchHookPoint(&afterCalled, &afterData, &afterOp)

	var beforeCreateCalled bool
	var beforeCreateData models.BatchHookPointData
	beforeCreate := createBatchSingleMethodHookPoint(&beforeCreateCalled, &beforeCreateData)

	var afterCreateCalled bool
	var afterCreateData models.BatchHookPointData
	afterCreate := createBatchSingleMethodHookPoint(&afterCreateCalled, &afterCreateData)

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).
		BatchCRUPDHooks(before, after).BatchCreateHooks(beforeCreate, afterCreate)

	var tx2 *gorm.DB
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		_, retErr = mapper.CreateMany(tx2, suite.who, suite.typeString, modelObjs, options, &cargo)
		return retErr
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}
	roles := []models.UserRole{models.UserRoleAdmin, models.UserRoleAdmin, models.UserRoleAdmin}

	// Expected
	expectedData := models.BatchHookPointData{
		Ms: []models.IModel{
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carName1},
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carName2},
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carName3},
		},
		DB: tx2, Who: suite.who, TypeString: suite.typeString, Roles: roles, URLParams: options,
		Cargo: &models.BatchHookCargo{Payload: cargo.Payload},
	}

	if assert.True(suite.T(), beforeCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &beforeData))
		// It won't be null because it ain't pointers
		// assert.Nil(suite.T(), beforeData.Ms[0].GetCreatedAt())
		// assert.Nil(suite.T(), beforeData.Ms[0].GetUpdatedAt())
		assert.Equal(suite.T(), beforeOp, models.CRUPDOpCreate)
	}

	if assert.True(suite.T(), beforeCreateCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &beforeCreateData))
		// 	assert.Nil(suite.T(), beforeCreateData.Ms[0].GetCreatedAt())
		// 	assert.Nil(suite.T(), beforeCreateData.Ms[0].GetUpdatedAt())
	}

	if assert.True(suite.T(), afterCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &afterData))
		assert.Equal(suite.T(), afterOp, models.CRUPDOpCreate)
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
	carID1 := datatypes.NewUUID()
	carName1 := "DSM"
	carID2 := datatypes.NewUUID()
	carName2 := "DSM4Life"
	carID3 := datatypes.NewUUID()
	carName3 := "Eclipse"
	modelObjs := []models.IModel{
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carName1},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carName2},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carName3},
	}

	suite.mock.ExpectBegin()
	// Gorm v1 insert 3 times
	// (Also Gorm v2 support modifying returning: https://gorm.io/docs/update.html#Returning-Data-From-Modified-Rows
	stmt1 := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	stmt3 := `SELECT * FROM "car" WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))
	// actually it might not be possible to fetch the id gorm gives
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatypes.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatypes.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatypes.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	var beforeCalled bool
	var beforeData models.BatchHookPointData
	var beforeOp models.CRUPDOp
	before := createBatchHookPoint(&beforeCalled, &beforeData, &beforeOp)

	var afterCalled bool
	var afterData models.BatchHookPointData
	var afterOp models.CRUPDOp
	after := createBatchHookPoint(&afterCalled, &afterData, &afterOp)

	var beforeCreateCalled bool
	var beforeCreateData models.BatchHookPointData
	beforeCreate := createBatchSingleMethodHookPoint(&beforeCreateCalled, &beforeCreateData)

	var afterCreateCalled bool
	var afterCreateData models.BatchHookPointData
	afterCreate := createBatchSingleMethodHookPoint(&afterCreateCalled, &afterCreateData)

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).
		BatchCRUPDHooks(before, after).BatchCreateHooks(beforeCreate, afterCreate).HookHandler(&CarControllerWithoutCallbacks{}, "CRUPD")

	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		mapper := SharedOwnershipMapper()
		_, retErr = mapper.CreateMany(tx, suite.who, suite.typeString, modelObjs, options, &cargo)
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
	carID1 := datatypes.NewUUID()
	carName1 := "DSM"
	carID2 := datatypes.NewUUID()
	carName2 := "DSM4Life"
	carID3 := datatypes.NewUUID()
	carName3 := "Eclipse"
	modelObjs := []models.IModel{
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carName1},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carName2},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carName3},
	}

	suite.mock.ExpectBegin()
	// Gorm v1 insert 3 times
	// (Also Gorm v2 support modifying returning: https://gorm.io/docs/update.html#Returning-Data-From-Modified-Rows
	stmt1 := `INSERT INTO "car" ("id","created_at","updated_at","deleted_at","name") VALUES ($1,$2,$3,$4,$5) RETURNING "car"."id"`
	stmt2 := `INSERT INTO "user_owns_car" ("id","created_at","updated_at","deleted_at","role","user_id","model_id") VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING "user_owns_car"."id"`
	stmt3 := `SELECT * FROM "car" WHERE "car"."deleted_at" IS NULL AND "car"."id" = $1 LIMIT 1`

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))
	// actually it might not be possible to fetch the id gorm gives
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatypes.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID1))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatypes.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID2))

	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(datatypes.NewUUID()))
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt3)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(carID3))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).HookHandler(&CarHandlerJBT{}, "CRUPD")

	var tx2 *gorm.DB
	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		mapper := SharedOwnershipMapper()
		tx2 = tx
		retVal, retErr = mapper.CreateMany(tx2, suite.who, suite.typeString, modelObjs, options, &cargo)
		return retErr
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	roles := []models.UserRole{models.UserRoleAdmin, models.UserRoleAdmin, models.UserRoleAdmin}
	data := hookhandler.Data{Ms: []models.IModel{
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carName1},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carName2},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carName3},
	}, DB: tx2, Who: suite.who, TypeString: suite.typeString, Roles: roles, Cargo: &cargo}
	info := hookhandler.EndPointInfo{Op: hookhandler.RESTOpCreate, Cardinality: hookhandler.APICardinalityMany}

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
		assert.Equal(suite.T(), info.Op, hdlr.beforeInfo.Op)
		assert.Equal(suite.T(), info.Cardinality, hdlr.beforeInfo.Cardinality)
	}
	if assert.True(suite.T(), hdlr.afterCalled) {
		assert.Condition(suite.T(), dataComparison(&data, hdlr.afterData))
		assert.Equal(suite.T(), info.Op, hdlr.afterInfo.Op)
		assert.Equal(suite.T(), info.Cardinality, hdlr.afterInfo.Cardinality)
	}
}

func dataComparison(expected *hookhandler.Data, actual *hookhandler.Data) func() (success bool) {
	return func() (success bool) {
		if expected.DB != actual.DB {
			log.Println("dataComparison 1")
			return false
		}

		if expected.Who != actual.Who {
			log.Println("dataComparison 2")
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

		if expected.TypeString != actual.TypeString {
			log.Println("dataComparison 7")
			return false
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

		if len(expected.URLParams) != 0 && len(actual.URLParams) != 0 {
			if !assert.ObjectsAreEqualValues(expected.URLParams, actual.URLParams) {
				log.Println("dataComparison 10")
				return false
			}
		}

		return true
	}
}

func dataComparisonNoDB(expected *hookhandler.Data, actual *hookhandler.Data) func() (success bool) {
	return func() (success bool) {
		if expected.Who != actual.Who {
			log.Println("dataComparison 2")
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

		if expected.TypeString != actual.TypeString {
			log.Println("dataComparison 7")
			return false
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

		if len(expected.URLParams) != 0 && len(actual.URLParams) != 0 {
			if !assert.ObjectsAreEqualValues(expected.URLParams, actual.URLParams) {
				log.Println("dataComparison 10")
				return false
			}
		}

		return true
	}
}

func hpDataComparison(expected *models.HookPointData, actual *models.HookPointData) func() (success bool) {
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
func hpDataComparisonNoDB(expected *models.HookPointData, actual *models.HookPointData) func() (success bool) {
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

func bhpDataComparison(expected *models.BatchHookPointData, actual *models.BatchHookPointData) func() (success bool) {
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

func bhpDataComparisonNoDB(expected *models.BatchHookPointData, actual *models.BatchHookPointData) func() (success bool) {
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

func deepCopyBHPData(src *models.BatchHookPointData, dst *models.BatchHookPointData) {
	dst.DB = src.DB
	dst.Who = src.Who
	dst.TypeString = src.TypeString
	dst.Cargo = &models.BatchHookCargo{Payload: src.Cargo.Payload}
	dst.Roles = src.Roles
	dst.URLParams = src.URLParams

	// https://stackoverflow.com/questions/56355212/deep-copying-data-structures-in-golang
	dst.Ms = make([]models.IModel, len(src.Ms))
	for i, model := range src.Ms {
		v := reflect.ValueOf(model).Elem()
		vp2 := reflect.New(v.Type())
		vp2.Elem().Set(v)
		dst.Ms[i] = vp2.Interface().(models.IModel)
	}
}

func deepCopyData(src *hookhandler.Data, dst *hookhandler.Data) {
	dst.DB = src.DB
	dst.Who = src.Who
	dst.TypeString = src.TypeString
	dst.Cargo = &hookhandler.Cargo{Payload: src.Cargo.Payload}
	dst.Roles = src.Roles
	dst.URLParams = src.URLParams

	// https://stackoverflow.com/questions/56355212/deep-copying-data-structures-in-golang
	dst.Ms = make([]models.IModel, len(src.Ms))
	for i, model := range src.Ms {
		v := reflect.ValueOf(model).Elem()
		vp2 := reflect.New(v.Type())
		vp2.Elem().Set(v)
		dst.Ms[i] = vp2.Interface().(models.IModel)
	}
}

func TestBaseMappingCreateSuite(t *testing.T) {
	suite.Run(t, new(TestBaseMapperCreateSuite))
}
