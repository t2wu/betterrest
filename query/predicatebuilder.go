package query

import (
	"errors"
	"fmt"
)

// args is either two arguments: "Name =" "Christy", or another predicate builder C()
func C(args ...interface{}) *PredicateRelationBuilder {
	builder := NewPredicateRelationBuilder()
	builder.addPredicateOrBuilder(args...)

	return builder
}

func NewPredicateRelationBuilder() *PredicateRelationBuilder {
	return &PredicateRelationBuilder{
		Rel: &PredicateRelation{
			PredOrRels: make([]Criteria, 0),
			Logics:     make([]PredicateLogic, 0),
		},
	}
}

type PredicateRelationBuilder struct {
	Rel   *PredicateRelation
	Error error // This allow us to chain and eventually discover any error by querying for Error
}

func (p *PredicateRelationBuilder) GetPredicateRelation() (*PredicateRelation, error) {
	return p.Rel, p.Error
}

func (p *PredicateRelationBuilder) C(args ...interface{}) *PredicateRelationBuilder {
	if p.Error != nil {
		return p
	}

	if len(p.Rel.PredOrRels) != 0 || len(p.Rel.Logics) != 0 {
		p.Error = fmt.Errorf("calling C() when predicate or relation not empty")
		return p
	}

	p.addPredicateOrBuilder(args...)

	return p
}

// s is Name =?, v is value
func (p *PredicateRelationBuilder) And(args ...interface{}) *PredicateRelationBuilder {
	args = append(args, PredicateLogicAND)
	p.addRelation(args...)
	return p
}

// s is Name =?, v is value
func (p *PredicateRelationBuilder) Or(args ...interface{}) *PredicateRelationBuilder {
	args = append(args, PredicateLogicOR)
	p.addRelation(args...)
	return p
}

// s string, v interface{}, logic PredicateLogic
func (p *PredicateRelationBuilder) addRelation(args ...interface{}) *PredicateRelationBuilder {
	if p.Error != nil {
		return p
	}

	if len(args) >= 2 {
		p.addPredicateOrBuilder(args[:len(args)-1]...)
	} else {
		p.Error = errors.New("And or Or should have at least two arguments")
	}

	logic := args[len(args)-1].(PredicateLogic)
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

func (p *PredicateRelationBuilder) addPredicateOrBuilder(args ...interface{}) {
	if s, ok := args[0].(string); ok && len(args) == 2 {
		p.addPredicate(s, args[1])
	} else if b, ok := args[0].(*PredicateRelationBuilder); ok && len(args) == 1 {
		rel, err := b.GetPredicateRelation()
		if err != nil {
			p.Error = err
		} else {
			// NewPredicateRelationBuilder()
			// p.Rel = NewPredicateRelation()
			p.Rel.PredOrRels = append(p.Rel.PredOrRels, rel)
		}
	} else {
		p.Error = fmt.Errorf("argument to file-level C function incorrect type")
	}
}
