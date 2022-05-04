package datamapper

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry"
)

type TestBaseMapperReadSuite struct {
	suite.Suite
	db         *gorm.DB
	mock       sqlmock.Sqlmock
	who        models.UserIDFetchable
	typeString string
}

func (suite *TestBaseMapperReadSuite) SetupTest() {
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
func (suite *TestBaseMapperReadSuite) TestReadOne_WhenGiven_GotCar() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))

	suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "model_id", "role"}).AddRow(suite.who.GetUserID(), carID, models.UserRoleGuest))

	modelID := carID
	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	mapper := SharedOwnershipMapper()
	retVal, role, retErr := mapper.ReadOne(suite.db, suite.who, suite.typeString, modelID, options, &cargo)
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	assert.Equal(suite.T(), models.UserRoleGuest, role)

	if car, ok := retVal.Ms[0].(*Car); assert.True(suite.T(), ok) {
		assert.Equal(suite.T(), carName, car.Name)
		assert.Equal(suite.T(), carID, car.ID)
	}
}

func (suite *TestBaseMapperReadSuite) TestReadOne_WhenNoController_CallRelevantOldCallbacks() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	role := models.UserRoleAdmin
	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))

	suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "model_id", "role"}).AddRow(suite.who.GetUserID(), carID, role))

	modelID := datatypes.NewUUID()
	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt)
	// opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	// registry.For(typeString).ModelWithOption(&Car{}, opt)

	mapper := SharedOwnershipMapper()
	retVal, _, retErr := mapper.ReadOne(suite.db, suite.who, suite.typeString, modelID, options, &cargo)
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	hpdata := models.HookPointData{DB: suite.db, Who: suite.who, TypeString: suite.typeString,
		Cargo: &models.ModelCargo{Payload: cargo.Payload}, Role: &role, URLParams: options}

	if _, ok := retVal.Ms[0].(*CarWithCallbacks); assert.True(suite.T(), ok) {
		assert.False(suite.T(), guardAPIEntryCalled) // not called when going through mapper
		// Read has no before callback since haven't been read
		assert.False(suite.T(), beforeCUPDDBCalled)
		assert.False(suite.T(), beforeReadDBCalled)
		if assert.True(suite.T(), afterCRUPDDBCalled) {
			afterCRUPDDBOp = models.CRUPDOpRead
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &afterCRUPDDBHpdata))
		}
		if assert.True(suite.T(), afterReadDBCalled) {
			afterCRUPDDBOp = models.CRUPDOpRead
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &afterCRUPDDBHpdata))
		}
	}
}

func (suite *TestBaseMapperReadSuite) TestReadOne_WhenHavingController_NotCallOldCallbacks() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))

	suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "model_id", "role"}).AddRow(suite.who.GetUserID(), carID, models.UserRoleAdmin))

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	hdlr := CarControllerWithoutCallbacks{}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).HookHandler(&hdlr, "CRUPD")

	modelID := datatypes.NewUUID()
	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	mapper := SharedOwnershipMapper()
	retVal, _, retErr := mapper.ReadOne(suite.db, suite.who, suite.typeString, modelID, options, &cargo)
	if !assert.Nil(suite.T(), retErr) {
		return
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

func (suite *TestBaseMapperReadSuite) TestReadOne_WhenHavingController_CallRelevantControllerCallbacks() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))

	role := models.UserRoleAdmin
	suite.mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "model_id", "role"}).AddRow(suite.who.GetUserID(), carID, role))

	modelID := datatypes.NewUUID()
	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).HookHandler(&CarHandlerJBT{}, "CRUPD")

	mapper := SharedOwnershipMapper()
	var retVal *MapperRet
	retVal, _, retErr := mapper.ReadOne(suite.db, suite.who, suite.typeString, modelID, options, &cargo)
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	data := hookhandler.Data{Ms: []models.IModel{&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID}, Name: carName}}, DB: suite.db, Who: suite.who, TypeString: suite.typeString, Roles: []models.UserRole{role}, Cargo: &cargo}
	info := hookhandler.EndPointInfo{Op: hookhandler.RESTOpRead, Cardinality: hookhandler.APICardinalityOne}

	ctrls := retVal.Fetcher.GetAllInstantiatedHanders()
	if !assert.Len(suite.T(), ctrls, 1) {
		return
	}

	hdlr, ok := ctrls[0].(*CarHandlerJBT)
	if !assert.True(suite.T(), ok) {
		return
	}

	assert.False(suite.T(), hdlr.guardAPIEntryCalled) // Not called when going through mapper (or lifecycle for that matter)

	assert.False(suite.T(), hdlr.beforeApplyCalled)
	assert.False(suite.T(), hdlr.beforeCalled)
	if assert.True(suite.T(), hdlr.afterCalled) {
		assert.Condition(suite.T(), dataComparison(&data, hdlr.afterData))
		assert.Equal(suite.T(), info, *hdlr.afterInfo)
	}
}

func (suite *TestBaseMapperReadSuite) TestReadMany_WhenGiven_GotCars() {
	carID1 := datatypes.NewUUID()
	carName1 := "DSM"
	carID2 := datatypes.NewUUID()
	carName2 := "DSM4Life"
	carID3 := datatypes.NewUUID()
	carName3 := "Eclipse"

	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $1 WHERE "car"."deleted_at" IS NULL ORDER BY "car".created_at DESC LIMIT 100 OFFSET 0`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID1, carName1).AddRow(carID2, carName2).AddRow(carID3, carName3))

	stmt2 := `SELECT "user_owns_car"."role" FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $1 WHERE ("car"."deleted_at" IS NULL) ORDER BY "car".created_at DESC LIMIT 100 OFFSET 0`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(models.UserRoleAdmin).AddRow(models.UserRoleGuest).AddRow(models.UserRoleAdmin))

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	mapper := SharedOwnershipMapper()
	retVal, roles, no, retErr := mapper.ReadMany(suite.db, suite.who, suite.typeString, options, &cargo)
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	assert.Nil(suite.T(), no) // since I didn't ask for total count

	assert.ElementsMatch(suite.T(), []models.UserRole{models.UserRoleAdmin, models.UserRoleGuest, models.UserRoleAdmin}, roles)
	if assert.Len(suite.T(), retVal.Ms, 3) {
		assert.Equal(suite.T(), carID1.String(), retVal.Ms[0].GetID().String())
		assert.Equal(suite.T(), carID2.String(), retVal.Ms[1].GetID().String())
		assert.Equal(suite.T(), carID3.String(), retVal.Ms[2].GetID().String())
		assert.Equal(suite.T(), carName1, retVal.Ms[0].(*Car).Name)
		assert.Equal(suite.T(), carName2, retVal.Ms[1].(*Car).Name)
		assert.Equal(suite.T(), carName3, retVal.Ms[2].(*Car).Name)
	}
}

func (suite *TestBaseMapperReadSuite) TestReadMany_WhenNoController_CallRelevantOldCallbacks() {
	carID1 := datatypes.NewUUID()
	carName1 := "DSM"
	carID2 := datatypes.NewUUID()
	carName2 := "DSM4Life"
	carID3 := datatypes.NewUUID()
	carName3 := "Eclipse"
	roles := []models.UserRole{models.UserRoleAdmin, models.UserRoleGuest, models.UserRoleAdmin}

	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $1 WHERE "car"."deleted_at" IS NULL ORDER BY "car".created_at DESC LIMIT 100 OFFSET 0`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID1, carName1).AddRow(carID2, carName2).AddRow(carID3, carName3))

	stmt2 := `SELECT "user_owns_car"."role" FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $1 WHERE ("car"."deleted_at" IS NULL) ORDER BY "car".created_at DESC LIMIT 100 OFFSET 0`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(roles[0]).AddRow(roles[1]).AddRow(roles[2]))

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

	var afterReadCalled bool
	var afterReadData models.BatchHookPointData
	afterRead := createBatchSingleMethodHookPoint(&afterReadCalled, &afterReadData)

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).
		BatchCRUPDHooks(before, after).BatchReadHooks(afterRead)

	mapper := SharedOwnershipMapper()
	_, _, _, retErr := mapper.ReadMany(suite.db, suite.who, suite.typeString, options, &cargo)
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	// Expected
	expectedData := models.BatchHookPointData{
		Ms: []models.IModel{
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carName1},
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carName2},
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carName3},
		},
		DB: suite.db, Who: suite.who, TypeString: suite.typeString, Roles: roles, URLParams: options,
		Cargo: &models.BatchHookCargo{Payload: cargo.Payload},
	}

	assert.False(suite.T(), beforeCalled) // before is not called on read

	if assert.True(suite.T(), afterCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &afterData))
		assert.Equal(suite.T(), afterOp, models.CRUPDOpRead)
	}

	if assert.True(suite.T(), afterReadCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &afterReadData))
	}
}

func (suite *TestBaseMapperReadSuite) TestReadMany_WhenHavingController_NotCallOldCallbacks() {
	carID1 := datatypes.NewUUID()
	carName1 := "DSM"
	carID2 := datatypes.NewUUID()
	carName2 := "DSM4Life"
	carID3 := datatypes.NewUUID()
	carName3 := "Eclipse"
	roles := []models.UserRole{models.UserRoleAdmin, models.UserRoleGuest, models.UserRoleAdmin}

	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $1 WHERE "car"."deleted_at" IS NULL ORDER BY "car".created_at DESC LIMIT 100 OFFSET 0`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID1, carName1).AddRow(carID2, carName2).AddRow(carID3, carName3))

	stmt2 := `SELECT "user_owns_car"."role" FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $1 WHERE ("car"."deleted_at" IS NULL) ORDER BY "car".created_at DESC LIMIT 100 OFFSET 0`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(roles[0]).AddRow(roles[1]).AddRow(roles[2]))

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

	var afterReadCalled bool
	var afterReadData models.BatchHookPointData
	afterRead := createBatchSingleMethodHookPoint(&afterReadCalled, &afterReadData)

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}

	hdlr := CarHandlerJBT{}

	// Both old and new are given
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).
		BatchCRUPDHooks(before, after).BatchReadHooks(afterRead).HookHandler(&hdlr, "CRUPD")

	mapper := SharedOwnershipMapper()
	_, _, _, retErr := mapper.ReadMany(suite.db, suite.who, suite.typeString, options, &cargo)
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	// Old hookpoint should not be called at all
	assert.False(suite.T(), beforeCalled) // before is not called on read
	assert.False(suite.T(), afterCalled)
	assert.False(suite.T(), afterReadCalled)
}

func (suite *TestBaseMapperReadSuite) TestReadMany_WhenHavingController_CallRelevantControllerCallbacks() {
	carID1 := datatypes.NewUUID()
	carName1 := "DSM"
	carID2 := datatypes.NewUUID()
	carName2 := "DSM4Life"
	carID3 := datatypes.NewUUID()
	carName3 := "Eclipse"
	roles := []models.UserRole{models.UserRoleAdmin, models.UserRoleGuest, models.UserRoleAdmin}

	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $1 WHERE "car"."deleted_at" IS NULL ORDER BY "car".created_at DESC LIMIT 100 OFFSET 0`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID1, carName1).AddRow(carID2, carName2).AddRow(carID3, carName3))

	stmt2 := `SELECT "user_owns_car"."role" FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $1 WHERE ("car"."deleted_at" IS NULL) ORDER BY "car".created_at DESC LIMIT 100 OFFSET 0`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(roles[0]).AddRow(roles[1]).AddRow(roles[2]))

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}

	// Both old and new are given
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).HookHandler(&CarHandlerJBT{}, "CRUPD")

	var retVal *MapperRet
	mapper := SharedOwnershipMapper()
	retVal, _, _, retErr := mapper.ReadMany(suite.db, suite.who, suite.typeString, options, &cargo)
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	data := hookhandler.Data{
		Ms: []models.IModel{
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carName1},
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carName2},
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carName3},
		},
		DB: suite.db, Who: suite.who,
		TypeString: suite.typeString,
		Roles:      roles, Cargo: &cargo,
	}
	info := hookhandler.EndPointInfo{Op: hookhandler.RESTOpRead, Cardinality: hookhandler.APICardinalityMany}

	ctrls := retVal.Fetcher.GetAllInstantiatedHanders()
	if !assert.Len(suite.T(), ctrls, 1) {
		return
	}

	hdlr, ok := ctrls[0].(*CarHandlerJBT)
	if !assert.True(suite.T(), ok) {
		return
	}

	assert.False(suite.T(), hdlr.beforeCalled)
	if assert.True(suite.T(), hdlr.afterCalled) {
		assert.Condition(suite.T(), dataComparison(&data, hdlr.afterData))
		assert.Equal(suite.T(), info, *hdlr.afterInfo)
	}
}

func TestBaseMappingReadSuite(t *testing.T) {
	suite.Run(t, new(TestBaseMapperReadSuite))
}
