package query

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/t2wu/betterrest/models"
)

// Defines Predicate{} and PredicateRelation{}
// Field refers to IModel field names

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
	// PredicateCondGTEQ is greater than or equal to
	PredicateCondIN PredicateCond = "IN"
)

func StringToPredicateCond(s string) (PredicateCond, error) {
	s2 := strings.ToUpper(string(s))
	switch s2 {
	case string(PredicateCondEQ):
		return PredicateCondEQ, nil
	case string(PredicateCondLT):
		return PredicateCondLT, nil
	case string(PredicateCondLTEQ):
		return PredicateCondLTEQ, nil
	case string(PredicateCondGT):
		return PredicateCondGT, nil
	case string(PredicateCondGTEQ):
		return PredicateCondGTEQ, nil
	case string(PredicateCondIN):
		return PredicateCondIN, nil
	}

	return PredicateCondEQ, fmt.Errorf("not a PredicateCond string")
}

type PredicateLogic string

const (
	PredicateLogicAND PredicateLogic = "AND"
	PredicateLogicOR  PredicateLogic = "OR"
)

type Criteria interface {
	BuildQueryStringAndValues(modelObj models.IModel) (string, []interface{}, error)

	// GetDesignatedModel gets the inner model within this modelObj designated by the criteria
	// If it's on the first-level, modelObj itself is returned
	GetDesignatedModel(modelObj models.IModel) (models.IModel, error)

	// GetDesignatedField gets the name of the current Field designator for the inner model
	// or empty string if it is modelObj itself
	GetDesignatedField(modelObj models.IModel) string
}

// Predicate is used to represent something like Age < 20
type Predicate struct {
	Field string        // e.g. Age
	Cond  PredicateCond // e.g. <, or IN
	Value interface{}   // e.g. 20 or an array of values
}

// If a predicte value is wrapped within an Escape class
// Assume it has a Stringer interface, and the result of the string
// is not wrapped in quotes as Postgres values, and also SQL injection is not checked
// So this should only be used internally
type Escape struct {
	Value string
}

// BuildQuryStringAndValues output proper query conditionals and the correponding values
// which field those fields
// Because this then is given to the database, the output needs to match the column names
func (p *Predicate) BuildQueryStringAndValues(modelObj models.IModel) (string, []interface{}, error) {
	// Check if it's inner
	var err error
	col := ""
	tblName := ""
	currModelObj := modelObj
	field := p.Field
	if strings.Contains(p.Field, ".") {
		// Inner, now we only want the field name
		toks := strings.Split(p.Field, ".")
		field = toks[len(toks)-1]
		fieldToModel := strings.Join(toks[:len(toks)-1], ".")
		currModelObj, err = GetInnerModelIfValid(modelObj, fieldToModel)
		if err != nil {
			return "", nil, err
		}
	}

	tblName = models.GetTableNameFromIModel(currModelObj)
	col, err = fieldToColumn(currModelObj, field)
	if err != nil {
		return "", nil, err
	}

	if p.Cond != PredicateCondIN {
		if escape, ok := p.Value.(*Escape); ok {
			return fmt.Sprintf("\"%s\".%s %s %s", tblName, col, p.Cond, escape.Value), []interface{}{}, nil
		} else {
			return fmt.Sprintf("\"%s\".%s %s ?", tblName, col, p.Cond), []interface{}{p.Value}, nil
		}
	}

	// The "IN" case, where p.Value is a slice
	values := reflect.ValueOf(p.Value)
	var sb strings.Builder
	sb.WriteString("?")
	for i := 1; i < values.Len(); i++ {
		sb.WriteString(", ?")
	}
	return fmt.Sprintf("\"%s\".%s %s (%s)", tblName, col, p.Cond, sb.String()), []interface{}{p.Value}, nil
}

func (p *Predicate) GetDesignatedModel(modelObj models.IModel) (models.IModel, error) {
	if strings.Contains(p.Field, ".") {
		// nested model
		toks := strings.Split(p.Field, ".")
		modelField := strings.Join(toks[:len(toks)-1], ".")
		return GetInnerModelIfValid(modelObj, modelField)
	} else {
		return modelObj, nil
	}
}

func (p *Predicate) GetDesignatedField(modelObj models.IModel) string {
	if strings.Contains(p.Field, ".") {
		// nested model
		toks := strings.Split(p.Field, ".")
		modelField := strings.Join(toks[:len(toks)-1], ".")
		return modelField
	} else {
		return ""
	}
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

	cond, err := StringToPredicateCond(toks[1])
	if err != nil {
		return nil, err
	}

	return &Predicate{
		Field: toks[0],
		Cond:  cond,
		Value: value,
	}, nil
}

func NewPredicateRelation() *PredicateRelation {
	return &PredicateRelation{
		PredOrRels: make([]Criteria, 0),
		Logics:     make([]PredicateLogic, 0),
	}
}

// PredicateRelation represents things like (age < 20 OR age > 70 OR age = 30)
// A Criteria is a Predicate or Predicate relation
// Every nested PredicateRelation is meant to work on one models.IModel. It can also designate criteria
// for nested class. But it cannot be used for another unrelated Model where there is no
// nesting relationships.
type PredicateRelation struct {
	// PredOrRel contains either be a *Predicate or *PredicateRelation
	// If PredicateRelation than it is nested comparison
	PredOrRels []Criteria
	Logics     []PredicateLogic // AND or OR. The number of Logic operators is one less than the number of predicates
}

func (pr *PredicateRelation) BuildQueryStringAndValues(modelObj models.IModel) (string, []interface{}, error) {
	operand := pr.PredOrRels[0]
	values := make([]interface{}, 0)
	isPred := false

	str, vals, err := operand.BuildQueryStringAndValues(modelObj)
	if err != nil {
		return "", nil, err
	}
	values = append(values, vals...)

	if len(pr.PredOrRels) == 1 {
		return str, values, nil
	}

	var sb strings.Builder
	if isPred {
		sb.WriteString(str)
	} else {
		sb.WriteString(fmt.Sprintf("(%s)", str))
	}

	for i, operand := range pr.PredOrRels[1:] {
		var s string

		s, vals, err = operand.BuildQueryStringAndValues(modelObj)
		if err != nil {
			return "", nil, err
		}
		values = append(values, vals...)
		// To simplify code, always wrap the query s in parenthesis (if it's predicate it's not really needed)
		sb.WriteString(fmt.Sprintf(" %s (%s)", pr.Logics[i], s))
	}

	return sb.String(), values, nil
}

func (pr *PredicateRelation) GetDesignatedModel(modelObj models.IModel) (models.IModel, error) {
	// All desigations are to the same model, so only need to grab one and check
	operand := pr.PredOrRels[0]
	return operand.GetDesignatedModel(modelObj)
}

func (pr *PredicateRelation) GetDesignatedField(modelObj models.IModel) string {
	// All desigations are to the same model, so only need to grab one and check
	operand := pr.PredOrRels[0]
	return operand.GetDesignatedField(modelObj)
}

// normalize query to column name query
func fieldToColumn(obj models.IModel, field string) (string, error) {
	col, err := FieldNameToColumn(obj, field) // this traverses the inner struct as well
	if err != nil {
		return "", err
	}
	return col, nil
}
