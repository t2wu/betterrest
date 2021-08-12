package jsontrans

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_fieldsPicked_Exists(t *testing.T) {
	f := &JSONFields{
		"Weight": FieldOmitEmpty,
		"Age":    FieldOmitEmpty,
	}

	m := make(map[string]interface{})
	m["Weight"] = 30
	m["Age"] = 20.0

	data, err := Transform(m, f, PermissionWhiteList)
	assert.Nil(t, err)
	assert.Equal(t, 30, data["Weight"])
	assert.Equal(t, 20.0, data["Age"])
}

func Test_fieldsNotPicked_NotExists(t *testing.T) {
	f := &JSONFields{
		"Weight": FieldOmitEmpty,
	}

	m := make(map[string]interface{})
	m["Age"] = 20.0

	data, err := Transform(m, f, PermissionWhiteList)

	assert.Nil(t, err)
	_, ok := data["Age"]
	assert.False(t, ok)
}

func Test_fieldsOmitEmpty_NotThereIfNil(t *testing.T) {
	f := &JSONFields{
		"Weight": FieldOmitEmpty,
	}

	m := make(map[string]interface{})
	m["Weight"] = nil

	data, err := Transform(m, f, PermissionWhiteList)

	assert.Nil(t, err)
	_, ok := data["Weight"]
	assert.False(t, ok)
}

func Test_fieldsIncludeEmpty_IncludedNil(t *testing.T) {
	f := &JSONFields{
		"Weight": FieldIncludeEmpty,
	}

	m := make(map[string]interface{})
	m["Weight"] = nil

	data, err := Transform(m, f, PermissionWhiteList)

	assert.Nil(t, err)
	v, ok := data["Weight"]
	assert.True(t, ok)
	assert.Nil(t, v)
}
