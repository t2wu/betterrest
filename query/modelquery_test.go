package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/libs/datatypes"
)

func TestQueryFirst_ByID(t *testing.T) {
	tm := TestModel{}
	uuid := datatypes.NewUUIDFromStringNoErr(uuid2)
	if err := Q(db).By("ID =", uuid).First(&tm).Error; err != nil {
		assert.Fail(t, err.Error())
	}

	assert.Equal(t, tm.Name, "second")
}

func TestQueryFirst_ByWrongID_ShouldNotBeFoundAndGiveError(t *testing.T) {
	tm := TestModel{}
	uuid := datatypes.NewUUIDFromStringNoErr("3587f5f3-efcb-4937-8783-b66a434104bd")
	// if err := ByID(db, &tm, uuid); err != nil {
	if err := Q(db).By("ID =", uuid).First(&tm).Error; err != nil {
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
		if err := Q(db).By(test.query, test.val).First(&tm).Error; err != nil {
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
		if err := Q(db).By(test.query, test.val).First(&tm).Error; err != nil {
			assert.Fail(t, err.Error(), "record not found")
		}
		assert.Equal(t, test.want, tm.ID.String())
	}
}

func TestQueryFirst_ByBothStringAndIntField(t *testing.T) {
	tm := TestModel{}
	if err := Q(db).By("Name =", "second", "Age = ", 3).First(&tm).Error; err != nil {
		assert.Fail(t, err.Error(), "record not found")
	}
	assert.Equal(t, uuid2, tm.ID.String())
}

func TestQueryFirst_ByWrongValue_NotFoundShouldGiveError(t *testing.T) {
	tm := TestModel{}

	if err := Q(db).By("Name =", "tim").First(&tm).Error; err != nil {
		assert.Error(t, err)
		return
	}

	assert.Fail(t, "should not be found")
}

// // Where would I actually check that the field is part of the model?
// // If I put it in there it seems too much work
// // But then again interface may look nicer
// // Maybe use dependency injection to do the translation?
// // But this interface could be used in places where there is NO need to check.
// // So it doesn't seem right to be used here.
// // Or maybe Init().FieldByFieldQueries()
// // Init().Unchecked().FieldByFieldQueries() for unchecked
// // and default checked.
// // FirstByFieldQueries(db, &tm, "name =", "tim", "inner.name =", "Kyle")

// // Automatically walk the key by type so it means tm has a foreign key to unnestedtable
// // I can walk back by simply querying for struct field in unestedtable{}
// // FirstByFieldQueries(db, &tm, "name =", "tim", "inner.name =", "Kyle").
// // InnerJoin(&UnnestedTable{})

// // Q("name =", "tim", "inner.name =", "Kyle").First(&tm)
// // Q("name =", "tim", "inner.name =", "Kyle").Find(&tm)
// // Q("name =", "tim", "inner.name =", "Kyle").
// // InnerJoins(Model(&userownssite).Q("name =", "tim", "inner.name =", "Kyle")).
// // InnerJoins(Modle(&User).Q("id =", "xxx").Find(&tm)
// // (This means innerJoins has to traverse both ways), using xxxID as a model
// // or User struct as a clue or some other tag to be really smart

func TestQueryFirst_ByNonExistingFieldName_ShouldGiveAnError(t *testing.T) {
	tm := TestModel{}

	// only name and age should be in there, to make sureinject some other SQL, for example
	if err := Q(db).By("deleteCmdForExample =", "same").First(&tm).Error; err != nil {
		assert.Error(t, err)
		return
	}

	assert.Fail(t, "should not be here")
}

func TestQueryFirst_ByNonExistingOperator_ShouldGiveAnError(t *testing.T) {
	tm := TestModel{}

	// only name and age should be in there, to make sureinject some other SQL, for example
	if err := Q(db).By("Name WrongOp", "same").First(&tm).Error; err != nil {
		assert.Error(t, err)
		return
	}

	assert.Fail(t, "should not be here")
}

func TestQueryFind_ShouldGiveMultiple(t *testing.T) {
	tms := make([]TestModel, 0)

	// only name and age should be in there, to make sureinject some other SQL, for example
	if err := Q(db).By("Name =", "same").Find(&tms).Error; err != nil {
		assert.Error(t, err)
		return
	}

	assert.Equal(t, uuid4, tms[0].ID.String())
	assert.Equal(t, uuid3, tms[1].ID.String())
}

func TestQueryFind_WhenNotFound_ShouldNotGiveAnError(t *testing.T) {
	tms := make([]TestModel, 0)

	// only name and age should be in there, to make sureinject some other SQL, for example
	err := Q(db).By("Name =", "Greg").Find(&tms).Error
	assert.Nil(t, err)

	assert.Equal(t, 0, len(tms))
}

// func TestQueryFind_Nested_Query(t *testing.T) {
// 	tm := TestModel{}

// 	// only name and age should be in there, to make sureinject some other SQL, for example
// 	err := Q(db).By("Dogs.Name =", "Doggie1").First(&tm).Error
// 	assert.Nil(t, err)
// 	if err == nil {
// 		assert.Equal(t, uuid3, tm.ID.UUID)
// 	}
// }
