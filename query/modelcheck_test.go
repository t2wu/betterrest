package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/models"
)

type Person struct {
	models.BaseModel

	FirstName  string  `json:"firstName"`
	LastName   *string `json:"surname"` // note, json is different from field name
	Age        int     `json:"age"`
	AnotherInt *int    `json:"anotherInt"`

	Contacts  []Contact  `json:"contacts"`
	Contacts2 []*Contact `json:"contacts2"`

	Pet  Pet  `json:"pet"`
	Pet2 *Pet `json:"pet2"`
}

type Contact struct {
	models.BaseModel

	Name      string
	Name2     *string
	Age       int
	Age2      *int
	Email     []string
	Addresses []*string
	Phone     string
	Work      *string
}

type Pet struct {
	models.BaseModel
	Name  string
	Name2 *string
	Age   int
	Age2  *int

	PetToy  PetToy
	PetToy2 *PetToy
}

type PetToy struct {
	models.BaseModel
	Name string
}

func TestIsFieldInModel_ExistingField_IsTrue(t *testing.T) {
	m := Person{}
	b := IsFieldInModel(&m, "FirstName")
	assert.Equal(t, true, b)

	b = IsFieldInModel(&m, "LastName")
	assert.Equal(t, true, b)

	b = IsFieldInModel(&m, "Age")
	assert.Equal(t, true, b)

	b = IsFieldInModel(&m, "AnotherInt")
	assert.Equal(t, true, b)

	b = IsFieldInModel(&m, "Contacts")
	assert.Equal(t, true, b)

	b = IsFieldInModel(&m, "Contacts2")
	assert.Equal(t, true, b)

	b = IsFieldInModel(&m, "Pet")
	assert.Equal(t, true, b)

	b = IsFieldInModel(&m, "Pet2")
	assert.Equal(t, true, b)

	b = IsFieldInModel(&m, "ID") // Wow, fieldbyname actually works on embedded
	assert.Equal(t, true, b)
}

func TestIsFieldInModel_NonExistingField_IsFalse(t *testing.T) {
	m := Person{}
	b := IsFieldInModel(&m, "firstName") // lower case f doesn't exist
	assert.Equal(t, false, b)

	b = IsFieldInModel(&m, "NonExisting")
	assert.Equal(t, false, b)
}

func TestIsFieldInModel_NestedModel_IsTrue(t *testing.T) {
	m := Person{}
	b := IsFieldInModel(&m, "Pet.Name")
	assert.Equal(t, true, b)
	b = IsFieldInModel(&m, "Pet.Name2")
	assert.Equal(t, true, b)
	b = IsFieldInModel(&m, "Pet.Age")
	assert.Equal(t, true, b)
	b = IsFieldInModel(&m, "Pet.Age2")
	assert.Equal(t, true, b)
}

func TestIsFieldInModel_NonExistingNestedModel_IsFalse(t *testing.T) {
	m := Person{}
	b := IsFieldInModel(&m, "NotExisting.Name")
	assert.Equal(t, false, b)
	b = IsFieldInModel(&m, "NotExisting.Name2")
	assert.Equal(t, false, b)
	b = IsFieldInModel(&m, "NotExisting.Age")
	assert.Equal(t, false, b)
	b = IsFieldInModel(&m, "NotExisting.Age2")
	assert.Equal(t, false, b)
}

func TestIsFieldInModel_NestedSliceField_IsTrue(t *testing.T) {
	m := Person{}
	b := IsFieldInModel(&m, "Contacts.Name")
	assert.Equal(t, true, b)
	b = IsFieldInModel(&m, "Contacts.Name2")
	assert.Equal(t, true, b)
	b = IsFieldInModel(&m, "Contacts.Age")
	assert.Equal(t, true, b)
	b = IsFieldInModel(&m, "Contacts.Age2")
	assert.Equal(t, true, b)
}

func TestIsFieldInModel_TwoLevelNestedModel_IsTrue(t *testing.T) {
	m := Person{}
	b := IsFieldInModel(&m, "Pet.PetToy.Name")
	assert.Equal(t, true, b)
}

// --------------------------- GetModelFieldTypeInModelIfValid ---------------

func TestIsFieldInModel_GetFieldType_Correct(t *testing.T) {
	m := Person{}
	typ, err := GetModelFieldTypeInModelIfValid(&m, "FirstName")
	assert.Nil(t, err)
	assert.Equal(t, "string", typ.String())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "LastName")
	assert.Nil(t, err)
	assert.Equal(t, "string", typ.String())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Age")
	assert.Nil(t, err)
	assert.Equal(t, "int", typ.String())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "AnotherInt")
	assert.Nil(t, err)
	assert.Equal(t, "int", typ.String())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Contacts")
	assert.Nil(t, err)
	assert.Equal(t, "Contact", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Contacts2")
	assert.Nil(t, err)
	assert.Equal(t, "Contact", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Pet")
	assert.Nil(t, err)
	assert.Equal(t, "Pet", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Pet2")
	assert.Nil(t, err)
	assert.Equal(t, "Pet", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "ID") // Wow, fieldbyname actually works on embedded
	assert.Nil(t, err)
	assert.Equal(t, "UUID", typ.Name())
}

func TestGetModelFieldTypeInModelIfValid_GetFieldTypeNotExists_ThrowError(t *testing.T) {
	m := Person{}
	_, err := GetModelFieldTypeInModelIfValid(&m, "Pet.PetToy.Name2")
	assert.NotNil(t, err)
}

func TestGetModelFieldTypeInModelIfValid_NestedModel_HaveCorrectType(t *testing.T) {
	m := Person{}

	typ, err := GetModelFieldTypeInModelIfValid(&m, "Pet.Name")
	assert.Nil(t, err)
	assert.Equal(t, "string", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Pet.Name2")
	assert.Nil(t, err)
	assert.Equal(t, "string", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Pet.Age")
	assert.Nil(t, err)
	assert.Equal(t, "int", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Pet.Age2")
	assert.Nil(t, err)
	assert.Equal(t, "int", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Pet2.Name")
	assert.Nil(t, err)
	assert.Equal(t, "string", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Pet2.Name2")
	assert.Nil(t, err)
	assert.Equal(t, "string", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Pet2.Age")
	assert.Nil(t, err)
	assert.Equal(t, "int", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Pet2.Age2")
	assert.Nil(t, err)
	assert.Equal(t, "int", typ.Name())
}

func TestGetModelFieldTypeInModelIfValid_NestedSliceField_HaveCorrectType(t *testing.T) {
	m := Person{}

	typ, err := GetModelFieldTypeInModelIfValid(&m, "Contacts.Name")
	assert.Nil(t, err)
	assert.Equal(t, "string", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Contacts.Name2")
	assert.Nil(t, err)
	assert.Equal(t, "string", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Contacts.Age")
	assert.Nil(t, err)
	assert.Equal(t, "int", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Contacts.Age2")
	assert.Nil(t, err)
	assert.Equal(t, "int", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Contacts2.Name")
	assert.Nil(t, err)
	assert.Equal(t, "string", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Contacts2.Name2")
	assert.Nil(t, err)
	assert.Equal(t, "string", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Contacts2.Age")
	assert.Nil(t, err)
	assert.Equal(t, "int", typ.Name())

	typ, err = GetModelFieldTypeInModelIfValid(&m, "Contacts2.Age2")
	assert.Nil(t, err)
	assert.Equal(t, "int", typ.Name())
}

func TestGetModelFieldTypeInModelIfValid_NonExistingNestedModel_HasError(t *testing.T) {
	m := Person{}

	_, err := GetModelFieldTypeInModelIfValid(&m, "NotExisting.Name")
	assert.Error(t, err)

	_, err = GetModelFieldTypeInModelIfValid(&m, "NotExisting.Name2")
	assert.Error(t, err)

	_, err = GetModelFieldTypeInModelIfValid(&m, "NotExisting.Age")
	assert.Error(t, err)

	_, err = GetModelFieldTypeInModelIfValid(&m, "NotExisting.Age2")
	assert.Error(t, err)
}

func TestGetModelFieldTypeInModelIfValid_TwoLevelNestedField_HasCorrectType(t *testing.T) {
	m := Person{}

	typ, err := GetModelFieldTypeInModelIfValid(&m, "Pet.PetToy.Name")
	assert.Nil(t, err)
	assert.Equal(t, "string", typ.Name())
}
