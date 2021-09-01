package query

import (
	"errors"
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/libs/datatypes"
)

func TestQueryFirst_ByID(t *testing.T) {
	tm := TestModel{}
	uuid := datatypes.NewUUIDFromStringNoErr(uuid2)
	if err := Q(db, C("ID =", uuid)).First(&tm).Error(); err != nil {
		assert.Fail(t, err.Error())
		return
	}

	assert.Equal(t, "second", tm.Name)
}

func TestQueryFirst_ByWrongID_ShouldNotBeFoundAndGiveError(t *testing.T) {
	tm := TestModel{}
	uuid := datatypes.NewUUIDFromStringNoErr("3587f5f3-efcb-4937-8783-b66a434104bd")
	if err := Q(db, C("ID =", uuid)).First(&tm).Error(); err != nil {
		assert.Error(t, err)
		return
	}
	assert.Fail(t, "should not be found")
}

func TestQueryFirst_ByOneIntField(t *testing.T) {
	tests := []struct {
		query string
		val   int
		want  string
	}{
		{"Age =", 3, uuid3},
		{"Age =", 1, uuid1},
	}

	for _, test := range tests {
		tm := TestModel{}
		if err := Q(db, C(test.query, test.val)).First(&tm).Error(); err != nil {
			assert.Fail(t, err.Error(), "record not found")
			return
		}
		assert.Equal(t, test.want, tm.ID.String())
	}
}

func TestQueryFirst_ByOneStringField(t *testing.T) {
	tests := []struct {
		query string
		val   string
		want  string
	}{
		{"Name =", "same", uuid5},
		{"Name =", "first", uuid1},
	}

	for _, test := range tests {
		tm := TestModel{}
		if err := Q(db, C(test.query, test.val)).First(&tm).Error(); err != nil {
			assert.Fail(t, err.Error(), "record not found")
			return
		}
		assert.Equal(t, test.want, tm.ID.String())
	}
}

func TestQueryFirst_ByBothStringAndIntField(t *testing.T) {
	tm := TestModel{}
	if err := Q(db, C("Name =", "second").And("Age =", 3)).First(&tm).Error(); err != nil {
		assert.Fail(t, err.Error(), "record not found")
	}
	assert.Equal(t, uuid2, tm.ID.String())
}

func TestQuery_ByDB_ThenQ_Works(t *testing.T) {
	tm := TestModel{}
	if err := DB(db).Q(C("Name =", "second").And("Age =", 3)).First(&tm).Error(); err != nil {
		assert.Fail(t, err.Error(), "record not found")
	}
	assert.Equal(t, uuid2, tm.ID.String())
}

func TestQueryFirst_ByWrongValue_NotFoundShouldGiveError(t *testing.T) {
	tm := TestModel{}

	if err := Q(db, C("Name =", "tim")).First(&tm).Error(); err != nil {
		assert.Equal(t, true, errors.Is(err, gorm.ErrRecordNotFound))
		return
	}

	assert.Fail(t, "should not be found")
}

func TestQueryFirst_ByNonExistingFieldName_ShouldGiveAnError(t *testing.T) {
	tm := TestModel{}

	if err := Q(db, C("deleteCmdForExample =", "same")).First(&tm).Error(); err != nil {
		assert.Error(t, err)
		return
	}

	assert.Fail(t, "should not be here")
}

func TestQueryFirst_ByNonExistingOperator_ShouldGiveAnError(t *testing.T) {
	tm := TestModel{}

	if err := Q(db, C("Name WrongOp", "same")).First(&tm).Error(); err != nil {
		assert.Error(t, err)
		return
	}

	assert.Fail(t, "should not be here")
}

func TestQueryFind_ShouldGiveMultiple(t *testing.T) {
	tms := make([]TestModel, 0)

	if err := Q(db, C("Name =", "same")).Find(&tms).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	if assert.Equal(t, 3, len(tms)) {
		assert.Equal(t, uuid5, tms[0].ID.String())
		assert.Equal(t, uuid4, tms[1].ID.String())
		assert.Equal(t, uuid3, tms[2].ID.String())
	}
}

func TestQueryFindOffset_ShouldBeCorrect(t *testing.T) {
	tms := make([]TestModel, 0)

	if err := Q(db, C("Name =", "same")).Offset(1).Find(&tms).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	if assert.Equal(t, 2, len(tms)) {
		assert.Equal(t, uuid4, tms[0].ID.String())
		assert.Equal(t, uuid3, tms[1].ID.String())
	}
}

func TestQueryFindLimit_ShouldBeCorrect(t *testing.T) {
	tms := make([]TestModel, 0)

	if err := Q(db, C("Name =", "same")).Limit(2).Find(&tms).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	if assert.Equal(t, 2, len(tms)) {
		assert.Equal(t, uuid5, tms[0].ID.String())
		assert.Equal(t, uuid4, tms[1].ID.String())
	}
}

func TestQueryFindOrderBy_ShouldBeCorrect(t *testing.T) {
	tms := make([]TestModel, 0)

	if err := Q(db, C("Name =", "same")).Order("CreatedAt ASC").Find(&tms).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	if assert.Equal(t, 3, len(tms)) {
		assert.Equal(t, uuid3, tms[0].ID.String())
		assert.Equal(t, uuid4, tms[1].ID.String())
		assert.Equal(t, uuid5, tms[2].ID.String())
	}
}

func TestQueryFindOrderBy_BogusFieldShouldHaveError(t *testing.T) {
	tms := make([]TestModel, 0)

	// Currently order works not by field.
	if err := Q(db, C("Name =", "same")).Order("Bogus ASC").Find(&tms).Error(); err != nil {
		assert.Error(t, err)
		return
	}

	assert.Fail(t, "should not be here")
}

func TestQueryFind_WhenNotFound_ShouldNotGiveAnError(t *testing.T) {
	tms := make([]TestModel, 0)

	err := Q(db, C("Name =", "Greg")).Find(&tms).Error()
	assert.Nil(t, err)

	assert.Equal(t, 0, len(tms))
}

func TestQueryFind_WithoutCriteria_ShouldGetAll(t *testing.T) {
	tms := make([]TestModel, 0)

	if err := Q(db).Find(&tms).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	if assert.Equal(t, 5, len(tms)) {
		assert.Equal(t, uuid5, tms[0].ID.String())
		assert.Equal(t, uuid4, tms[1].ID.String())
		assert.Equal(t, uuid3, tms[2].ID.String())
		assert.Equal(t, uuid2, tms[3].ID.String())
		assert.Equal(t, uuid1, tms[4].ID.String())
	}
}

func TestQueryFind_Limit_ShouldWork(t *testing.T) {
	tms := make([]TestModel, 0)

	if err := Q(db).Limit(3).Find(&tms).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	if assert.Equal(t, 3, len(tms)) {
		assert.Equal(t, uuid5, tms[0].ID.String())
		assert.Equal(t, uuid4, tms[1].ID.String())
		assert.Equal(t, uuid3, tms[2].ID.String())
	}
}

func TestQueryFind_LimitAndOffset_ShouldWork(t *testing.T) {
	tms := make([]TestModel, 0)

	if err := Q(db).Offset(2).Limit(2).Find(&tms).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	if assert.Equal(t, 2, len(tms)) {
		assert.Equal(t, uuid3, tms[0].ID.String())
		assert.Equal(t, uuid2, tms[1].ID.String())
	}
}

func TestQueryFind_Offset_ShouldWork(t *testing.T) {
	tms := make([]TestModel, 0)

	if err := Q(db).Offset(2).Find(&tms).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	if assert.Equal(t, 3, len(tms)) {
		assert.Equal(t, uuid3, tms[0].ID.String())
		assert.Equal(t, uuid2, tms[1].ID.String())
		assert.Equal(t, uuid1, tms[2].ID.String())
	}
}

func TestQueryFind_TwoLevelNested_Query(t *testing.T) { // FIXME not work yet
	tms := make([]TestModel, 0)

	err := Q(db, C("Dogs.DogToys.ToyName =", "DogToySameName")).Find(&tms).Error()
	if assert.Nil(t, err) && assert.Equal(t, 2, len(tms)) {
		assert.Equal(t, uuid5, tms[0].ID.String())
		assert.Equal(t, uuid3, tms[1].ID.String())
	}
}

func TestFind_NestedQueryWithInnerJoin_Works(t *testing.T) {
	tms := make([]TestModel, 0)

	err := Q(db).InnerJoin(&UnNested{}, &TestModel{}, C("UnNestedInner.Name =", "UnNestedInnerSameNameWith1&2")).Find(&tms).Error()
	if assert.Nil(t, err) {
		assert.Equal(t, uuid2, tms[0].ID.String())
		assert.Equal(t, uuid1, tms[1].ID.String())
	}
}

// -------------------

func TestQueryFirst_Nested_Query(t *testing.T) {
	tm := TestModel{}

	err := Q(db, C("Dogs.Name =", "Doggie1")).First(&tm).Error()
	if assert.Nil(t, err) {
		assert.Equal(t, uuid3, tm.ID.String())
	}
}

func TestFirst_InnerJoin_Works(t *testing.T) {
	tm := TestModel{}

	err := Q(db).InnerJoin(&UnNested{}, &TestModel{}, C("Name =", "unnested2")).First(&tm).Error()
	assert.Nil(t, err)
	if err == nil {
		assert.Equal(t, uuid2, tm.ID.String())
	}
}

func TestFirst_NestedQueryWithInnerJoinWithCriteriaOnMainTable_Works(t *testing.T) {
	tm := TestModel{}

	err := Q(db, C("Dogs.Name =", "Doggie0")).InnerJoin(&UnNested{}, &TestModel{}, C("UnNestedInner.Name =", "UnNestedInnerSameNameWith1&2")).First(&tm).Error()
	if assert.Nil(t, err) {
		assert.Equal(t, uuid1, tm.ID.String())
	}
}
