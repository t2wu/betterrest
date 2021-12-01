package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFieldNameToColumn_works(t *testing.T) {
	p := Person{}
	var v IModel = &p
	columnName, err := FieldNameToColumn(v, "FirstName")
	assert.Nil(t, err)
	assert.Equal(t, "first_name", columnName)
}

func TestFieldNameToCustomColumn_works(t *testing.T) {
	p := Person{}
	var v IModel = &p
	columnName, err := FieldNameToColumn(v, "CustomColumn")
	assert.Nil(t, err)
	assert.Equal(t, "My_columnname", columnName)
}

func TestFieldNameToColumn_NestedThreeLevel_Works(t *testing.T) {
	p := Person{}
	var v IModel = &p
	columnName, err := FieldNameToColumn(v, "Contacts.Name")
	assert.Nil(t, err)
	assert.Equal(t, "contacts.name", columnName)

	columnName, err = FieldNameToColumn(v, "Contacts.Name2")
	assert.Nil(t, err)
	assert.Equal(t, "contacts.name2", columnName)

	columnName, err = FieldNameToColumn(v, "Contacts2.Name")
	assert.Nil(t, err)
	assert.Equal(t, "contacts2.name", columnName)

	columnName, err = FieldNameToColumn(v, "Contacts2.Name2")
	assert.Nil(t, err)
	assert.Equal(t, "contacts2.name2", columnName)

	columnName, err = FieldNameToColumn(v, "Pet.Name")
	assert.Nil(t, err)
	assert.Equal(t, "pet.name", columnName)

	columnName, err = FieldNameToColumn(v, "Pet.Name2")
	assert.Nil(t, err)
	assert.Equal(t, "pet.name2", columnName)

	columnName, err = FieldNameToColumn(v, "Pet2.Name")
	assert.Nil(t, err)
	assert.Equal(t, "pet2.name", columnName)

	columnName, err = FieldNameToColumn(v, "Pet2.Name2")
	assert.Nil(t, err)
	assert.Equal(t, "pet2.name2", columnName)
}

func TestFieldNameToColumn_NestedThreeLevelAndCustom_Works(t *testing.T) {
	var v IModel = &Person{}
	columnName, err := FieldNameToColumn(v, "Pet.PetToy.Name")
	assert.Nil(t, err)
	assert.Equal(t, "pet.pet_toy.pet_toy_name", columnName)

	columnName, err = FieldNameToColumn(v, "Pet.PetToy2.Name")
	assert.Nil(t, err)
	assert.Equal(t, "pet.pet_toy2.pet_toy_name", columnName)
}

func TestFieldNameToColumnWrongName_Error(t *testing.T) {
	p := Person{}
	var v IModel = &p
	_, err := FieldNameToColumn(v, "NotHere")
	assert.Error(t, err)
}
