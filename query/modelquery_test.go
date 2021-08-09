package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/libs/datatypes"
)

func TestQueryByID(t *testing.T) {
	tm := TestModel{}
	uuid := datatypes.NewUUIDFromStringNoErr(uuid2)
	if err := ByID(db, &tm, uuid); err != nil {
		assert.Fail(t, err.Error())
	}

	assert.Equal(t, tm.Name, "second")
}

func TestQueryByWrongID_ShouldNotBeFoundAndGiveError(t *testing.T) {
	tm := TestModel{}
	uuid := datatypes.NewUUIDFromStringNoErr("3587f5f3-efcb-4937-8783-b66a434104bd")
	if err := ByID(db, &tm, uuid); err != nil {
		assert.Error(t, err)
		return
	}
	assert.Fail(t, "should not be found")
}

func TestQueryByOneIntField(t *testing.T) {
	tests := []struct {
		field string
		val   int
		want  string
	}{
		{"age =", 3, uuid3},
		{"age =", 1, uuid1},
	}

	for _, test := range tests {
		tm := TestModel{}
		if err := FirstByFieldQueries(db, &tm, test.field, test.val); err != nil {
			assert.Fail(t, err.Error(), "record not found")
		}
		assert.Equal(t, test.want, tm.ID.String())
	}
}

func TestQueryByOneStringField(t *testing.T) {
	tests := []struct {
		field string
		val   string
		want  string
	}{
		{"name =", "same", uuid4},
		{"name =", "first", uuid1},
	}

	for _, test := range tests {
		tm := TestModel{}
		if err := FirstByFieldQueries(db, &tm, test.field, test.val); err != nil {
			assert.Fail(t, err.Error(), "record not found")
		}
		assert.Equal(t, test.want, tm.ID.String())
	}
}

func TestQueryByBothStringAndIntField(t *testing.T) {
	tm := TestModel{}
	if err := FirstByFieldQueries(db, &tm, "name =", "second", "age = ", 3); err != nil {
		assert.Fail(t, err.Error(), "record not found")
	}
	assert.Equal(t, uuid2, tm.ID.String())
}

func TestQueryByWrongValue_NotFoundShouldGiveError(t *testing.T) {
	tm := TestModel{}
	if err := FirstByFieldQueries(db, &tm, "name =", "tim"); err != nil {
		assert.Error(t, err)
		return
	}

	assert.Fail(t, "should not be found")
}

// func TestQueryByBothStringAndIntField_should_not_exits(t *testing.T) {
// 	tests := []struct {
// 		fqs  []*FieldQuery
// 		want string
// 	}{
// 		{[]*FieldQuery{
// 			{Name: "name", Value: "first"},
// 			{Name: "age", Value: "3"},
// 		}, uuid2},
// 	}

// 	for _, test := range tests {
// 		tm := TestModel{}
// 		if err := FirstByFields(db, &tm, test.fqs); err == nil {
// 			assert.Fail(t, err.Error(), "record should not be found")
// 		}
// 	}
// }
