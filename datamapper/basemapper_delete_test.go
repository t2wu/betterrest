package datamapper

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/t2wu/betterrest/controller"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/utils/transact"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
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
	cargo := controller.Cargo{}

	opt := models.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: models.MapperTypeViaOwnership}
	models.For(suite.typeString).ModelWithOption(&Car{}, opt)

	mapper := SharedOwnershipMapper()

	var modelObj2 models.IModel
	retval := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retval *webrender.RetVal) {
		if modelObj2, retval = mapper.DeleteOne(tx, suite.who, suite.typeString, modelObj.GetID(),
			options, &cargo); retval != nil {
			return retval
		}
		return nil
	}, "lifecycle.DeleteOne")

	if !assert.Nil(suite.T(), retval) {
		return
	}

	if car, ok := modelObj2.(*Car); assert.True(suite.T(), ok) {
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
	cargo := controller.Cargo{}

	opt := models.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: models.MapperTypeViaOwnership}
	models.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt)

	mapper := SharedOwnershipMapper()
	var modelObj2 models.IModel
	var tx2 *gorm.DB
	retval := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retval *webrender.RetVal) {
		tx2 = tx
		if modelObj2, retval = mapper.DeleteOne(tx2, suite.who, suite.typeString, modelObj.GetID(),
			options, &cargo); retval != nil {
			return retval
		}
		return nil
	}, "lifecycle.DeleteOne")

	if !assert.Nil(suite.T(), retval) {
		return
	}

	role := models.UserRoleAdmin
	hpdata := models.HookPointData{DB: tx2, Who: suite.who, TypeString: suite.typeString,
		Cargo: &models.ModelCargo{Payload: cargo.Payload}, Role: &role, URLParams: options}

	// No, update is not easy to test because I load the obj from the db first, and it's not the
	// same as the car object I have now (all the more reason controller make more sense)
	if _, ok := modelObj2.(*CarWithCallbacks); assert.True(suite.T(), ok) {
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
	cargo := controller.Cargo{}

	opt := models.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: models.MapperTypeViaOwnership}
	models.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).Controller(&CarControllerWithoutCallbacks{})

	mapper := SharedOwnershipMapper()

	var modelObj2 models.IModel
	var tx2 *gorm.DB
	retval := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retval *webrender.RetVal) {
		tx2 = tx
		if modelObj2, retval = mapper.DeleteOne(tx2, suite.who, suite.typeString, modelObj.GetID(),
			options, &cargo); retval != nil {
			return retval
		}
		return nil
	}, "lifecycle.DeleteOne")
	if !assert.Nil(suite.T(), retval) {
		return
	}

	if _, ok := modelObj2.(*CarWithCallbacks); assert.True(suite.T(), ok) {
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
	cargo := controller.Cargo{}

	ctrl := CarController{}
	opt := models.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: models.MapperTypeViaOwnership}
	models.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).Controller(&ctrl)

	mapper := SharedOwnershipMapper()

	var tx2 *gorm.DB
	retval := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retval *webrender.RetVal) {
		tx2 = tx
		if _, retval = mapper.DeleteOne(tx2, suite.who, suite.typeString, modelObj.GetID(),
			options, &cargo); retval != nil {
			return retval
		}
		return nil
	}, "lifecycle.DeleteOne")
	if !assert.Nil(suite.T(), retval) {
		return
	}

	role := models.UserRoleAdmin
	data := controller.Data{Ms: []models.IModel{&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID}, Name: carName}},
		DB: tx2, Who: suite.who, TypeString: suite.typeString, Roles: []models.UserRole{role}, Cargo: &cargo}
	info := controller.EndPointInfo{Op: controller.RESTOpDelete, Cardinality: controller.APICardinalityOne}

	assert.False(suite.T(), ctrl.guardAPIEntryCalled) // Not called when going through mapper (or lifecycle for that matter)

	if assert.True(suite.T(), ctrl.beforeCalled) {
		assert.Equal(suite.T(), info, *ctrl.beforeInfo)
		assert.Condition(suite.T(), dataComparisonNoDB(&data, ctrl.beforeData))
	}

	if assert.True(suite.T(), ctrl.afterCalled) {
		assert.Condition(suite.T(), dataComparisonNoDB(&data, ctrl.afterData))
		assert.Equal(suite.T(), info.Cardinality, ctrl.afterInfo.Cardinality)
		assert.Equal(suite.T(), info, *ctrl.afterInfo)
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
	cargo := controller.Cargo{}

	opt := models.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: models.MapperTypeViaOwnership}
	models.For(suite.typeString).ModelWithOption(&Car{}, opt)

	var modelObjs2 []models.IModel
	retval := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retval *webrender.RetVal) {
		mapper := SharedOwnershipMapper()
		modelObjs2, retval = mapper.DeleteMany(tx, suite.who, suite.typeString, modelObjs, options, &cargo)
		return retval
	}, "lifecycle.DeleteMany")
	if !assert.Nil(suite.T(), retval) {
		return
	}

	if assert.Len(suite.T(), modelObjs2, 3) {
		assert.Equal(suite.T(), carID1.String(), modelObjs2[0].GetID().String())
		assert.Equal(suite.T(), carID2.String(), modelObjs2[1].GetID().String())
		assert.Equal(suite.T(), carID3.String(), modelObjs2[2].GetID().String())
		assert.Equal(suite.T(), carName1, modelObjs2[0].(*Car).Name)
		assert.Equal(suite.T(), carName2, modelObjs2[1].(*Car).Name)
		assert.Equal(suite.T(), carName3, modelObjs2[2].(*Car).Name)
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
	cargo := controller.Cargo{}

	opt := models.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: models.MapperTypeViaOwnership}
	models.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).BatchCRUPDHooks(before, after).
		BatchDeleteHooks(beforeDelete, afterDelete)

	var tx2 *gorm.DB
	retval := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retval *webrender.RetVal) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		_, retval = mapper.DeleteMany(tx2, suite.who, suite.typeString, modelObjs, options, &cargo)
		return retval
	}, "lifecycle.DeleteMany")
	if !assert.Nil(suite.T(), retval) {
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

func (suite *TestBaseMapperDeleteSuite) TestCreateMany_WhenHavingController_NotCallOldCallbacks() {
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
	cargo := controller.Cargo{}

	opt := models.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: models.MapperTypeViaOwnership}
	models.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).BatchCRUPDHooks(before, after).
		BatchDeleteHooks(beforeDelete, afterDelete).Controller(&CarControllerWithoutCallbacks{})

	var tx2 *gorm.DB
	retval := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retval *webrender.RetVal) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		_, retval = mapper.DeleteMany(tx2, suite.who, suite.typeString, modelObjs, options, &cargo)
		return retval
	}, "lifecycle.DeleteMany")
	if !assert.Nil(suite.T(), retval) {
		return
	}

	assert.False(suite.T(), beforeCalled)
	assert.False(suite.T(), beforeDeleteCalled)
	assert.False(suite.T(), afterCalled)
	assert.False(suite.T(), afterDeleteCalled)
}

func (suite *TestBaseMapperDeleteSuite) TestCreateMany_WhenHavingController_CallRelevantControllerCallbacks() {
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
	cargo := controller.Cargo{}

	ctrl := CarController{}
	opt := models.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: models.MapperTypeViaOwnership}
	models.For(suite.typeString).ModelWithOption(&CarWithCallbacks{}, opt).Controller(&ctrl)

	var tx2 *gorm.DB
	retval := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retval *webrender.RetVal) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		_, retval = mapper.DeleteMany(tx2, suite.who, suite.typeString, modelObjs, options, &cargo)
		return retval
	}, "lifecycle.DeleteMany")
	if !assert.Nil(suite.T(), retval) {
		return
	}

	// Expected
	roles := []models.UserRole{models.UserRoleAdmin, models.UserRoleAdmin, models.UserRoleAdmin}
	data := controller.Data{Ms: []models.IModel{
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID1}, Name: carName1},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID2}, Name: carName2},
		&CarWithCallbacks{BaseModel: models.BaseModel{ID: carID3}, Name: carName3},
	}, DB: tx2, Who: suite.who, TypeString: suite.typeString, Roles: roles, Cargo: &cargo}
	info := controller.EndPointInfo{Op: controller.RESTOpDelete, Cardinality: controller.APICardinalityMany}

	assert.False(suite.T(), ctrl.guardAPIEntryCalled) // not called when call createMany directly
	assert.False(suite.T(), ctrl.beforeApplyCalled)
	if assert.True(suite.T(), ctrl.beforeCalled) {
		assert.Condition(suite.T(), dataComparisonNoDB(&data, ctrl.beforeData))
		assert.Equal(suite.T(), info.Op, ctrl.beforeInfo.Op)
		assert.Equal(suite.T(), info.Cardinality, ctrl.beforeInfo.Cardinality)
	}
	if assert.True(suite.T(), ctrl.afterCalled) {
		assert.Condition(suite.T(), dataComparisonNoDB(&data, ctrl.afterData))
		assert.Equal(suite.T(), info.Op, ctrl.afterInfo.Op)
		assert.Equal(suite.T(), info.Cardinality, ctrl.afterInfo.Cardinality)
	}
}

func TestBaseMappingDeleteSuite(t *testing.T) {
	suite.Run(t, new(TestBaseMapperDeleteSuite))
}
