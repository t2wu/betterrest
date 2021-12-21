package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSave_Works(t *testing.T) {
	id2 := "d113ed09-cfc5-47a5-b35c-6f60c49cbd08"

	tx := db.Begin()
	defer tx.Rollback()

	tm := TestModel{}
	if err := Q(tx, C("ID =", id2)).First(&tm).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	// Change the name to something else
	tm.Name = "TestSave_Works"
	if err := Q(tx).Save(&tm).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	// Find it back to make sure it has been changed
	searched := TestModel{}
	if err := Q(tx, C("ID =", id2)).First(&searched).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	assert.Equal(t, "TestSave_Works", searched.Name)
}

func TestUpdate_Field_Works(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()
	if err := Q(tx, C("Name =", "second")).Update(&TestModel{}, C("Age =", 120)).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	check := TestModel{}
	if err := Q(tx, C("Name =", "second")).First(&check).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	assert.Equal(t, 120, check.Age)
}

func TestUpdate_NestedField_ShouldGiveWarning(t *testing.T) {
	// Name:        "Doggie2",
	if err := Q(db, C("Name =", "second")).Update(&TestModel{}, C("Dogs.Color =", "purple")).Error(); err != nil {
		assert.Equal(t, "dot notation in update", err.Error())
		return
	}

	assert.Fail(t, "should not be here")
}

// Batch create and delete operations
