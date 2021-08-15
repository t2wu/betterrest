package models

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/libs/utils/jsontrans"
)

// Unfortunately, I can't test jsontransfrom from within
// because I need to derive reflect.Value from an IModel
// and that would run into circular dependency

type TestModel struct {
	BaseModel

	Name1 string  `json:"name1"`
	Name2 *string `json:"name2"`
	Age1  int     `json:"age1"`
	Age2  *int    `json:"age2"`

	InnerModel1 InnerModel  `json:"innerModel1"`
	InnerModel2 *InnerModel `json:"innerModel2"`
}

type InnerModel struct {
	Name1 string  `json:"name1"`
	Name2 *string `json:"name2"`
}

func TestJsonTrans_FilledFields_Transformed(t *testing.T) {
	// f := &JSONFields{
	// 	"Name1": FieldOmitEmpty,
	// 	"Name2": FieldOmitEmpty,
	// }
	name := "MyName2"
	age := 5
	m := TestModel{
		Name1: "MyName1",
		Name2: &name,
		Age1:  3,
		Age2:  &age,
	}
	var modelObj IModel
	modelObj = &m
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	mapping := jsontrans.TransFromByHidingDateFieldsFromIModel(v, true)

	_, ok1 := mapping["name1"]
	_, ok2 := mapping["name2"]
	_, ok3 := mapping["age1"]
	_, ok4 := mapping["age2"]

	assert.True(t, ok1)
	assert.True(t, ok2)
	assert.True(t, ok3)
	assert.True(t, ok4)
}

func TestJsonTrans_MissingField_TransfromedToNilValue(t *testing.T) {
	m := TestModel{}
	var modelObj IModel
	modelObj = &m
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	mapping := jsontrans.TransFromByHidingDateFieldsFromIModel(v, true)
	v1, _ := mapping["name1"]
	v2, _ := mapping["name2"]
	v3, _ := mapping["age1"]
	v4, _ := mapping["age2"]
	// v5, _ := mapping["innerModel1"] // well this is a struct nevertheless
	v6, _ := mapping["innerModel2"]

	assert.Equal(t, "", v1)
	assert.Nil(t, v2)
	assert.Equal(t, 0, v3)
	assert.Nil(t, v4)
	assert.Nil(t, v6)
}

func TestJsonTrans_DatesField_NotTransformed(t *testing.T) {
	m := TestModel{}
	var modelObj IModel
	modelObj = &m
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	mapping := jsontrans.TransFromByHidingDateFieldsFromIModel(v, false)
	_, ok1 := mapping["createdAt"]
	_, ok2 := mapping["updatedAt"]
	_, ok3 := mapping["deletedAt"]
	assert.False(t, ok1)
	assert.False(t, ok2)
	assert.False(t, ok3)
}

func TestJsonTrans_InnerModel_Transformed(t *testing.T) {
	inner1name2 := "inner1name2"
	inner2name2 := "inner2name2"
	m := TestModel{
		InnerModel1: InnerModel{
			Name1: "inner1name1",
			Name2: &inner1name2,
		},
		InnerModel2: &InnerModel{
			Name1: "inner2name1",
			Name2: &inner2name2,
		},
	}
	var modelObj IModel
	modelObj = &m
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	mapping := jsontrans.TransFromByHidingDateFieldsFromIModel(v, true)

	if inner1, ok := mapping["innerModel1"]; ok {
		if m, ok := inner1.(map[string]interface{}); ok {
			assert.Equal(t, "inner1name1", m["name1"].(string))
			if v, ok2 := m["name2"].(*string); ok2 {
				assert.Equal(t, "inner1name2", *v)
			} else {
				assert.Fail(t, "inner1name2 should be *string")
			}
		} else {
			assert.Fail(t, "inner1 should exists")
		}
	} else {
		assert.Fail(t, "inner1 should exists")
	}

	if inner2, ok := mapping["innerModel2"]; ok {
		if m, ok := inner2.(map[string]interface{}); ok {
			assert.Equal(t, "inner2name1", m["name1"].(string))
			if v, ok2 := m["name2"].(*string); ok2 {
				assert.Equal(t, "inner2name2", *v)
			} else {
				assert.Fail(t, "inner1name2 should be *string")
			}
		} else {
			assert.Fail(t, "inner2 should exists")
		}
	} else {
		assert.Fail(t, "inner2 should exists")
	}
}

func TestJsonTrans_InnerModel_NilFieldsAreEmpty(t *testing.T) {
	m := TestModel{
		InnerModel1: InnerModel{
			Name2: nil,
		},
		InnerModel2: &InnerModel{
			Name2: nil,
		},
	}
	var modelObj IModel
	modelObj = &m
	v := reflect.Indirect(reflect.ValueOf(modelObj))
	mapping := jsontrans.TransFromByHidingDateFieldsFromIModel(v, true)

	if inner1, ok := mapping["innerModel1"]; ok {
		if m, ok := inner1.(map[string]interface{}); ok {
			assert.Equal(t, "", m["name1"].(string))
			if v, ok2 := m["name2"].(*string); ok2 {
				assert.Nil(t, v)
			} else {
				assert.Fail(t, "inner1name2 should be *string")
			}
		} else {
			assert.Fail(t, "inner1 should exists")
		}
	} else {
		assert.Fail(t, "inner1 should exists")
	}

	if inner2, ok := mapping["innerModel2"]; ok {
		if m, ok := inner2.(map[string]interface{}); ok {
			assert.Equal(t, "", m["name1"].(string))
			if v, ok2 := m["name2"].(*string); ok2 {
				assert.Nil(t, v)
			} else {
				assert.Fail(t, "inner1name2 should be *string")
			}
		} else {
			assert.Fail(t, "inner2 should exists")
		}
	} else {
		assert.Fail(t, "inner2 should exists")
	}
}
