package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/models"
)

func TestJSONKeysToFieldName_OneLevelTag_found(t *testing.T) {
	var v models.IModel = &Person{}
	fieldName, err := JSONKeysToFieldName(v, "id")
	assert.Nil(t, err)
	assert.Equal(t, "ID", fieldName)

	fieldName, err = JSONKeysToFieldName(v, "surname")
	assert.Nil(t, err)
	assert.Equal(t, "LastName", fieldName)

	fieldName, err = JSONKeysToFieldName(v, "contacts")
	assert.Nil(t, err)
	assert.Equal(t, "Contacts", fieldName)

	fieldName, err = JSONKeysToFieldName(v, "age")
	assert.Nil(t, err)
	assert.Equal(t, "Age", fieldName)

	fieldName, err = JSONKeysToFieldName(v, "anotherInt")
	assert.Nil(t, err)
	assert.Equal(t, "AnotherInt", fieldName)
}

func TestJSONKeysToFieldName_FieldWithoutJSONTag_HasError(t *testing.T) {
	var v models.IModel = &Person{}
	_, err := JSONKeysToFieldName(v, "missingJson")
	assert.Error(t, err)
}

func TestJSONKeysToFieldName_WrongJSONTag_HasError(t *testing.T) {
	var v models.IModel = &Person{}
	_, err := JSONKeysToFieldName(v, "lastName")
	assert.Error(t, err)
}

func TestJSONKeysToFieldName_TwoLevelTag_found(t *testing.T) {
	var v models.IModel = &Person{}
	fieldName, err := JSONKeysToFieldName(v, "pet.name")
	assert.Nil(t, err)
	assert.Equal(t, "Pet.Name", fieldName)

	fieldName, err = JSONKeysToFieldName(v, "pet.name2")
	assert.Nil(t, err)
	assert.Equal(t, "Pet.Name2", fieldName)

	fieldName, err = JSONKeysToFieldName(v, "pet2.name")
	assert.Nil(t, err)
	assert.Equal(t, "Pet2.Name", fieldName)

	fieldName, err = JSONKeysToFieldName(v, "pet2.name2")
	assert.Nil(t, err)
	assert.Equal(t, "Pet2.Name2", fieldName)

	fieldName, err = JSONKeysToFieldName(v, "contacts.name")
	assert.Nil(t, err)
	assert.Equal(t, "Contacts.Name", fieldName)

	fieldName, err = JSONKeysToFieldName(v, "contacts.name2")
	assert.Nil(t, err)
	assert.Equal(t, "Contacts.Name2", fieldName)

	fieldName, err = JSONKeysToFieldName(v, "contacts2.name")
	assert.Nil(t, err)
	assert.Equal(t, "Contacts2.Name", fieldName)

	fieldName, err = JSONKeysToFieldName(v, "contacts2.name2")
	assert.Nil(t, err)
	assert.Equal(t, "Contacts2.Name2", fieldName)
}

func TestJSONKeysToFieldName_ThreeLevel_Found(t *testing.T) {
	fieldName, err := JSONKeysToFieldName(&Person{}, "pet.petToy.name")
	assert.Nil(t, err)
	assert.Equal(t, "Pet.PetToy.Name", fieldName)
}
