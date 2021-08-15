package query

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/libs/datatypes"
)

// --- Predicate ---

func TestPredicateFromStringAndVal_works(t *testing.T) {
	tests := []struct {
		query string
		value interface{}
		want  *Predicate
	}{
		{
			query: "Age >",
			value: 20,
			want: &Predicate{
				Field: "Age",
				Cond:  PredicateCondGT,
				Value: 20,
			}},
		{
			query: "Name =",
			value: "Christy",
			want: &Predicate{
				Field: "Name",
				Cond:  PredicateCondEQ,
				Value: "Christy",
			}},
	}

	for _, test := range tests {
		result, _ := NewPredicateFromStringAndVal(test.query, test.value)
		isTrue := reflect.DeepEqual(test.want, result)
		assert.True(t, isTrue)
	}
}

func TestPredicateFromStringAndVal_whenWrongValue_hasError(t *testing.T) {
	tests := []struct {
		query string
		value interface{}
		want  *Predicate
	}{
		{
			query: "Age > wrong",
			value: 20,
			want: &Predicate{
				Field: "Age",
				Cond:  PredicateCondGT,
				Value: 20,
			}},
	}

	for _, test := range tests {
		_, err := NewPredicateFromStringAndVal(test.query, test.value)
		assert.Error(t, err)
	}
}

func TestBuildQueryStringAndValueForAllTypeOfConditions_Works(t *testing.T) {
	tests := []struct {
		predicate *Predicate
		want      struct {
			s string
			v interface{}
		}
	}{
		{
			predicate: &Predicate{
				Field: "Age",
				Cond:  PredicateCondEQ,
				Value: 20,
			},
			want: struct {
				s string
				v interface{}
			}{s: "\"test_model\".age = ?", v: 20},
		},
		{
			predicate: &Predicate{
				Field: "Age",
				Cond:  PredicateCondLT,
				Value: 20,
			},
			want: struct {
				s string
				v interface{}
			}{s: "\"test_model\".age < ?", v: 20},
		},
		{
			predicate: &Predicate{
				Field: "Age",
				Cond:  PredicateCondLTEQ,
				Value: 20,
			},
			want: struct {
				s string
				v interface{}
			}{s: "\"test_model\".age <= ?", v: 20},
		},
		{
			predicate: &Predicate{
				Field: "Age",
				Cond:  PredicateCondGT,
				Value: 20,
			},
			want: struct {
				s string
				v interface{}
			}{s: "\"test_model\".age > ?", v: 20},
		},
		{
			predicate: &Predicate{
				Field: "Age",
				Cond:  PredicateCondGTEQ,
				Value: 20,
			},
			want: struct {
				s string
				v interface{}
			}{s: "\"test_model\".age >= ?", v: 20},
		},
	}
	for _, test := range tests {
		s, vals, err := test.predicate.BuildQueryStringAndValues(&TestModel{})
		assert.Nil(t, err)
		assert.Equal(t, test.want.s, s)
		if assert.Equal(t, 1, len(vals)) {
			assert.Equal(t, test.want.v, vals[0])
		}
	}
}

func TestBuildQueryStringAndValue_Escape_Rawtring(t *testing.T) {
	tests := []struct {
		predicate *Predicate
		want      struct {
			s string
		}
	}{
		{
			predicate: &Predicate{
				Field: "Age",
				Cond:  PredicateCondEQ,
				Value: &Escape{Value: "20"},
			},
			want: struct {
				s string
			}{s: "\"test_model\".age = 20"},
		},
	}
	for _, test := range tests {
		s, vals, err := test.predicate.BuildQueryStringAndValues(&TestModel{})
		assert.Nil(t, err)
		assert.Equal(t, test.want.s, s)
		assert.Equal(t, 0, len(vals))
	}
}

func TestBuildQueryStringAndValueForInClause_Works(t *testing.T) {
	tests := []struct {
		predicate *Predicate
		want      struct {
			s string
			v []string
		}
	}{
		{
			predicate: &Predicate{
				Field: "ID",
				Cond:  PredicateCondIN,
				Value: []*datatypes.UUID{
					datatypes.NewUUIDFromStringNoErr(uuid1),
					datatypes.NewUUIDFromStringNoErr(uuid2),
					datatypes.NewUUIDFromStringNoErr(uuid4),
				},
			},
			want: struct {
				s string
				v []string
			}{s: "\"test_model\".id IN (?, ?, ?)", v: []string{uuid1, uuid2, uuid4}},
		},
	}
	for _, test := range tests {
		s, vals, err := test.predicate.BuildQueryStringAndValues(&TestModel{})
		assert.Nil(t, err)
		assert.Equal(t, test.want.s, s)

		if assert.Equal(t, 1, len(vals)) {
			v2, ok := vals[0].([]*datatypes.UUID)
			if ok {
				assert.Equal(t, test.want.v[0], v2[0].String())
				assert.Equal(t, test.want.v[1], v2[1].String())
				assert.Equal(t, test.want.v[2], v2[2].String())
			} else {
				assert.Fail(t, "wrong type")
			}
		}
	}
}

func TestBuildQueryString_InnerStructQuery_Works(t *testing.T) {
	tests := []struct {
		predicate *Predicate
		want      struct {
			s string
			v interface{}
		}
	}{
		{
			predicate: &Predicate{
				Field: "Dogs.Name",
				Cond:  PredicateCondEQ,
				Value: "doggie1",
			},
			want: struct {
				s string
				v interface{}
			}{s: "\"dog\".name = ?", v: "doggie1"},
		},
	}
	for _, test := range tests {
		s, vals, err := test.predicate.BuildQueryStringAndValues(&TestModel{})
		assert.Nil(t, err)
		assert.Equal(t, test.want.s, s)
		if assert.Equal(t, len(vals), 1) {
			assert.Equal(t, test.want.v, vals[0])
		}
	}
}

func TestBuildQueryString_NonExistingInnerStructQuery_ReturnsError(t *testing.T) {
	tests := []struct {
		predicate *Predicate
		want      struct {
			s string
			v interface{}
		}
	}{
		{
			predicate: &Predicate{
				Field: "Bogus.Name",
				Cond:  PredicateCondEQ,
				Value: "doggie1",
			},
			want: struct {
				s string
				v interface{}
			}{s: "\"bogus\".name = ?", v: "doggie1"},
		},
	}
	for _, test := range tests {
		_, _, err := test.predicate.BuildQueryStringAndValues(&TestModel{})
		assert.Error(t, err)
	}
}

func TestBuildQueryString_Level2InnerStructQuery_Works(t *testing.T) {
	tests := []struct {
		predicate *Predicate
		want      struct {
			s string
			v interface{}
		}
	}{
		{
			predicate: &Predicate{
				Field: "Dogs.DogToy.ToyName",
				Cond:  PredicateCondEQ,
				Value: "MyToy",
			},
			want: struct {
				s string
				v interface{}
			}{s: "\"dog_toy\".toy_name = ?", v: "MyToy"},
		},
	}
	for _, test := range tests {
		s, vals, err := test.predicate.BuildQueryStringAndValues(&TestModel{})
		assert.Nil(t, err)
		assert.Equal(t, test.want.s, s)
		if assert.Equal(t, len(vals), 1) {
			assert.Equal(t, test.want.v, vals[0])
		}
	}
}

// --- PredicateRelation ---

func TestPredicateRelationStringAndValuesOnePredicte(t *testing.T) {
	tests := []struct {
		pr   *PredicateRelation
		want struct {
			s string
			v int
		}
	}{
		{
			pr: &PredicateRelation{
				PredOrRels: []Criteria{
					&Predicate{
						Field: "Age",
						Cond:  PredicateCondGT,
						Value: 20,
					},
				},
			},
			want: struct {
				s string
				v int
			}{
				s: "\"test_model\".age > ?",
				v: 20,
			},
		},
	}

	for _, test := range tests {
		s, vals, err := test.pr.BuildQueryStringAndValues(&TestModel{})
		assert.Nil(t, err)
		assert.Equal(t, test.want.s, s)
		if assert.Equal(t, 1, len(vals)) {
			assert.Equal(t, test.want.v, vals[0])
		}
	}
}

func TestPredicateRelationStringAndValuesWithSecondNested(t *testing.T) {
	innerPred := &PredicateRelation{
		PredOrRels: []Criteria{
			&Predicate{
				Field: "Name",
				Cond:  PredicateCondEQ,
				Value: "Christy",
			},
			&Predicate{
				Field: "Name",
				Cond:  PredicateCondEQ,
				Value: "Jenny",
			},
		},
		Logics: []PredicateLogic{PredicateLogicOR},
	}

	outerPred := &PredicateRelation{
		PredOrRels: []Criteria{
			&Predicate{
				Field: "Age",
				Cond:  PredicateCondGT,
				Value: 20,
			},
			&Predicate{
				Field: "Age",
				Cond:  PredicateCondLT,
				Value: 30,
			},
			innerPred,
		},
		Logics: []PredicateLogic{PredicateLogicAND, PredicateLogicAND},
	}
	s, vals, err := outerPred.BuildQueryStringAndValues(&TestModel{})
	assert.Nil(t, err)
	assert.Equal(t, "(\"test_model\".age > ?) AND (\"test_model\".age < ?) AND ((\"test_model\".real_name_column = ?) OR (\"test_model\".real_name_column = ?))", s)
	if assert.Equal(t, 4, len(vals)) {
		assert.Equal(t, 20, vals[0].(int), 20)
		assert.Equal(t, 30, vals[1].(int), 30)
		assert.Equal(t, "Christy", vals[2].(string))
		assert.Equal(t, "Jenny", vals[3].(string))

	}
}

func TestPredicateRelationStringAndValuesWithFirstNested(t *testing.T) {
	innerRel := &PredicateRelation{
		PredOrRels: []Criteria{
			&Predicate{
				Field: "Name",
				Cond:  PredicateCondEQ,
				Value: "Christy",
			},
			&Predicate{
				Field: "Name",
				Cond:  PredicateCondEQ,
				Value: "Jenny",
			},
		},
		Logics: []PredicateLogic{PredicateLogicOR},
	}

	outerPred := &PredicateRelation{
		PredOrRels: []Criteria{
			innerRel,
			&Predicate{
				Field: "Age",
				Cond:  PredicateCondGT,
				Value: 20,
			},
			&Predicate{
				Field: "Age",
				Cond:  PredicateCondLT,
				Value: 30,
			},
		},
		Logics: []PredicateLogic{PredicateLogicAND, PredicateLogicAND},
	}
	s, vals, err := outerPred.BuildQueryStringAndValues(&TestModel{})
	assert.Nil(t, err)
	assert.Equal(t, "((\"test_model\".real_name_column = ?) OR (\"test_model\".real_name_column = ?)) AND (\"test_model\".age > ?) AND (\"test_model\".age < ?)", s)
	if assert.Equal(t, 4, len(vals)) {
		assert.Equal(t, "Christy", vals[0].(string))
		assert.Equal(t, "Jenny", vals[1].(string))
		assert.Equal(t, 20, vals[2].(int), 20)
		assert.Equal(t, 30, vals[3].(int), 30)
	}
}

func TestBuildQueryString_DifferentLevelOfNesting_ReturnError(t *testing.T) {
	rel := &PredicateRelation{
		PredOrRels: []Criteria{
			&Predicate{
				Field: "Inner.Name",
				Cond:  PredicateCondEQ,
				Value: "Christy",
			},
			&Predicate{
				Field: "Name",
				Cond:  PredicateCondEQ,
				Value: "Jenny",
			},
		},
		Logics: []PredicateLogic{PredicateLogicOR},
	}

	_, _, err := rel.BuildQueryStringAndValues(&TestModel{})
	assert.Error(t, err)
}
