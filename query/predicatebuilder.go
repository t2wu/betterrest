package query

import (
	"fmt"
)

// So we can do something like AND("name =", "Christy").OR("age = ", 2)

// func C(s string, v interface{}) *PredicateRelationBuilder {
func C(args ...interface{}) *PredicateRelationBuilder {
	builder := NewPredicateRelationBuilder()
	if s, ok := args[0].(string); ok && len(args) == 2 {
		return builder.C(s, args[1])
	} else if b, ok := args[0].(*PredicateRelationBuilder); ok && len(args) == 1 {
		rel, err := b.GetPredicateRelation()
		if err != nil {
			builder.Error = err
		} else {
			NewPredicateRelationBuilder()
			builder.Rel = NewPredicateRelation()
			builder.Rel.PredOrRel = append(builder.Rel.PredOrRel, rel)
		}
	} else {
		builder.Error = fmt.Errorf("argument to file-level C function incorrect type")
	}

	return builder
}

func NewPredicateRelationBuilder() *PredicateRelationBuilder {
	return &PredicateRelationBuilder{
		Rel: &PredicateRelation{
			PredOrRel: make([]interface{}, 0),
			Logics:    make([]PredicateLogic, 0),
		},
	}
}

type PredicateRelationBuilder struct {
	Rel   *PredicateRelation
	Error error // This allow us to chain and eventually discover any error by querying for Error
}

func (p *PredicateRelationBuilder) BuildQueryStringAndValues() (string, []interface{}, error) {
	rel, err := p.GetPredicateRelation()
	if err != nil {
		return "", nil, err
	}
	query, values := rel.BuildQueryStringAndValues()
	return query, values, nil
}

func (p *PredicateRelationBuilder) GetPredicateRelation() (*PredicateRelation, error) {
	return p.Rel, p.Error
}

func (p *PredicateRelationBuilder) C(s string, v interface{}) *PredicateRelationBuilder {
	if p.Error != nil {
		return p
	}

	// If calling C and not AND or OR, it means that predicate should be empty
	if len(p.Rel.PredOrRel) != 0 || len(p.Rel.Logics) != 0 {
		p.Error = fmt.Errorf("calling C() when predicate or relation not empty")
		return p
	}

	p.addPredicate(s, v)
	return p
}

func (p *PredicateRelationBuilder) And(s string, v interface{}) *PredicateRelationBuilder {
	p.addRelation(s, v, PredicateLogicAND)
	return p
}

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

func (p *PredicateRelationBuilder) addPredicate(s string, v interface{}) {
	pred, err := NewPredicateFromStringAndVal(s, v)
	if err != nil {
		p.Error = err
	} else {
		p.Rel.PredOrRel = append(p.Rel.PredOrRel, pred)
	}
}
