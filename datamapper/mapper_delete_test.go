package datamapper

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/rest"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/utils/transact"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/registry"
)

// ----------------------------------------------------------------------------------------------

type TestBaseMapperDeleteSuite struct {
	suite.Suite
	db         *gorm.DB
	mock       sqlmock.Sqlmock
	who        models.UserIDFetchable
	typeString string
}

func (suite *TestBaseMapperDeleteSuite) SetupTest() {
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
func (suite *TestBaseMapperDeleteSuite) TestDeleteOne_WhenGiven_GotCar() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	var modelObj models.IModel = &Car{BaseModel: models.BaseModel{ID: carID}, Name: carName}

	// The first three SQL probably could be made into one
	suite.mock.ExpectBegin()
	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
	stmt2 := `SELECT * FROM "user_owns_car" WHERE (user_id = $1 AND model_id = $2)`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	stmt3 := `DELETE FROM user_owns_car WHERE model_id = $1`
	result := sqlmock.NewResult(0, 1)
	suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
	stmt4 := `DELETE FROM "car"  WHERE "car"."id" = $1`
	suite.mock.ExpectExec(regexp.QuoteMeta(stmt4)).WillReturnResult(result)
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	mapper := SharedOwnershipMapper()

	var retVal *MapperRet
	ep := hook.EndPoint{
		TypeString: suite.typeString,
		URLParams:  options,
		Who:        suite.who,
	}
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		if retVal, retErr = mapper.DeleteOne(tx, modelObj.GetID(), &ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.DeleteOne")

	if !assert.Nil(suite.T(), retErr) {
		return
	}

	if car, ok := retVal.Ms[0].(*Car); assert.True(suite.T(), ok) {
		assert.Equal(suite.T(), carName, car.Name)
		assert.Equal(suite.T(), carID, car.ID)
	}
}

func (suite *TestBaseMapperDeleteSuite) TestDeleteOne_WhenNoController_CallRelevantOldCallbacks() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	var modelObj models.IModel = &Car{BaseModel: models.BaseModel{ID: carID}, Name: carName}

	// The first three SQL probably could be made into one
	suite.mock.ExpectBegin()
	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
	stmt2 := `SELECT * FROM "user_owns_car" WHERE (user_id = $1 AND model_id = $2)`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	stmt3 := `DELETE FROM user_owns_car WHERE model_id = $1`
	result := sqlmock.NewResult(0, 1)
	suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
	stmt4 := `DELETE FROM "car"  WHERE "car"."id" = $1`
	suite.mock.ExpectExec(regexp.QuoteMeta(stmt4)).WillReturnResult(result)
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt)

	mapper := SharedOwnershipMapper()
	var retVal *MapperRet
	var tx2 *gorm.DB
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		ep := hook.EndPoint{
			Op:          rest.OpDelete,
			Cardinality: rest.CardinalityOne,
			TypeString:  suite.typeString,
			URLParams:   options,
			Who:         suite.who,
		}
		if retVal, retErr = mapper.DeleteOne(tx2, modelObj.GetID(), &ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.DeleteOne")

	if !assert.Nil(suite.T(), retErr) {
		return
	}

	role := models.UserRoleAdmin
	hpdata := models.HookPointData{DB: tx2, Who: suite.who, TypeString: suite.typeString,
		Cargo: &models.ModelCargo{Payload: cargo.Payload}, Role: &role, URLParams: options}

	// No, update is not easy to test because I load the obj from the db first, and it's not the
	// same as the car object I have now (all the more reason hook make more sense)
	if _, ok := retVal.Ms[0].(*CarWithCallbacks); assert.True(suite.T(), ok) {
		assert.False(suite.T(), guardAPIEntryCalled) // not called when going through mapper

		if assert.True(suite.T(), beforeCUPDDBCalled) {
			assert.Equal(suite.T(), beforeCUPDDBOp, models.CRUPDOpDelete)
			assert.Condition(suite.T(), hpDataComparisonNoDB(&hpdata, &beforeCUPDDBHpdata))
		}

		if assert.True(suite.T(), beforeDeleteDBCalled) {
			assert.Condition(suite.T(), hpDataComparisonNoDB(&hpdata, &beforeDeleteDBHpdata))
		}

		if assert.True(suite.T(), afterCRUPDDBCalled) {
			assert.Equal(suite.T(), afterCRUPDDBOp, models.CRUPDOpDelete)
			assert.Condition(suite.T(), hpDataComparisonNoDB(&hpdata, &afterCRUPDDBHpdata))
		}

		if assert.True(suite.T(), afterDeleteDBCalled) {
			assert.Condition(suite.T(), hpDataComparisonNoDB(&hpdata, &afterCRUPDDBHpdata))
		}
	}
}

func (suite *TestBaseMapperDeleteSuite) TestDeleteOne_WhenHavingController_NotCallOldCallbacks() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	var modelObj models.IModel = &Car{BaseModel: models.BaseModel{ID: carID}, Name: carName}

	// The first three SQL probably could be made into one
	suite.mock.ExpectBegin()
	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
	stmt2 := `SELECT * FROM "user_owns_car" WHERE (user_id = $1 AND model_id = $2)`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	stmt3 := `DELETE FROM user_owns_car WHERE model_id = $1`
	result := sqlmock.NewResult(0, 1)
	suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
	stmt4 := `DELETE FROM "car"  WHERE "car"."id" = $1`
	suite.mock.ExpectExec(regexp.QuoteMeta(stmt4)).WillReturnResult(result)
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).Hook(&CarControllerWithoutCallbacks{}, "CRUPD")

	mapper := SharedOwnershipMapper()

	var retVal *MapperRet
	var tx2 *gorm.DB
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		ep := hook.EndPoint{
			Op:          rest.OpDelete,
			Cardinality: rest.CardinalityOne,
			TypeString:  suite.typeString,
			URLParams:   options,
			Who:         suite.who,
		}
		if retVal, retErr = mapper.DeleteOne(tx2, modelObj.GetID(), &ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.DeleteOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	if _, ok := retVal.Ms[0].(*CarWithCallbacks); assert.True(suite.T(), ok) {
		assert.False(suite.T(), guardAPIEntryCalled) // not called when going through mapper
		assert.False(suite.T(), beforeCUPDDBCalled)
		assert.False(suite.T(), beforeDeleteDBCalled)
		assert.False(suite.T(), afterCRUPDDBCalled)
		assert.False(suite.T(), afterDeleteDBCalled)
	}
}

func (suite *TestBaseMapperDeleteSuite) TestDeleteOne_WhenHavingController_CallRelevantControllerCallbacks() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	var modelObj models.IModel = &Car{BaseModel: models.BaseModel{ID: carID}, Name: carName}

	// The first three SQL probably could be made into one
	suite.mock.ExpectBegin()
	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
	stmt2 := `SELECT * FROM "user_owns_car" WHERE (user_id = $1 AND model_id = $2)`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	stmt3 := `DELETE FROM user_owns_car WHERE model_id = $1`
	result := sqlmock.NewResult(0, 1)
	suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
	stmt4 := `DELETE FROM "car"  WHERE "car"."id" = $1`
	suite.mock.ExpectExec(regexp.QuoteMeta(stmt4)).WillReturnResult(result)
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).Hook(&CarHandlerJBT{}, "CRUPD")

	mapper := SharedOwnershipMapper()

	var tx2 *gorm.DB
	var retVal *MapperRet
	ep := hook.EndPoint{
		Op:          rest.OpDelete,
		Cardinality: rest.CardinalityOne,
		TypeString:  suite.typeString,
		URLParams:   options,
		Who:         suite.who,
	}
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		if retVal, retErr = mapper.DeleteOne(tx2, modelObj.GetID(), &ep, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.DeleteOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	role := models.UserRoleAdmin
	data := hook.Data{Ms: []models.IModel{&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID}, Name: carName}},
		DB: tx2, Roles: []models.UserRole{role}, Cargo: &cargo}

	ctrls := retVal.Fetcher.GetAllInstantiatedHanders()
	if !assert.Len(suite.T(), ctrls, 1) {
		return
	}

	hdlr, ok := ctrls[0].(*CarHandlerJBT)
	if !assert.True(suite.T(), ok) {
		return
	}

	assert.False(suite.T(), hdlr.guardAPIEntryCalled) // Not called when going through mapper (or lifecycle for that matter)

	if assert.True(suite.T(), hdlr.beforeCalled) {
		assert.Equal(suite.T(), ep, *hdlr.beforeInfo)
		assert.Condition(suite.T(), dataComparisonNoDB(&data, hdlr.beforeData))
	}

	if assert.True(suite.T(), hdlr.afterCalled) {
		assert.Condition(suite.T(), dataComparisonNoDB(&data, hdlr.afterData))
		assert.Equal(suite.T(), ep.Cardinality, hdlr.afterInfo.Cardinality)
		assert.Equal(suite.T(), ep, *hdlr.afterInfo)
	}
}

func (suite *TestBaseMapperDeleteSuite) TestDeleteMany_WhenGiven_GotCars() {
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
	// The first three SQL probably could be made into one (well no I can't, I need to pull the old one so
	// I can call the callback...or when I can test for whether the call back exists before I optimize.)
	suite.mock.ExpectBegin()
	stmt1 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id IN ($1,$2,$3) INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $4 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID1, carName1).AddRow(carID2, carName2).AddRow(carID3, carName3))
	stmt2 := `SELECT "user_owns_car"."role" FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id IN ($1,$2,$3) INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $4`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(models.UserRoleAdmin).AddRow(models.UserRoleAdmin).AddRow(models.UserRoleAdmin))

	// Obviously not very efficient, update needs to run 3 times, but read can be done in 1 (for the update algorithm and Gorm)
	// Hard to do if we're in updateOneCore, probably have to re-write it to updateManyCore
	// It should work on single and multiple updates
	for i := 0; i < 3; i++ {
		// carID, carName := carIDs[i], carNames[i]
		stmt3 := `DELETE FROM user_owns_car WHERE model_id = $1`
		result := sqlmock.NewResult(0, 1)
		suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
	}
	for i := 0; i < 3; i++ {
		stmt4 := `DELETE FROM "car"  WHERE "car"."id" = $1`
		result := sqlmock.NewResult(0, 1)
		suite.mock.ExpectExec(regexp.QuoteMeta(stmt4)).WillReturnResult(result)
	}

	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		mapper := SharedOwnershipMapper()
		ep := hook.EndPoint{
			Op:          rest.OpDelete,
			Cardinality: rest.CardinalityMany,
			TypeString:  suite.typeString,
			URLParams:   options,
			Who:         suite.who,
		}
		retVal, retErr = mapper.DeleteMany(tx, modelObjs, &ep, &cargo)
		return retErr
	}, "lifecycle.DeleteMany")
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

func (suite *TestBaseMapperDeleteSuite) TestDeleteMany_WhenNoController_CallRelevantOldCallbacks() {
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

	// The first three SQL probably could be made into one (well no I can't, I need to pull the old one so
	// I can call the callback...or when I can test for whether the call back exists before I optimize.)
	suite.mock.ExpectBegin()
	stmt1 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id IN ($1,$2,$3) INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $4 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID1, carName1).AddRow(carID2, carName2).AddRow(carID3, carName3))
	stmt2 := `SELECT "user_owns_car"."role" FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id IN ($1,$2,$3) INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $4`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(models.UserRoleAdmin).AddRow(models.UserRoleAdmin).AddRow(models.UserRoleAdmin))

	// Obviously not very efficient, update needs to run 3 times, but read can be done in 1 (for the update algorithm and Gorm)
	// Hard to do if we're in updateOneCore, probably have to re-write it to updateManyCore
	// It should work on single and multiple updates
	for i := 0; i < 3; i++ {
		// carID, carName := carIDs[i], carNames[i]
		stmt3 := `DELETE FROM user_owns_car WHERE model_id = $1`
		result := sqlmock.NewResult(0, 1)
		suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
	}
	for i := 0; i < 3; i++ {
		stmt4 := `DELETE FROM "car"  WHERE "car"."id" = $1`
		result := sqlmock.NewResult(0, 1)
		suite.mock.ExpectExec(regexp.QuoteMeta(stmt4)).WillReturnResult(result)
	}
	suite.mock.ExpectCommit()

	var beforeCalled bool
	var beforeData models.BatchHookPointData
	var beforeOp models.CRUPDOp
	before := createBatchHookPoint(&beforeCalled, &beforeData, &beforeOp)

	var afterCalled bool
	var afterData models.BatchHookPointData
	var afterOp models.CRUPDOp
	after := createBatchHookPoint(&afterCalled, &afterData, &afterOp)

	var beforeDeleteCalled bool
	var beforeDeleteData models.BatchHookPointData
	beforeDelete := createBatchSingleMethodHookPoint(&beforeDeleteCalled, &beforeDeleteData)

	var afterDeleteCalled bool
	var afterDeleteData models.BatchHookPointData
	afterDelete := createBatchSingleMethodHookPoint(&afterDeleteCalled, &afterDeleteData)

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).BatchCRUPDHooks(before, after).
		BatchDeleteHooks(beforeDelete, afterDelete)

	var tx2 *gorm.DB
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		ep := hook.EndPoint{
			Op:          rest.OpDelete,
			Cardinality: rest.CardinalityMany,
			TypeString:  suite.typeString,
			URLParams:   options,
			Who:         suite.who,
		}
		_, retErr = mapper.DeleteMany(tx2, modelObjs, &ep, &cargo)
		return retErr
	}, "lifecycle.DeleteMany")
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
		assert.Condition(suite.T(), bhpDataComparisonNoDB(&expectedData, &beforeData))
		assert.Equal(suite.T(), beforeOp, models.CRUPDOpDelete)
	}

	if assert.True(suite.T(), beforeDeleteCalled) {
		assert.Condition(suite.T(), bhpDataComparisonNoDB(&expectedData, &beforeDeleteData))
	}

	if assert.True(suite.T(), afterCalled) {
		assert.Condition(suite.T(), bhpDataComparisonNoDB(&expectedData, &afterData))
		assert.Equal(suite.T(), afterOp, models.CRUPDOpDelete)
	}

	if assert.True(suite.T(), afterDeleteCalled) {
		assert.Condition(suite.T(), bhpDataComparisonNoDB(&expectedData, &afterDeleteData))
	}
}

func (suite *TestBaseMapperDeleteSuite) TestDeleteMany_WhenHavingController_NotCallOldCallbacks() {
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

	// The first three SQL probably could be made into one (well no I can't, I need to pull the old one so
	// I can call the callback...or when I can test for whether the call back exists before I optimize.)
	suite.mock.ExpectBegin()
	stmt1 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id IN ($1,$2,$3) INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $4 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID1, carName1).AddRow(carID2, carName2).AddRow(carID3, carName3))
	stmt2 := `SELECT "user_owns_car"."role" FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id IN ($1,$2,$3) INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $4`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(models.UserRoleAdmin).AddRow(models.UserRoleAdmin).AddRow(models.UserRoleAdmin))

	// Obviously not very efficient, update needs to run 3 times, but read can be done in 1 (for the update algorithm and Gorm)
	// Hard to do if we're in updateOneCore, probably have to re-write it to updateManyCore
	// It should work on single and multiple updates
	for i := 0; i < 3; i++ {
		// carID, carName := carIDs[i], carNames[i]
		stmt3 := `DELETE FROM user_owns_car WHERE model_id = $1`
		result := sqlmock.NewResult(0, 1)
		suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
	}
	for i := 0; i < 3; i++ {
		stmt4 := `DELETE FROM "car"  WHERE "car"."id" = $1`
		result := sqlmock.NewResult(0, 1)
		suite.mock.ExpectExec(regexp.QuoteMeta(stmt4)).WillReturnResult(result)
	}
	suite.mock.ExpectCommit()

	var beforeCalled bool
	var beforeData models.BatchHookPointData
	var beforeOp models.CRUPDOp
	before := createBatchHookPoint(&beforeCalled, &beforeData, &beforeOp)

	var afterCalled bool
	var afterData models.BatchHookPointData
	var afterOp models.CRUPDOp
	after := createBatchHookPoint(&afterCalled, &afterData, &afterOp)

	var beforeDeleteCalled bool
	var beforeDeleteData models.BatchHookPointData
	beforeDelete := createBatchSingleMethodHookPoint(&beforeDeleteCalled, &beforeDeleteData)

	var afterDeleteCalled bool
	var afterDeleteData models.BatchHookPointData
	afterDelete := createBatchSingleMethodHookPoint(&afterDeleteCalled, &afterDeleteData)

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).BatchCRUPDHooks(before, after).
		BatchDeleteHooks(beforeDelete, afterDelete).Hook(&CarControllerWithoutCallbacks{}, "CRUPD")

	var tx2 *gorm.DB
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		ep := hook.EndPoint{
			TypeString: suite.typeString,
			URLParams:  options,
			Who:        suite.who,
		}
		_, retErr = mapper.DeleteMany(tx2, modelObjs, &ep, &cargo)
		return retErr
	}, "lifecycle.DeleteMany")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	assert.False(suite.T(), beforeCalled)
	assert.False(suite.T(), beforeDeleteCalled)
	assert.False(suite.T(), afterCalled)
	assert.False(suite.T(), afterDeleteCalled)
}

func (suite *TestBaseMapperDeleteSuite) TestDeleteMany_WhenHavingController_CallRelevantControllerCallbacks() {
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

	// The first three SQL probably could be made into one (well no I can't, I need to pull the old one so
	// I can call the callback...or when I can test for whether the call back exists before I optimize.)
	suite.mock.ExpectBegin()
	stmt1 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id IN ($1,$2,$3) INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $4 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID1, carName1).AddRow(carID2, carName2).AddRow(carID3, carName3))
	stmt2 := `SELECT "user_owns_car"."role" FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id IN ($1,$2,$3) INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $4`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(models.UserRoleAdmin).AddRow(models.UserRoleAdmin).AddRow(models.UserRoleAdmin))

	// Obviously not very efficient, update needs to run 3 times, but read can be done in 1 (for the update algorithm and Gorm)
	// Hard to do if we're in updateOneCore, probably have to re-write it to updateManyCore
	// It should work on single and multiple updates
	for i := 0; i < 3; i++ {
		// carID, carName := carIDs[i], carNames[i]
		stmt3 := `DELETE FROM user_owns_car WHERE model_id = $1`
		result := sqlmock.NewResult(0, 1)
		suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
	}
	for i := 0; i < 3; i++ {
		stmt4 := `DELETE FROM "car"  WHERE "car"."id" = $1`
		result := sqlmock.NewResult(0, 1)
		suite.mock.ExpectExec(regexp.QuoteMeta(stmt4)).WillReturnResult(result)
	}
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).Hook(&CarHandlerJBT{}, "CRUPD")

	var tx2 *gorm.DB
	var retVal *MapperRet
	ep := hook.EndPoint{
		Op:          rest.OpDelete,
		Cardinality: rest.CardinalityMany,
		TypeString:  suite.typeString,
		URLParams:   options,
		Who:         suite.who,
	}
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		retVal, retErr = mapper.DeleteMany(tx2, modelObjs, &ep, &cargo)
		return retErr
	}, "lifecycle.DeleteMany")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	// Expected
	roles := []models.UserRole{models.UserRoleAdmin, models.UserRoleAdmin, models.UserRoleAdmin}
	data := hook.Data{Ms: []models.IModel{
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carName1},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carName2},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carName3},
	}, DB: tx2, Roles: roles, Cargo: &cargo}

	ctrls := retVal.Fetcher.GetAllInstantiatedHanders()
	if !assert.Len(suite.T(), ctrls, 1) {
		return
	}

	hdlr, ok := ctrls[0].(*CarHandlerJBT)
	if !assert.True(suite.T(), ok) {
		return
	}

	assert.False(suite.T(), hdlr.guardAPIEntryCalled) // not called when call createMany directly
	assert.False(suite.T(), hdlr.beforeApplyCalled)
	if assert.True(suite.T(), hdlr.beforeCalled) {
		assert.Condition(suite.T(), dataComparisonNoDB(&data, hdlr.beforeData))
		assert.Equal(suite.T(), ep.Op, hdlr.beforeInfo.Op)
		assert.Equal(suite.T(), ep.Cardinality, hdlr.beforeInfo.Cardinality)
	}
	if assert.True(suite.T(), hdlr.afterCalled) {
		assert.Condition(suite.T(), dataComparisonNoDB(&data, hdlr.afterData))
		assert.Equal(suite.T(), ep.Op, hdlr.afterInfo.Op)
		assert.Equal(suite.T(), ep.Cardinality, hdlr.afterInfo.Cardinality)
	}
}

func TestBaseMappingDeleteSuite(t *testing.T) {
	suite.Run(t, new(TestBaseMapperDeleteSuite))
}
