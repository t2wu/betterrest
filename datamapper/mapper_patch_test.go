package datamapper

import (
	"fmt"
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
	"github.com/t2wu/betterrest/model/mappertype"
	"github.com/t2wu/betterrest/registry"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

// ----------------------------------------------------------------------------------------------

type TestBaseMapperPatchSuite struct {
	suite.Suite
	db         *gorm.DB
	mock       sqlmock.Sqlmock
	who        mdlutil.UserIDFetchable
	typeString string
}

func (suite *TestBaseMapperPatchSuite) SetupTest() {
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

// func (suite *TestBaseMapperPatchSuite) Test_should_fail() {
// 	assert.Equal(suite.T(), 1, 2)
// }

// All methods that begin with "Test" are run as tests within a
// suite.
// func (suite *TestBaseMapperPatchSuite) TestPatchOne_WhenGiven_GotCar() {
// 	carID := datatype.NewUUID()
// 	carName := "DSM"
// 	carNameNew := "DSM New"
// 	var modelObj mdl.IModel = &Car{BaseModel: mdl.BaseModel{ID: carID}, Name: carName}

// 	// The first three SQL probably could be made into one
// 	suite.mock.ExpectBegin()
// 	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
// 	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
// 	stmt2 := `SELECT * FROM "user_owns_car" WHERE (user_id = $1 AND model_id = $2)`
// 	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), userrole.UserRoleAdmin))
// 	stmt3 := `UPDATE "car" SET "updated_at" = $1, "deleted_at" = $2, "name" = $3  WHERE "car"."id" = $4`
// 	result := sqlmock.NewResult(0, 1)
// 	// WithArgs (how do I test what gorm insert when date can be arbitrary? Use hooks?)
// 	suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
// 	// suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WithArgs(carID, carNameNew).WillReturnResult(result)
// 	// These two queries can be made into one as well (or with returning, all 5 can be in 1?)
// 	stmt4 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2`
// 	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt4)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carNameNew))
// 	stmt5 := `SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`
// 	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt5)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), userrole.UserRoleAdmin))
// 	suite.mock.ExpectCommit()

// 	options := make(map[urlparam.Param]interface{})
// 	cargo := hook.Cargo{}

// 	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: mappertype.DirectOwnership}
// 	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

// 	mapper := SharedOwnershipMapper()

// 	var jsonPatch = []byte(fmt.Sprintf(`[{
// 		"op": "replace", "path": "/name", "value": "%s"
// 	}]`, carNameNew))

// 	var retVal *MapperRet
// 	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
// 		ep := hook.EndPoint{
// 			Op:          rest.OpPatch,
// 			Cardinality: rest.CardinalityOne,
// 			TypeString:  suite.typeString,
// 			URLParams:   options,
// 			Who:         suite.who,
// 		}
// 		if retVal, retErr = mapper.PatchOne(tx, jsonPatch, modelObj.GetID(), &ep, &cargo); retErr != nil {
// 			return retErr
// 		}
// 		return nil
// 	}, "lifecycle.PatchOne")
// 	if !assert.Nil(suite.T(), retErr) {
// 		return
// 	}

// 	if car, ok := retVal.Ms[0].(*Car); assert.True(suite.T(), ok) {
// 		assert.Equal(suite.T(), carNameNew, car.Name)
// 		assert.Equal(suite.T(), carID, car.ID)
// 	}
// }

// func (suite *TestBaseMapperPatchSuite) TestPatchOne_WhenHavingController_CallRelevantControllerCallbacks() {
// 	carID := datatype.NewUUID()
// 	carName := "DSM"
// 	carNameNew := "DSM New"
// 	var modelObj mdl.IModel = &Car{BaseModel: mdl.BaseModel{ID: carID}, Name: carName}

// 	// The first three SQL probably could be made into one
// 	suite.mock.ExpectBegin()
// 	stmt := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2 WHERE "car"."deleted_at" IS NULL`
// 	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carName))
// 	stmt2 := `SELECT * FROM "user_owns_car" WHERE (user_id = $1 AND model_id = $2)`
// 	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), userrole.UserRoleAdmin))
// 	stmt3 := `UPDATE "car" SET "updated_at" = $1, "deleted_at" = $2, "name" = $3  WHERE "car"."id" = $4`
// 	result := sqlmock.NewResult(0, 1)
// 	// WithArgs (how do I test what gorm insert when date can be arbitrary? Use hooks?)
// 	suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WillReturnResult(result)
// 	// suite.mock.ExpectExec(regexp.QuoteMeta(stmt3)).WithArgs(carID, carNameNew).WillReturnResult(result)
// 	// These two queries can be made into one as well (or with returning, all 5 can be in 1?)
// 	stmt4 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id = $1 INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $2`
// 	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt4)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID, carNameNew))
// 	stmt5 := `SELECT * FROM "user_owns_car"  WHERE (user_id = $1 AND model_id = $2)`
// 	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt5)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), userrole.UserRoleAdmin))
// 	suite.mock.ExpectCommit()

// 	options := make(map[urlparam.Param]interface{})
// 	cargo := hook.Cargo{}

// 	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: mappertype.DirectOwnership}
// 	registry.For(suite.typeString).ModelWithOption(&Car{}, opt).Hook(&CarHandlerJBT{}, "CRUPD")

// 	jsonPatch := []byte(fmt.Sprintf(`[{
// 		"op": "replace", "path": "/name", "value": "%s"
// 	}]`, carNameNew))

// 	var tx2 *gorm.DB
// 	var retVal *MapperRet
// 	ep := hook.EndPoint{
// 		Op:          rest.OpPatch,
// 		Cardinality: rest.CardinalityOne,
// 		TypeString:  suite.typeString,
// 		URLParams:   options,
// 		Who:         suite.who,
// 	}
// 	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
// 		tx2 = tx
// 		mapper := SharedOwnershipMapper()
// 		if retVal, retErr = mapper.PatchOne(tx2, jsonPatch, modelObj.GetID(), &ep, &cargo); retErr != nil {
// 			return retErr
// 		}
// 		return nil
// 	}, "lifecycle.PatchOne")
// 	if !assert.Nil(suite.T(), retErr) {
// 		return
// 	}

// 	role := userrole.UserRoleAdmin
// 	dataBeforeApply := hook.Data{Ms: []mdl.IModel{&Car{BaseModel: mdl.BaseModel{ID: carID},
// 		Name: carName}}, DB: tx2, Roles: []userrole.UserRole{role}, Cargo: &cargo}

// 	data := hook.Data{Ms: []mdl.IModel{&Car{BaseModel: mdl.BaseModel{ID: carID},
// 		Name: carNameNew}}, DB: tx2, Roles: []userrole.UserRole{role}, Cargo: &cargo}

// 	ctrls := retVal.Fetcher.GetAllInstantiatedHanders()
// 	if !assert.Len(suite.T(), ctrls, 1) {
// 		return
// 	}

// 	hdlr, ok := ctrls[0].(*CarHandlerJBT)
// 	if !assert.True(suite.T(), ok) {
// 		return
// 	}

// 	assert.False(suite.T(), hdlr.guardAPIEntryCalled) // Not called when going through mapper (or lifecycle for that matter)

// 	if assert.True(suite.T(), hdlr.beforeApplyCalled) {
// 		assert.Equal(suite.T(), ep, *hdlr.beforeApplyInfo)
// 		assert.Condition(suite.T(), dataComparison(&dataBeforeApply, hdlr.beforeApplyData))
// 	}

// 	if assert.True(suite.T(), hdlr.beforeCalled) {
// 		assert.Equal(suite.T(), ep, *hdlr.beforeInfo)
// 		assert.Condition(suite.T(), dataComparison(&data, hdlr.beforeData))
// 		// assert.Equal(suite.T(), ep.Op, hdlr.beforeInfo.Op)
// 		// assert.Equal(suite.T(), ep, *hdlr.beforeInfo)
// 	}

// 	if assert.True(suite.T(), hdlr.afterCalled) {
// 		assert.Condition(suite.T(), dataComparison(&data, hdlr.afterData))
// 		assert.Equal(suite.T(), ep, *hdlr.afterInfo)
// 	}
// }

func (suite *TestBaseMapperPatchSuite) TestPatchMany_WhenGiven_GotCars() {
	carID1 := datatype.NewUUID()
	carName1 := "DSM"
	carNameNew1 := "DSM New"
	carID2 := datatype.NewUUID()
	carName2 := "DSM4Life"
	carNameNew2 := "DSM4Life New"
	carID3 := datatype.NewUUID()
	carName3 := "Eclipse"
	carNameNew3 := "Eclipse New"
	carIDs := []*datatype.UUID{carID1, carID2, carID3}
	carNamesNew := []string{carNameNew1, carNameNew2, carNameNew3}

	// The first three SQL probably could be made into one (well no I can't, I need to pull the old one so
	// I can call the callback...or when I can test for whether the call back exists before I optimize.)
	suite.mock.ExpectBegin()
	stmt1 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id IN ($1,$2,$3) INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $4 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID1, carName1).AddRow(carID2, carName2).AddRow(carID3, carName3))
	stmt2 := `SELECT "user_owns_car"."role" FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id IN ($1,$2,$3) INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $4`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(userrole.UserRoleAdmin).AddRow(userrole.UserRoleAdmin).AddRow(userrole.UserRoleAdmin))

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
		suite.mock.ExpectQuery(regexp.QuoteMeta(stmt5)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), userrole.UserRoleAdmin))
	}
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: mappertype.DirectOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt)

	jsonPatches := []mdlutil.JSONIDPatch{
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
		ep := hook.EndPoint{
			Op:          rest.OpPatch,
			Cardinality: rest.CardinalityMany,
			TypeString:  suite.typeString,
			URLParams:   options,
			Who:         suite.who,
		}
		retVal, retErr = mapper.Patch(tx, jsonPatches, &ep, &cargo)
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

func (suite *TestBaseMapperPatchSuite) TestCreateMany_WhenHavingController_CallRelevantControllerCallbacks() {
	carID1 := datatype.NewUUID()
	carName1 := "DSM"
	carNameNew1 := "DSM New"
	carID2 := datatype.NewUUID()
	carName2 := "DSM4Life"
	carNameNew2 := "DSM4Life New"
	carID3 := datatype.NewUUID()
	carName3 := "Eclipse"
	carNameNew3 := "Eclipse New"
	carIDs := []*datatype.UUID{carID1, carID2, carID3}
	carNamesNew := []string{carNameNew1, carNameNew2, carNameNew3}

	// The first three SQL probably could be made into one (well no I can't, I need to pull the old one so
	// I can call the callback...or when I can test for whether the call back exists before I optimize.)
	suite.mock.ExpectBegin()
	stmt1 := `SELECT "car".* FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id IN ($1,$2,$3) INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $4 WHERE "car"."deleted_at" IS NULL`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt1)).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(carID1, carName1).AddRow(carID2, carName2).AddRow(carID3, carName3))
	stmt2 := `SELECT "user_owns_car"."role" FROM "car" INNER JOIN "user_owns_car" ON "car".id = "user_owns_car".model_id AND "car".id IN ($1,$2,$3) INNER JOIN "user" ON "user".id = "user_owns_car".user_id AND "user_owns_car".user_id = $4`
	suite.mock.ExpectQuery(regexp.QuoteMeta(stmt2)).WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(userrole.UserRoleAdmin).AddRow(userrole.UserRoleAdmin).AddRow(userrole.UserRoleAdmin))

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
		suite.mock.ExpectQuery(regexp.QuoteMeta(stmt5)).WillReturnRows(sqlmock.NewRows([]string{"model_id", "user_id", "role"}).AddRow(carID, suite.who.GetUserID(), userrole.UserRoleAdmin))
	}
	suite.mock.ExpectCommit()

	options := make(map[urlparam.Param]interface{})
	cargo := hook.Cargo{}

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: mappertype.DirectOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt).Hook(&CarHandlerJBT{}, "CRUPD")

	jsonPatches := []mdlutil.JSONIDPatch{
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
		ep := hook.EndPoint{
			Op:          rest.OpPatch,
			Cardinality: rest.CardinalityMany,
			TypeString:  suite.typeString,
			URLParams:   options,
			Who:         suite.who,
		}
		retVal, retErr = mapper.Patch(tx2, jsonPatches, &ep, &cargo)
		return retErr
	}, "lifecycle.PatchMany")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	// Expected
	roles := []userrole.UserRole{userrole.UserRoleAdmin, userrole.UserRoleAdmin, userrole.UserRoleAdmin}
	dataBeforePatch := hook.Data{Ms: []mdl.IModel{
		&Car{BaseModel: mdl.BaseModel{ID: carID1}, Name: carName1},
		&Car{BaseModel: mdl.BaseModel{ID: carID2}, Name: carName2},
		&Car{BaseModel: mdl.BaseModel{ID: carID3}, Name: carName3},
	}, DB: tx2, Roles: roles, Cargo: &cargo}
	data := hook.Data{Ms: []mdl.IModel{
		&Car{BaseModel: mdl.BaseModel{ID: carID1}, Name: carNameNew1},
		&Car{BaseModel: mdl.BaseModel{ID: carID2}, Name: carNameNew2},
		&Car{BaseModel: mdl.BaseModel{ID: carID3}, Name: carNameNew3},
	}, DB: tx2, Roles: roles, Cargo: &cargo}
	ep := hook.EndPoint{
		Op:          rest.OpPatch,
		Cardinality: rest.CardinalityMany,
		TypeString:  suite.typeString,
		URLParams:   options,
		Who:         suite.who,
	}

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
		assert.Equal(suite.T(), ep, *hdlr.beforeInfo)
	}

	if assert.True(suite.T(), hdlr.beforeCalled) {
		assert.Condition(suite.T(), dataComparison(&data, hdlr.beforeData))
		assert.Equal(suite.T(), ep, *hdlr.beforeInfo)
	}

	if assert.True(suite.T(), hdlr.afterCalled) {
		assert.Condition(suite.T(), dataComparison(&data, hdlr.afterData))
		assert.Equal(suite.T(), ep, *hdlr.beforeInfo)
	}
}

func TestBaseMappingPatchSuite(t *testing.T) {
	suite.Run(t, new(TestBaseMapperPatchSuite))
}
