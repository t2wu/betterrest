package query

import (
	"fmt"

	"github.com/t2wu/betterrest/models"
)

// args is either two arguments: "Name =" "Christy", or another predicate builder C()
func C(args ...interface{}) *PredicateRelationBuilder {
	builder := &PredicateRelationBuilder{
		Rel: &PredicateRelation{
			PredOrRels: make([]Criteria, 0),
			Logics:     make([]PredicateLogic, 0),
		},
	}

	if s, ok := args[0].(string); ok && len(args) == 2 {
		return builder.C(s, args[1])
	} else if b, ok := args[0].(*PredicateRelationBuilder); ok && len(args) == 1 {
		rel, err := b.GetPredicateRelation()
		if err != nil {
			builder.Error = err
		} else {
			// NewPredicateRelationBuilder()
			builder.Rel = NewPredicateRelation()
			builder.Rel.PredOrRels = append(builder.Rel.PredOrRels, rel)
		}
	} else {
		builder.Error = fmt.Errorf("argument to file-level C function incorrect type")
	}

	return builder
}

type PredicateRelationBuilder struct {
	Rel   *PredicateRelation
	Error error // This allow us to chain and eventually discover any error by querying for Error
}

func (p *PredicateRelationBuilder) BuildQueryStringAndValues(model models.IModel) (string, []interface{}, error) {
	rel, err := p.GetPredicateRelation()
	if err != nil {
		return "", nil, err
	}
	return rel.BuildQueryStringAndValues(model)
}

func (p *PredicateRelationBuilder) GetPredicateRelation() (*PredicateRelation, error) {
	return p.Rel, p.Error
}

// s is Name =?, v is value
func (p *PredicateRelationBuilder) C(s string, v interface{}) *PredicateRelationBuilder {
	if p.Error != nil {
		return p
	}

	// If calling C() and not And() or Or(), it means that predicate should be empty
	// (first time, has not called And() or Or())
	if len(p.Rel.PredOrRels) != 0 || len(p.Rel.Logics) != 0 {
		p.Error = fmt.Errorf("calling C() when predicate or relation not empty")
		return p
	}

	p.addPredicate(s, v)
	return p
}

// s is Name =?, v is value
func (p *PredicateRelationBuilder) And(s string, v interface{}) *PredicateRelationBuilder {
	p.addRelation(s, v, PredicateLogicAND)
	return p
}

// s is Name =?, v is value
func (p *PredicateRelationBuilder) Or(s string, v interface{}) *PredicateRelationBuilder {
	p.addRelation(s, v, PredicateLogicOR)
	return p
}

func (p *PredicateRelationBuilder) addRelation(s string, v interface{}, logic PredicateLogic) *PredicateRelationBuilder {
	if p.Error != nil {
		return p
	}
	p.addPredicate(s, v)
	p.Rel.Logics = append(p.Rel.Logics, logic)
	return p
}

// s is Name =?, v is value
func (p *PredicateRelationBuilder) addPredicate(s string, v interface{}) {
	pred, err := NewPredicateFromStringAndVal(s, v)
	if err != nil {
		p.Error = err
	} else {
		p.Rel.PredOrRels = append(p.Rel.PredOrRels, pred)
	}
}

// ---------------------------------------------------
