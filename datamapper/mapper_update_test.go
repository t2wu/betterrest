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

type TestBaseMapperUpdateSuite struct {
	suite.Suite
	db         *gorm.DB
	mock       sqlmock.Sqlmock
	who        mdlutil.UserIDFetchable
	typeString string
}

func (suite *TestBaseMapperUpdateSuite) SetupTest() {
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

func (suite *TestBaseMapperUpdateSuite) TestUpdateMany_WhenGiven_GotCars() {
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
	carIDs := []*datatype.UUID{carID1, carID2, carID3}
	carNames := []string{carName1, carName2, carName3}

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
		carID, carName := carIDs[i], carNames[i]
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

	var retVal *MapperRet
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		mapper := SharedOwnershipMapper()
		ep := hook.EndPoint{
			Op:          rest.OpUpdate,
			Cardinality: rest.CardinalityMany,
			TypeString:  suite.typeString,
			URLParams:   options,
			Who:         suite.who,
		}
		retVal, retErr = mapper.Update(tx, modelObjs, &ep, &cargo)
		return retErr
	}, "lifecycle.UpdateMany")
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

func (suite *TestBaseMapperUpdateSuite) TestUpdateMany_WhenHavingController_CallRelevantControllerCallbacks() {
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
	carIDs := []*datatype.UUID{carID1, carID2, carID3}
	carNames := []string{carName1, carName2, carName3}

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
		carID, carName := carIDs[i], carNames[i]
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

	var tx2 *gorm.DB
	var retVal *MapperRet
	ep := hook.EndPoint{
		Op:          rest.OpUpdate,
		Cardinality: rest.CardinalityMany,
		TypeString:  suite.typeString,
		URLParams:   options,
		Who:         suite.who,
	}
	retErr := transact.TransactCustomError(suite.db, func(tx *gorm.DB) (retErr *webrender.RetError) {
		tx2 = tx
		mapper := SharedOwnershipMapper()
		retVal, retErr = mapper.Update(tx2, modelObjs, &ep, &cargo)
		return retErr
	}, "lifecycle.UpdateMany")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	// Expected
	roles := []userrole.UserRole{userrole.UserRoleAdmin, userrole.UserRoleAdmin, userrole.UserRoleAdmin}
	data := hook.Data{Ms: []mdl.IModel{
		&Car{BaseModel: mdl.BaseModel{ID: carID1}, Name: carName1},
		&Car{BaseModel: mdl.BaseModel{ID: carID2}, Name: carName2},
		&Car{BaseModel: mdl.BaseModel{ID: carID3}, Name: carName3},
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
		assert.Condition(suite.T(), dataComparison(&data, hdlr.beforeData))
		assert.Equal(suite.T(), ep, *hdlr.beforeInfo)
	}
	if assert.True(suite.T(), hdlr.afterCalled) {
		assert.Condition(suite.T(), dataComparison(&data, hdlr.afterData))
		assert.Equal(suite.T(), ep, *hdlr.afterInfo)
	}
}

func TestBaseMappingUpdateSuite(t *testing.T) {
	suite.Run(t, new(TestBaseMapperUpdateSuite))
}
