package query

import (
	"fmt"
	"strings"
)

type PredicateCond string

const (
	// PredicateCondEQ is equals
	PredicateCondEQ PredicateCond = "="
	// PredicateCondLT is less than
	PredicateCondLT PredicateCond = "<"
	// PredicateCondLTEQ is less than or equal to
	PredicateCondLTEQ PredicateCond = "<="
	// PredicateCondGT is equal to
	PredicateCondGT PredicateCond = ">"
	// PredicateCondGTEQ is greater than or equal to
	PredicateCondGTEQ PredicateCond = ">="
)

type PredicateLogic string

const (
	PredicateLogicAND PredicateLogic = "AND"
	PredicateLogicOR  PredicateLogic = "OR"
)

// Predicate is used to represent something like age < 20
type Predicate struct {
	Field string        // e.g. age
	Cond  PredicateCond // e.g. <
	Value interface{}   // e.g. 20
}

func (p *Predicate) BuildQueryStringAndValue() (string, interface{}) {
	return fmt.Sprintf("%s %s ?", p.Field, p.Cond), p.Value
}

// NewPredicateFromStringAndVal, turn string like "age <" and value into proper predicate
// This is for convenience
// I cannot get "age < 20" directly because I'd have to know in advance the type
// of object (unless of course I just send it as a string, wonder if SQL can take it)
func NewPredicateFromStringAndVal(s string, value interface{}) (*Predicate, error) {
	toks := strings.Split(strings.TrimSpace(s), " ")
	if len(toks) != 2 {
		return nil, fmt.Errorf("PredicateFromString format incorrect")
	}

	return &Predicate{
		Field: toks[0],
		Cond:  PredicateCond(toks[1]),
		Value: value,
	}, nil
}

func NewPredicateRelation() *PredicateRelation {
	return &PredicateRelation{
		PredOrRel: make([]interface{}, 0),
		Logics:    make([]PredicateLogic, 0),
	}
}

// PredicateRelation represents things like (age < 20 OR age > 70 OR age = 30)
// Or it can contain other relations (age < 20 AND (name = "Timothy" OR name = "Christy") by
// compositing another relations
type PredicateRelation struct {
	// PredOrRel contains either be a *Predicate or *PredicateRelation
	// If PredicateRelation than it is nested comparison
	PredOrRel []interface{}
	Logics    []PredicateLogic // AND or OR. The number of Logic operators is one less than the number of predicates
}

func (pr *PredicateRelation) BuildQueryStringAndValues() (string, []interface{}) {
	operand := pr.PredOrRel[0]
	var str string
	var v interface{}
	var v2 []interface{}
	values := make([]interface{}, 0)
	isPred := false
	if p, ok := operand.(*Predicate); ok {
		isPred = true
		str, v = p.BuildQueryStringAndValue()
		values = append(values, v)
	} else if rel, ok := operand.(*PredicateRelation); ok {
		str, v2 = rel.BuildQueryStringAndValues()
		values = append(values, v2...)
	}

	if len(pr.PredOrRel) == 1 {
		return str, values
	}

	var sb strings.Builder
	if isPred {
		sb.WriteString(str)
	} else {
		sb.WriteString(fmt.Sprintf("(%s)", str))
	}

	for i, operand := range pr.PredOrRel[1:] {
		var s string
		if p, ok := operand.(*Predicate); ok {
			s, v = p.BuildQueryStringAndValue()
			values = append(values, v)
			sb.WriteString(fmt.Sprintf(" %s %s", pr.Logics[i], s))
		} else if rel, ok := operand.(*PredicateRelation); ok {
			s, v2 = rel.BuildQueryStringAndValues()
			values = append(values, v2...)
			sb.WriteString(fmt.Sprintf(" %s (%s)", pr.Logics[i], s))
		}
	}

	return sb.String(), values
}
