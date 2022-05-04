package datamapper

import (
	"fmt"
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

type TestBaseMapperPatchSuite struct {
	suite.Suite
	db         *gorm.DB
	mock       sqlmock.Sqlmock
	who        models.UserIDFetchable
	typeString string
}

func (suite *TestBaseMapperPatchSuite) SetupTest() {
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

// func (suite *TestBaseMapperPatchSuite) Test_should_fail() {
// 	assert.Equal(suite.T(), 1, 2)
// }

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestBaseMapperPatchSuite) TestPatchOne_WhenGiven_GotCar() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	carNameNew := "DSM New"
	var modelObj models.IModel = &Car{BaseModel: models.BaseModel{ID: carID}, Name: carName}

	// The first three SQL probably could be made into one
	suite.mock.ExpectBegin()
	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
	stmt2 := `SELECT * FROM "user_owns_car" WHERE (user_id = $1 AND model_id = $2)`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	stmt3 := `UPDATE "car" SET "updated_at" = $1, "deleted_at" = $2, "name" = $3  WHERE "car"."id" = $4`
	result := sqlmock.NewResult(0, 1)
	// WithArgs (how do I test what gorm insert when date can be arbitrary? Use hooks?)
	suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
	// suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WithArgs(carID, carNameNew).WillReturnResult(result)
	// These two queries can be made into one as well (or with returning, all 5 can be in 1?)
	stmt4 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt4)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carNameNew))
	stmt5 := `SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt5)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	mapper := SharedOwnershipMapper()

	var jsonPatch = []byte(fmt.Sprintf(`[{
		"op": "replace", "path": "/name", "value": "%s"
	}]`, carNameNew))

	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		if retVal, retErr = mapper.PatchOne(tx, suite.who, suite.typeString, jsonPatch, modelObj.GetID(),
			options, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.PatchOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	if car, ok := retVal.Ms[0].(*Car); assert.True(suite.T(), ok) {
		assert.Equal(suite.T(), carNameNew, car.Name)
		assert.Equal(suite.T(), carID, car.ID)
	}
}

func (suite *TestBaseMapperPatchSuite) TestPatchOne_WhenNoController_CallRelevantOldCallbacks() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	carNameNew := "DSM New"
	var modelObj models.IModel = &CarWithCallbacks{BaseModel: models.BaseModel{ID: carID}, Name: carName}

	// The first three SQL probably could be made into one
	suite.mock.ExpectBegin()
	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
	stmt2 := `SELECT * FROM "user_owns_car" WHERE (user_id = $1 AND model_id = $2)`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	stmt3 := `UPDATE "car" SET "updated_at" = $1, "deleted_at" = $2, "name" = $3  WHERE "car"."id" = $4`
	result := sqlmock.NewResult(0, 1)
	// WithArgs (how do I test what gorm insert when date can be arbitrary? Use hooks?)
	suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
	// suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WithArgs(carID, carNameNew).WillReturnResult(result)
	// These two queries can be made into one as well (or with returning, all 5 can be in 1?)
	stmt4 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt4)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carNameNew))
	stmt5 := `SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt5)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt)

	var jsonPatch = []byte(fmt.Sprintf(`[{
		"op": "replace", "path": "/name", "value": "%s"
	}]`, carNameNew))

	var tx2 *gorm.DB
	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		if retVal, retErr = mapper.PatchOne(tx2, suite.who, suite.typeString, jsonPatch, modelObj.GetID(),
			options, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.PatchOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	role := models.UserRoleAdmin
	hpdata := models.HookPointData{DB: tx2, Who: suite.who, TypeString: suite.typeString,
		Cargo: &models.ModelCargo{Payload: cargo.Payload}, Role: &role, URLParams: options}

	// No, update is not easy to test because I load the obj from the db first, and it's not the
	// same as the car object I have now (all the more reason hookhandler make more sense)

	if _, ok := retVal.Ms[0].(*CarWithCallbacks); assert.True(suite.T(), ok) {
		assert.False(suite.T(), guardAPIEntryCalled) // not called when going through mapper
		if assert.True(suite.T(), beforeCUPDDBCalled) {
			assert.Equal(suite.T(), beforeCUPDDBOp, models.CRUPDOpPatch)
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &beforeCUPDDBHpdata))
		}

		if assert.True(suite.T(), beforePatchDBCalled) {
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &beforePatchDBHpdata))
		}

		if assert.True(suite.T(), afterCRUPDDBCalled) {
			assert.Equal(suite.T(), afterCRUPDDBOp, models.CRUPDOpPatch)
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &afterCRUPDDBHpdata))
		}

		if assert.True(suite.T(), afterPatchDBCalled) {
			assert.Condition(suite.T(), hpDataComparison(&hpdata, &afterCRUPDDBHpdata))
		}
	}
}

func (suite *TestBaseMapperPatchSuite) TestPatchOne_WhenHavingController_NotCallOldCallbacks() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	carNameNew := "DSM New"
	var modelObj models.IModel = &CarWithCallbacks{BaseModel: models.BaseModel{ID: carID}, Name: carName}

	// The first three SQL probably could be made into one
	suite.mock.ExpectBegin()
	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
	stmt2 := `SELECT * FROM "user_owns_car" WHERE (user_id = $1 AND model_id = $2)`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	stmt3 := `UPDATE "car" SET "updated_at" = $1, "deleted_at" = $2, "name" = $3  WHERE "car"."id" = $4`
	result := sqlmock.NewResult(0, 1)
	// WithArgs (how do I test what gorm insert when date can be arbitrary? Use hooks?)
	suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
	// suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WithArgs(carID, carNameNew).WillReturnResult(result)
	// These two queries can be made into one as well (or with returning, all 5 can be in 1?)
	stmt4 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt4)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carNameNew))
	stmt5 := `SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt5)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	hdlr := CarControllerWithoutCallbacks{}
	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).HookHandler(&hdlr, "CRUPD")

	var jsonPatch = []byte(fmt.Sprintf(`[{
		"op": "replace", "path": "/name", "value": "%s"
	}]`, carNameNew))

	var tx2 *gorm.DB
	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		if retVal, retErr = mapper.PatchOne(tx2, suite.who, suite.typeString, jsonPatch, modelObj.GetID(),
			options, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.PatchOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	if _, ok := retVal.Ms[0].(*CarWithCallbacks); assert.True(suite.T(), ok) {
		assert.False(suite.T(), guardAPIEntryCalled) // not called when going through mapper
		assert.False(suite.T(), beforeCUPDDBCalled)
		assert.False(suite.T(), beforePatchDBCalled)
		assert.False(suite.T(), afterCRUPDDBCalled)
		assert.False(suite.T(), afterPatchDBCalled)
	}
}

func (suite *TestBaseMapperPatchSuite) TestPatchOne_WhenHavingController_CallRelevantControllerCallbacks() {
	carID := datatypes.NewUUID()
	carName := "DSM"
	carNameNew := "DSM New"
	var modelObj models.IModel = &CarWithCallbacks{BaseModel: models.BaseModel{ID: carID}, Name: carName}

	// The first three SQL probably could be made into one
	suite.mock.ExpectBegin()
	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
	stmt2 := `SELECT * FROM "user_owns_car" WHERE (user_id = $1 AND model_id = $2)`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	stmt3 := `UPDATE "car" SET "updated_at" = $1, "deleted_at" = $2, "name" = $3  WHERE "car"."id" = $4`
	result := sqlmock.NewResult(0, 1)
	// WithArgs (how do I test what gorm insert when date can be arbitrary? Use hooks?)
	suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
	// suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WithArgs(carID, carNameNew).WillReturnResult(result)
	// These two queries can be made into one as well (or with returning, all 5 can be in 1?)
	stmt4 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt4)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carNameNew))
	stmt5 := `SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt5)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).HookHandler(&CarHandlerJBT{}, "CRUPD")

	jsonPatch := []byte(fmt.Sprintf(`[{
		"op": "replace", "path": "/name", "value": "%s"
	}]`, carNameNew))

	var tx2 *gorm.DB
	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		if retVal, retErr = mapper.PatchOne(tx2, suite.who, suite.typeString, jsonPatch, modelObj.GetID(),
			options, &cargo); retErr != nil {
			return retErr
		}
		return nil
	}, "lifecycle.PatchOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	role := models.UserRoleAdmin
	dataBeforeApply := hookhandler.Data{Ms: []models.IModel{&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID},
		Name: carName}}, DB: tx2, Who: suite.who,
		TypeString: suite.typeString, Roles: []models.UserRole{role}, Cargo: &cargo}

	data := hookhandler.Data{Ms: []models.IModel{&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID},
		Name: carNameNew}}, DB: tx2, Who: suite.who,
		TypeString: suite.typeString, Roles: []models.UserRole{role}, Cargo: &cargo}

	info := hookhandler.EndPointInfo{Op: hookhandler.RESTOpPatch, Cardinality: hookhandler.APICardinalityOne}

	ctrls := retVal.Fetcher.GetAllInstantiatedHanders()
	if !assert.Len(suite.T(), ctrls, 1) {
		return
	}

	hdlr, ok := ctrls[0].(*CarHandlerJBT)
	if !assert.True(suite.T(), ok) {
		return
	}

	assert.False(suite.T(), hdlr.guardAPIEntryCalled) // Not called when going through mapper (or lifecycle for that matter)

	if assert.True(suite.T(), hdlr.beforeApplyCalled) {
		assert.Equal(suite.T(), info, *hdlr.beforeApplyInfo)
		assert.Condition(suite.T(), dataComparison(&dataBeforeApply, hdlr.beforeApplyData))
	}

	if assert.True(suite.T(), hdlr.beforeCalled) {
		assert.Equal(suite.T(), info, *hdlr.beforeInfo)
		assert.Condition(suite.T(), dataComparison(&data, hdlr.beforeData))
		assert.Equal(suite.T(), info.Op, hdlr.beforeInfo.Op)
		assert.Equal(suite.T(), info, *hdlr.beforeInfo)
	}

	if assert.True(suite.T(), hdlr.afterCalled) {
		assert.Condition(suite.T(), dataComparison(&data, hdlr.afterData))
		assert.Equal(suite.T(), info.Op, hdlr.afterInfo.Op)
		assert.Equal(suite.T(), info.Cardinality, hdlr.afterInfo.Cardinality)
		assert.Equal(suite.T(), info, *hdlr.afterInfo)
	}
}

func (suite *TestBaseMapperPatchSuite) TestPatchMany_WhenGiven_GotCars() {
	carID1 := datatypes.NewUUID()
	carName1 := "DSM"
	carNameNew1 := "DSM New"
	carID2 := datatypes.NewUUID()
	carName2 := "DSM4Life"
	carNameNew2 := "DSM4Life New"
	carID3 := datatypes.NewUUID()
	carName3 := "Eclipse"
	carNameNew3 := "Eclipse New"
	carIDs := []*datatypes.UUID{carID1, carID2, carID3}
	carNamesNew := []string{carNameNew1, carNameNew2, carNameNew3}

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
		carID, carName := carIDs[i], carNamesNew[i]
		stmt3 := `UPDATE "car" SET "updated_at" = $1, "deleted_at" = $2, "name" = $3  WHERE "car"."id" = $4`
		result := sqlmock.NewResult(0, 1)
		suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
		stmt4 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2`
		suite.mock.ExpectQuery(regexp.QuoteMeta(stmt4)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
		stmt5 := `SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`
		suite.mock.ExpectQuery(regexp.QuoteMeta(stmt5)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	}
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	jsonPatches := []models.JSONIDPatch{
		{
			ID: carID1,
			Patch: []byte(fmt.Sprintf(`[{
				"op": "replace", "path": "/name", "value": "%s"
			}]`, carNameNew1)),
		},
		{
			ID: carID2,
			Patch: []byte(fmt.Sprintf(`[{
				"op": "replace", "path": "/name", "value": "%s"
			}]`, carNameNew2)),
		},
		{
			ID: carID3,
			Patch: []byte(fmt.Sprintf(`[{
				"op": "replace", "path": "/name", "value": "%s"
			}]`, carNameNew3)),
		},
	}

	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		mapper := SharedOwnershipMapper()
		retVal, retErr = mapper.PatchMany(tx, suite.who, suite.typeString, jsonPatches, options, &cargo)
		return retErr
	}, "lifecycle.PatchMany")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	if assert.Len(suite.T(), retVal.Ms, 3) {
		assert.Equal(suite.T(), carID1.String(), retVal.Ms[0].GetID().String())
		assert.Equal(suite.T(), carID2.String(), retVal.Ms[1].GetID().String())
		assert.Equal(suite.T(), carID3.String(), retVal.Ms[2].GetID().String())
		assert.Equal(suite.T(), carNameNew1, retVal.Ms[0].(*Car).Name)
		assert.Equal(suite.T(), carNameNew2, retVal.Ms[1].(*Car).Name)
		assert.Equal(suite.T(), carNameNew3, retVal.Ms[2].(*Car).Name)
	}
}

func (suite *TestBaseMapperPatchSuite) TestPatchMany_WhenNoController_CallRelevantOldCallbacks() {
	carID1 := datatypes.NewUUID()
	carName1 := "DSM"
	carNameNew1 := "DSM New"
	carID2 := datatypes.NewUUID()
	carName2 := "DSM4Life"
	carNameNew2 := "DSM4Life New"
	carID3 := datatypes.NewUUID()
	carName3 := "Eclipse"
	carNameNew3 := "Eclipse New"
	carIDs := []*datatypes.UUID{carID1, carID2, carID3}
	carNamesNew := []string{carNameNew1, carNameNew2, carNameNew3}

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
		carID, carName := carIDs[i], carNamesNew[i]
		stmt3 := `UPDATE "car" SET "updated_at" = $1, "deleted_at" = $2, "name" = $3  WHERE "car"."id" = $4`
		result := sqlmock.NewResult(0, 1)
		suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
		stmt4 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2`
		suite.mock.ExpectQuery(regexp.QuoteMeta(stmt4)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
		stmt5 := `SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`
		suite.mock.ExpectQuery(regexp.QuoteMeta(stmt5)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
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

	var beforeApplyCalled bool
	var beforeApplyData models.BatchHookPointData
	beforeApply := createBatchSingleMethodHookPoint(&beforeApplyCalled, &beforeApplyData)

	var beforePatchCalled bool
	var beforePatchData models.BatchHookPointData
	beforePatch := createBatchSingleMethodHookPoint(&beforePatchCalled, &beforePatchData)

	var afterPatchCalled bool
	var afterPatchData models.BatchHookPointData
	afterPatch := createBatchSingleMethodHookPoint(&afterPatchCalled, &afterPatchData)

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).BatchCRUPDHooks(before, after).
		BatchPatchHooks(beforeApply, beforePatch, afterPatch)

	jsonPatches := []models.JSONIDPatch{
		{
			ID: carID1,
			Patch: []byte(fmt.Sprintf(`[{
					"op": "replace", "path": "/name", "value": "%s"
				}]`, carNameNew1)),
		},
		{
			ID: carID2,
			Patch: []byte(fmt.Sprintf(`[{
					"op": "replace", "path": "/name", "value": "%s"
				}]`, carNameNew2)),
		},
		{
			ID: carID3,
			Patch: []byte(fmt.Sprintf(`[{
					"op": "replace", "path": "/name", "value": "%s"
				}]`, carNameNew3)),
		},
	}

	var tx2 *gorm.DB
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		_, retErr = mapper.PatchMany(tx2, suite.who, suite.typeString, jsonPatches, options, &cargo)
		return retErr
	}, "lifecycle.PatchMany")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	roles := []models.UserRole{models.UserRoleAdmin, models.UserRoleAdmin, models.UserRoleAdmin}

	// Expected
	expectedBeforeApplyData := models.BatchHookPointData{
		Ms: []models.IModel{
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carName1},
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carName2},
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carName3},
		},
		DB: tx2, Who: suite.who, TypeString: suite.typeString, Roles: roles, URLParams: options,
		Cargo: &models.BatchHookCargo{Payload: cargo.Payload},
	}

	expectedData := models.BatchHookPointData{
		Ms: []models.IModel{
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carNameNew1},
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carNameNew2},
			&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carNameNew3},
		},
		DB: tx2, Who: suite.who, TypeString: suite.typeString, Roles: roles, URLParams: options,
		Cargo: &models.BatchHookCargo{Payload: cargo.Payload},
	}

	if assert.True(suite.T(), beforeApplyCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedBeforeApplyData, &beforeApplyData))
		assert.Equal(suite.T(), beforeOp, models.CRUPDOpPatch)
	}

	if assert.True(suite.T(), beforeCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &beforeData))
		assert.Equal(suite.T(), beforeOp, models.CRUPDOpPatch)
	}

	if assert.True(suite.T(), beforePatchCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &beforePatchData))
	}

	if assert.True(suite.T(), afterCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &afterData))
		assert.Equal(suite.T(), afterOp, models.CRUPDOpPatch)
	}

	if assert.True(suite.T(), afterPatchCalled) {
		assert.Condition(suite.T(), bhpDataComparison(&expectedData, &afterPatchData))
	}
}

func (suite *TestBaseMapperPatchSuite) TestCreateMany_WhenHavingController_NotCallOldCallbacks() {
	carID1 := datatypes.NewUUID()
	carName1 := "DSM"
	carNameNew1 := "DSM New"
	carID2 := datatypes.NewUUID()
	carName2 := "DSM4Life"
	carNameNew2 := "DSM4Life New"
	carID3 := datatypes.NewUUID()
	carName3 := "Eclipse"
	carNameNew3 := "Eclipse New"
	carIDs := []*datatypes.UUID{carID1, carID2, carID3}
	carNamesNew := []string{carNameNew1, carNameNew2, carNameNew3}

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
		carID, carName := carIDs[i], carNamesNew[i]
		stmt3 := `UPDATE "car" SET "updated_at" = $1, "deleted_at" = $2, "name" = $3  WHERE "car"."id" = $4`
		result := sqlmock.NewResult(0, 1)
		suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
		stmt4 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2`
		suite.mock.ExpectQuery(regexp.QuoteMeta(stmt4)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
		stmt5 := `SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`
		suite.mock.ExpectQuery(regexp.QuoteMeta(stmt5)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
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

	var beforePatchApplyCalled bool
	var beforePatchApplyData models.BatchHookPointData
	beforeApplyPatch := createBatchSingleMethodHookPoint(&beforePatchApplyCalled, &beforePatchApplyData)

	var beforePatchCalled bool
	var beforePatchData models.BatchHookPointData
	beforePatch := createBatchSingleMethodHookPoint(&beforePatchCalled, &beforePatchData)

	var afterPatchCalled bool
	var afterPatchData models.BatchHookPointData
	afterPatch := createBatchSingleMethodHookPoint(&afterPatchCalled, &afterPatchData)

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	hdlr := CarControllerWithoutCallbacks{}
	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).BatchCRUPDHooks(before, after).
		BatchPatchHooks(beforeApplyPatch, beforePatch, afterPatch).HookHandler(&hdlr, "CRUPD")

	jsonPatches := []models.JSONIDPatch{
		{
			ID: carID1,
			Patch: []byte(fmt.Sprintf(`[{
						"op": "replace", "path": "/name", "value": "%s"
					}]`, carNameNew1)),
		},
		{
			ID: carID2,
			Patch: []byte(fmt.Sprintf(`[{
						"op": "replace", "path": "/name", "value": "%s"
					}]`, carNameNew2)),
		},
		{
			ID: carID3,
			Patch: []byte(fmt.Sprintf(`[{
						"op": "replace", "path": "/name", "value": "%s"
					}]`, carNameNew3)),
		},
	}

	var tx2 *gorm.DB
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		_, retErr = mapper.PatchMany(tx2, suite.who, suite.typeString, jsonPatches, options, &cargo)
		return retErr
	}, "lifecycle.PatchMany")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	assert.False(suite.T(), beforePatchApplyCalled)
	assert.False(suite.T(), beforeCalled)
	assert.False(suite.T(), beforePatchCalled)
	assert.False(suite.T(), afterCalled)
	assert.False(suite.T(), afterPatchCalled)
}

func (suite *TestBaseMapperPatchSuite) TestCreateMany_WhenHavingController_CallRelevantControllerCallbacks() {
	carID1 := datatypes.NewUUID()
	carName1 := "DSM"
	carNameNew1 := "DSM New"
	carID2 := datatypes.NewUUID()
	carName2 := "DSM4Life"
	carNameNew2 := "DSM4Life New"
	carID3 := datatypes.NewUUID()
	carName3 := "Eclipse"
	carNameNew3 := "Eclipse New"
	carIDs := []*datatypes.UUID{carID1, carID2, carID3}
	carNamesNew := []string{carNameNew1, carNameNew2, carNameNew3}

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
		carID, carName := carIDs[i], carNamesNew[i]
		stmt3 := `UPDATE "car" SET "updated_at" = $1, "deleted_at" = $2, "name" = $3  WHERE "car"."id" = $4`
		result := sqlmock.NewResult(0, 1)
		suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
		stmt4 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2`
		suite.mock.ExpectQuery(regexp.QuoteMeta(stmt4)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
		stmt5 := `SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`
		suite.mock.ExpectQuery(regexp.QuoteMeta(stmt5)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), models.UserRoleAdmin))
	}
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hookhandler.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: registry.MapperTypeViaOwnership}
	registry.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).HookHandler(&CarHandlerJBT{}, "CRUPD")

	jsonPatches := []models.JSONIDPatch{
		{
			ID: carID1,
			Patch: []byte(fmt.Sprintf(`[{
						"op": "replace", "path": "/name", "value": "%s"
					}]`, carNameNew1)),
		},
		{
			ID: carID2,
			Patch: []byte(fmt.Sprintf(`[{
						"op": "replace", "path": "/name", "value": "%s"
					}]`, carNameNew2)),
		},
		{
			ID: carID3,
			Patch: []byte(fmt.Sprintf(`[{
						"op": "replace", "path": "/name", "value": "%s"
					}]`, carNameNew3)),
		},
	}

	var tx2 *gorm.DB
	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		retVal, retErr = mapper.PatchMany(tx2, suite.who, suite.typeString, jsonPatches, options, &cargo)
		return retErr
	}, "lifecycle.PatchMany")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	// Expected
	roles := []models.UserRole{models.UserRoleAdmin, models.UserRoleAdmin, models.UserRoleAdmin}
	dataBeforePatch := hookhandler.Data{Ms: []models.IModel{
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carName1},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carName2},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carName3},
	}, DB: tx2, Who: suite.who, TypeString: suite.typeString, Roles: roles, Cargo: &cargo}
	data := hookhandler.Data{Ms: []models.IModel{
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carNameNew1},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carNameNew2},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carNameNew3},
	}, DB: tx2, Who: suite.who, TypeString: suite.typeString, Roles: roles, Cargo: &cargo}
	info := hookhandler.EndPointInfo{Op: hookhandler.RESTOpPatch, Cardinality: hookhandler.APICardinalityMany}

	ctrls := retVal.Fetcher.GetAllInstantiatedHanders()
	if !assert.Len(suite.T(), ctrls, 1) {
		return
	}

	hdlr, ok := ctrls[0].(*CarHandlerJBT)
	if !assert.True(suite.T(), ok) {
		return
	}

	assert.False(suite.T(), hdlr.guardAPIEntryCalled) // not called when call createMany directly
	if assert.True(suite.T(), hdlr.beforeApplyCalled) {
		assert.Condition(suite.T(), dataComparison(&dataBeforePatch, hdlr.beforeApplyData))
		assert.Equal(suite.T(), info.Op, hdlr.beforeInfo.Op)
		assert.Equal(suite.T(), info.Cardinality, hdlr.beforeInfo.Cardinality)
	}

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

func TestBaseMappingPatchSuite(t *testing.T) {
	suite.Run(t, new(TestBaseMapperPatchSuite))
}
