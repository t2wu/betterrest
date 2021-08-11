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

// Where would I actually check that the field is part of the model?
// If I put it in there it seems too much work
// But then again interface may look nicer
// Maybe use dependency injection to do the translation?
// But this interface could be used in places where there is NO need to check.
// So it doesn't seem right to be used here.
// Or maybe Init().FieldByFieldQueries()
// Init().Unchecked().FieldByFieldQueries() for unchecked
// and default checked.
// FirstByFieldQueries(db, &tm, "name =", "tim", "inner.name =", "Kyle")

// Automatically walk the key by type so it means tm has a foreign key to unnestedtable
// I can walk back by simply querying for struct field in unestedtable{}
// FirstByFieldQueries(db, &tm, "name =", "tim", "inner.name =", "Kyle").
// InnerJoin(&UnnestedTable{})

// Q("name =", "tim", "inner.name =", "Kyle").First(&tm)
// Q("name =", "tim", "inner.name =", "Kyle").Find(&tm)
// Q("name =", "tim", "inner.name =", "Kyle").
// InnerJoins(Model(&userownssite).Q("name =", "tim", "inner.name =", "Kyle")).
// InnerJoins(Modle(&User).Q("id =", "xxx").Find(&tm)
// (This means innerJoins has to traverse both ways), using xxxID as a model
// or User struct as a clue or some other tag to be really smart

func TestQueryByNonExistingFieldName_ShouldGiveAnError(t *testing.T) {
	tm := TestModel{}

	// only name and age should be in there, to make sureinject some other SQL, for example
	if err := FirstByFieldQueries(db, &tm, "deleteCmdForExample =", "tim"); err != nil {
		assert.Equal(t, "field not found", err.Error())
		return
	}

	assert.Fail(t, "should not be found")
}
