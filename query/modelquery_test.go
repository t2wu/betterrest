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

func TestQueryByWrongID_ShouldNotBeFound(t *testing.T) {
	tm := TestModel{}
	uuid := datatypes.NewUUIDFromStringNoErr("3587f5f3-efcb-4937-8783-b66a434104bd")
	if err := ByID(db, &tm, uuid); err == nil {
		assert.Fail(t, "should not be found")
	}
}

func TestQueryByStringField(t *testing.T) {
	tests := []struct {
		field string
		val   string
		want  string
	}{
		{"name", "same", uuid4},
		{"name", "first", uuid1},
	}

	for _, test := range tests {
		tm := TestModel{}
		if err := FirstByField(db, &tm, test.field, test.val); err != nil {
			assert.Fail(t, err.Error(), "record not found")
		}

		assert.Equal(t, test.want, tm.ID.String())
	}
}

func TestQueryByIntField(t *testing.T) {
	tests := []struct {
		field string
		val   int
		want  string
	}{
		{"age", 3, uuid3},
		{"age", 1, uuid1},
	}

	for _, test := range tests {
		tm := TestModel{}
		if err := FirstByField(db, &tm, test.field, test.val); err != nil {
			assert.Fail(t, err.Error(), "record not found")
		}

		assert.Equal(t, test.want, tm.ID.String())
	}
}

func TestQueryByBothStringAndIntField(t *testing.T) {
	tests := []struct {
		fqs  []*FieldQuery
		want string
	}{
		{[]*FieldQuery{
			{Name: "name", Value: "second"},
			{Name: "age", Value: "3"},
		}, uuid2},
	}

	for _, test := range tests {
		tm := TestModel{}
		if err := FirstByFields(db, &tm, test.fqs); err != nil {
			assert.Fail(t, err.Error(), "record not found")
		}

		assert.Equal(t, test.want, tm.ID.String())
	}
}

func TestQueryByBothStringAndIntField_should_not_exits(t *testing.T) {
	tests := []struct {
		fqs  []*FieldQuery
		want string
	}{
		{[]*FieldQuery{
			{Name: "name", Value: "first"},
			{Name: "age", Value: "3"},
		}, uuid2},
	}

	for _, test := range tests {
		tm := TestModel{}
		if err := FirstByFields(db, &tm, test.fqs); err == nil {
			assert.Fail(t, err.Error(), "record should not be found")
		}
	}
}
