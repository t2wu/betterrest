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
	"github.com/t2wu/betterrest/model/mappertype"
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

func resetGlobals() {
	guardAPIEntryCalled = false
	guardAPIEntryWho = nil
	guardAPIEntryHTTP = mdlutil.HTTP{}
}

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
	log.Println("len(data.Ms):", len(data.Ms))
	log.Println("len(data.Roles):", len(data.Roles))
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

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: mappertype.DirectOwnership}
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
		retVal, retErr = mapper.Create(tx, modelObjs, &ep, &cargo)
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

func (suite *TestBaseMapperCreateSuite) TestCreateMany_WhenHavingController_CallRelevantControllerCallbacks() {
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

	opt := registry.RegOptions{BatchMethods: "CRUPD", IdvMethods: "RUPD", Mapper: mappertype.DirectOwnership}
	registry.For(suite.typeString).ModelWithOption(&Car{}, opt).Hook(&CarHandlerJBT{}, "CRUPD")

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
		retVal, retErr = mapper.Create(tx2, modelObjs, &ep, &cargo)
		return retErr
	}, "lifecycle.CreateOne")
	if !assert.Nil(suite.T(), retErr) {
		return
	}

	roles := []userrole.UserRole{userrole.UserRoleAdmin, userrole.UserRoleAdmin, userrole.UserRoleAdmin}
	data := hook.Data{Ms: []mdl.IModel{
		&Car{BaseModel: mdl.BaseModel{ID: carID1}, Name: carName1},
		&Car{BaseModel: mdl.BaseModel{ID: carID2}, Name: carName2},
		&Car{BaseModel: mdl.BaseModel{ID: carID3}, Name: carName3},
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
			if c, ok := expected.Ms[i].(*Car); ok {
				if c.ID.String() != actual.Ms[i].(*Car).ID.String() {
					log.Println(".............5.3")
					return false
				}
				if c.Name != actual.Ms[i].(*Car).Name {
					log.Println(".............5.4")
					return false
				}
			}
		}

		log.Println("expected.Roles:", expected.Roles)
		log.Println("actual.Roles:", actual.Roles)
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
