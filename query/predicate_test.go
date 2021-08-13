package query

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/libs/datatypes"
)

func TestPredicateFromStringAndVal_works(t *testing.T) {
	tests := []struct {
		query string
		value interface{}
		want  *Predicate
	}{
		{query: "age >", value: 20, want: &Predicate{
			Field: "age",
			Cond:  PredicateCondGT,
			Value: 20,
		}},
		{query: "name =", value: "Christy", want: &Predicate{
			Field: "name",
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
		{query: "age > wrong", value: 20, want: &Predicate{
			Field: "age",
			Cond:  PredicateCondGT,
			Value: 20,
		}},
	}

	for _, test := range tests {
		_, err := NewPredicateFromStringAndVal(test.query, test.value)
		assert.Error(t, err)
	}
}

func TestBuildQueryStringAndValue(t *testing.T) {
	tests := []struct {
		predicate *Predicate
		want      struct {
			s string
			v interface{}
		}
	}{
		{
			predicate: &Predicate{
				Field: "age",
				Cond:  PredicateCondEQ,
				Value: 20,
			},
			want: struct {
				s string
				v interface{}
			}{s: "age = ?", v: 20},
		},
		{
			predicate: &Predicate{
				Field: "age",
				Cond:  PredicateCondLT,
				Value: 20,
			},
			want: struct {
				s string
				v interface{}
			}{s: "age < ?", v: 20},
		},
		{
			predicate: &Predicate{
				Field: "age",
				Cond:  PredicateCondLTEQ,
				Value: 20,
			},
			want: struct {
				s string
				v interface{}
			}{s: "age <= ?", v: 20},
		},
		{
			predicate: &Predicate{
				Field: "age",
				Cond:  PredicateCondGT,
				Value: 20,
			},
			want: struct {
				s string
				v interface{}
			}{s: "age > ?", v: 20},
		},
		{
			predicate: &Predicate{
				Field: "age",
				Cond:  PredicateCondGTEQ,
				Value: 20,
			},
			want: struct {
				s string
				v interface{}
			}{s: "age >= ?", v: 20},
		},
	}
	for _, test := range tests {
		s, v := test.predicate.BuildQueryStringAndValue()
		assert.Equal(t, test.want.s, s)
		assert.Equal(t, test.want.v, v)
	}
}

func TestBuildQueryStringAndValueForInClause(t *testing.T) {
	tests := []struct {
		predicate *Predicate
		want      struct {
			s string
			v []string
		}
	}{
		{
			predicate: &Predicate{
				Field: "id",
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
			}{s: "id IN (?, ?, ?)", v: []string{uuid1, uuid2, uuid4}},
		},
	}
	for _, test := range tests {
		s, v := test.predicate.BuildQueryStringAndValue()
		assert.Equal(t, test.want.s, s)

		v2, ok := v.([]*datatypes.UUID)
		if ok {
			assert.Equal(t, test.want.v[0], v2[0].String())
			assert.Equal(t, test.want.v[1], v2[1].String())
			assert.Equal(t, test.want.v[2], v2[2].String())
		} else {
			assert.Fail(t, "wrong type")
		}
	}
}

func TestBuildQueryStringAndValueWithNonNested(t *testing.T) {
	tests := []struct {
		pr   *PredicateRelation
		want struct {
			s string
			v []interface{}
		}
	}{
		{
			pr: &PredicateRelation{
				PredOrRel: []interface{}{
					&Predicate{
						Field: "age",
						Cond:  PredicateCondGT,
						Value: 20,
					},
					&Predicate{
						Field: "age",
						Cond:  PredicateCondLT,
						Value: 30,
					},
				},
				Logics: []PredicateLogic{PredicateLogicAND},
			},
			want: struct {
				s string
				v []interface{}
			}{
				s: "age > ? AND age < ?",
				v: []interface{}{20, 30},
			},
		},
	}

	for _, test := range tests {
		s, v := test.pr.BuildQueryStringAndValues()
		assert.Equal(t, test.want.s, s)
		assert.Equal(t, test.want.v[0], v[0])
		assert.Equal(t, test.want.v[1], v[1])
	}
}

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
				PredOrRel: []interface{}{
					&Predicate{
						Field: "age",
						Cond:  PredicateCondGT,
						Value: 20,
					},
				},
			},
			want: struct {
				s string
				v int
			}{
				s: "age > ?",
				v: 20,
			},
		},
	}

	for _, test := range tests {
		s, v := test.pr.BuildQueryStringAndValues()
		assert.Equal(t, test.want.s, s)
		assert.Equal(t, test.want.v, v[0])
	}
}

func TestPredicateRelationStringAndValuesWithSecondNested(t *testing.T) {
	innerPred := &PredicateRelation{
		PredOrRel: []interface{}{
			&Predicate{
				Field: "name",
				Cond:  PredicateCondEQ,
				Value: "Christy",
			},
			&Predicate{
				Field: "name",
				Cond:  PredicateCondEQ,
				Value: "Jenny",
			},
		},
		Logics: []PredicateLogic{PredicateLogicOR},
	}

	outerPred := &PredicateRelation{
		PredOrRel: []interface{}{
			&Predicate{
				Field: "age",
				Cond:  PredicateCondGT,
				Value: 20,
			},
			&Predicate{
				Field: "age",
				Cond:  PredicateCondLT,
				Value: 30,
			},
			innerPred,
		},
		Logics: []PredicateLogic{PredicateLogicAND, PredicateLogicAND},
	}
	s, v := outerPred.BuildQueryStringAndValues()
	assert.Equal(t, "age > ? AND age < ? AND (name = ? OR name = ?)", s)
	assert.Equal(t, 20, v[0].(int), 20)
	assert.Equal(t, 30, v[1].(int), 30)
	assert.Equal(t, "Christy", v[2].(string))
	assert.Equal(t, "Jenny", v[3].(string))
}

func TestPredicateRelationStringAndValuesWithFirstNested(t *testing.T) {
	innerPred := &PredicateRelation{
		PredOrRel: []interface{}{
			&Predicate{
				Field: "name",
				Cond:  PredicateCondEQ,
				Value: "Christy",
			},
			&Predicate{
				Field: "name",
				Cond:  PredicateCondEQ,
				Value: "Jenny",
			},
		},
		Logics: []PredicateLogic{PredicateLogicOR},
	}

	outerPred := &PredicateRelation{
		PredOrRel: []interface{}{
			innerPred,
			&Predicate{
				Field: "age",
				Cond:  PredicateCondGT,
				Value: 20,
			},
			&Predicate{
				Field: "age",
				Cond:  PredicateCondLT,
				Value: 30,
			},
		},
		Logics: []PredicateLogic{PredicateLogicAND, PredicateLogicAND},
	}
	s, v := outerPred.BuildQueryStringAndValues()
	assert.Equal(t, "(name = ? OR name = ?) AND age > ? AND age < ?", s)
	assert.Equal(t, "Christy", v[0].(string))
	assert.Equal(t, "Jenny", v[1].(string))
	assert.Equal(t, 20, v[2].(int), 20)
	assert.Equal(t, 30, v[3].(int), 30)
}

func TestFirstByPredicateRelation_works(t *testing.T) {
	tm := TestModel{}
	p, _ := NewPredicateFromStringAndVal("real_name_column =", "same")
	rel := &PredicateRelation{
		PredOrRel: []interface{}{p},
	}
	if err := FirstByPredicateRelation(db, &tm, rel); err != nil {
		assert.Fail(t, err.Error())
	}
	assert.Equal(t, tm.ID.String(), uuid4)
}

func TestFindByPredicateRelation_works(t *testing.T) {
	tms := make([]TestModel, 0)
	p, _ := NewPredicateFromStringAndVal("real_name_column =", "same")
	rel := &PredicateRelation{
		PredOrRel: []interface{}{p},
	}
	if err := FindByPredicateRelation(db, &tms, rel); err != nil {
		assert.Fail(t, err.Error())
	}

	assert.Equal(t, tms[0].ID.String(), uuid4)
	assert.Equal(t, tms[1].ID.String(), uuid3)
}
