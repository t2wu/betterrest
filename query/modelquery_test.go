package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/libs/datatypes"
)

func TestQueryFirst_ByID(t *testing.T) {
	tm := TestModel{}
	uuid := datatypes.NewUUIDFromStringNoErr(uuid2)
	if err := Q(db, C("ID =", uuid)).First(&tm).Error; err != nil {
		assert.Fail(t, err.Error())
	}

	assert.Equal(t, "second", tm.Name)
}

func TestQueryFirst_ByWrongID_ShouldNotBeFoundAndGiveError(t *testing.T) {
	tm := TestModel{}
	uuid := datatypes.NewUUIDFromStringNoErr("3587f5f3-efcb-4937-8783-b66a434104bd")
	// if err := ByID(db, &tm, uuid); err != nil {
	if err := Q(db, C("ID =", uuid)).First(&tm).Error; err != nil {
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
		if err := Q(db, C(test.query, test.val)).First(&tm).Error; err != nil {
			assert.Fail(t, err.Error(), "record not found")
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
		{"Name =", "same", uuid4},
		{"Name =", "first", uuid1},
	}

	for _, test := range tests {
		tm := TestModel{}
		if err := Q(db, C(test.query, test.val)).First(&tm).Error; err != nil {
			assert.Fail(t, err.Error(), "record not found")
		}
		assert.Equal(t, test.want, tm.ID.String())
	}
}

func TestQueryFirst_ByBothStringAndIntField(t *testing.T) {
	tm := TestModel{}
	if err := Q(db, C("Name =", "second").And("Age =", 3)).First(&tm).Error; err != nil {
		assert.Fail(t, err.Error(), "record not found")
	}
	assert.Equal(t, uuid2, tm.ID.String())
}

func TestQueryFirst_ByWrongValue_NotFoundShouldGiveError(t *testing.T) {
	tm := TestModel{}

	if err := Q(db, C("Name =", "tim")).First(&tm).Error; err != nil {
		assert.Error(t, err)
		return
	}

	assert.Fail(t, "should not be found")
}

// I can assume modelObj2 has a modelObjID field?
// Q(db, C("Name =", "tim").And("Inner.Name =", "Kyle")).
//  InnerJoin(modelObj2, modelObj, C("ModelObj2Field1 =", "Claire")). // On clause is automatically inferred
//  InnerJoin(modelObj3, modelObj, C("...")).
//  InnerJoin(modelObj2, modelObj4, C("...")).First(&modelObj)

// So in every PredicateBuilder such s "C", some could actually be inner field
// but originally I designed it so that it's not inner...
//

func TestQueryFirst_ByNonExistingFieldName_ShouldGiveAnError(t *testing.T) {
	tm := TestModel{}

	if err := Q(db, C("deleteCmdForExample =", "same")).First(&tm).Error; err != nil {
		assert.Error(t, err)
		return
	}

	assert.Fail(t, "should not be here")
}

func TestQueryFirst_ByNonExistingOperator_ShouldGiveAnError(t *testing.T) {
	tm := TestModel{}

	if err := Q(db, C("Name WrongOp", "same")).First(&tm).Error; err != nil {
		assert.Error(t, err)
		return
	}

	assert.Fail(t, "should not be here")
}

func TestQueryFind_ShouldGiveMultiple(t *testing.T) {
	tms := make([]TestModel, 0)

	if err := Q(db, C("Name =", "same")).Find(&tms).Error; err != nil {
		assert.Error(t, err)
		return
	}

	assert.Equal(t, uuid4, tms[0].ID.String())
	assert.Equal(t, uuid3, tms[1].ID.String())
}

func TestQueryFind_WhenNotFound_ShouldNotGiveAnError(t *testing.T) {
	tms := make([]TestModel, 0)

	err := Q(db, C("Name =", "Greg")).Find(&tms).Error
	assert.Nil(t, err)

	assert.Equal(t, 0, len(tms))
}

func TestQueryFirst_Nested_Query(t *testing.T) {
	tm := TestModel{}

	err := Q(db, C("Dogs.Name =", "Doggie1")).First(&tm).Error
	assert.Nil(t, err)
	if err == nil {
		assert.Equal(t, uuid3, tm.ID.UUID.String())
	}
}

func TestQueryFirst_InnerJoin_Works(t *testing.T) {
	tm := TestModel{}

	err := Q(db).InnerJoin(&UnNested{}, &TestModel{}, C("Name =", "unnested2")).First(&tm).Error
	assert.Nil(t, err)
	if err == nil {
		assert.Equal(t, uuid2, tm.ID.UUID.String())
	}
}
