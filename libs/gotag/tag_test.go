package gotag

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTagHasPrefix(t *testing.T) {
	tests := []struct {
		tagVal string
		want   bool
	}{
		{tagVal: "peg", want: true},
		{tagVal: "peg;", want: true},
		{tagVal: "peg;xxx", want: true},
		{tagVal: "xxx;peg;yyy", want: true},
		{tagVal: "xxx;", want: false},
	}

	for _, test := range tests {
		assert.Equal(t, test.want, TagValueHasPrefix(test.tagVal, "peg"))
	}
}

func TestTagReturnField(t *testing.T) {
	tests := []struct {
		tagVal string
		want   string
	}{
		{tagVal: "", want: ""},
		{tagVal: "peg", want: "peg"},
		{tagVal: "peg:xxx", want: "peg:xxx"},
		{tagVal: "xxx;peg:xyz;yyy", want: "peg:xyz"},
	}

	for _, test := range tests {
		assert.Equal(t, test.want, TagFieldByPrefix(test.tagVal, "peg"))
	}
}

// tests := []struct {
// 	query string
// 	value interface{}
// 	want  *Predicate
// }{
// 	{
// 		query: "Age >",
// 		value: 20,
// 		want: &Predicate{
// 			Field: "Age",
// 			Cond:  PredicateCondGT,
// 			Value: 20,
// 		},
// 	},
// 	{
// 		query: "Age >=",
// 		value: 20,
// 		want: &Predicate{
// 			Field: "Age",
// 			Cond:  PredicateCondGTEQ,
// 			Value: 20,
// 		},
// 	},
// 	{
// 		query: "Age <",
// 		value: 20,
// 		want: &Predicate{
// 			Field: "Age",
// 			Cond:  PredicateCondLT,
// 			Value: 20,
// 		},
// 	},
// 	{
// 		query: "Age <=",
// 		value: 20,
// 		want: &Predicate{
// 			Field: "Age",
// 			Cond:  PredicateCondLTEQ,
// 			Value: 20,
// 		},
// 	},
// 	{
// 		query: "Name =",
// 		value: "Christy",
// 		want: &Predicate{
// 			Field: "Name",
// 			Cond:  PredicateCondEQ,
// 			Value: "Christy",
// 		},
// 	},
// }

// for _, test := range tests {
// 	result, _ := NewPredicateFromStringAndVal(test.query, test.value)
// 	isTrue := reflect.DeepEqual(test.want, result)
// 	assert.True(t, isTrue)
// }
