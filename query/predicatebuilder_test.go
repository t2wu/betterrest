package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPredicateBuilder_whenGiven_HasPredicate(t *testing.T) {
	b := C("Name =", "Christy")
	rel, err := b.GetPredicateRelation()

	assert.Nil(t, err)
	assert.Equal(t, 1, len(rel.PredOrRels), "there should be 1 predicate")
	p := rel.PredOrRels[0].(*Predicate)
	assert.Equal(t, p.Field, "Name")
	assert.Equal(t, p.Cond, PredicateCondEQ)
	assert.Equal(t, p.Value, "Christy")
}

func TestPredicateBuilder_whenGivenWrongValue_HasError(t *testing.T) {
	b := C("Name deleteCmdForExample", "Christy")
	_, err := b.GetPredicateRelation()
	assert.Error(t, err)

	b = C("Name @#$#", "Christy")
	_, err = b.GetPredicateRelation()
	assert.Error(t, err)
}

func TestPredicateBuilder_whenCallingCTwice_shouldHaveError(t *testing.T) {
	b := C("Name =", "Christy")
	b.C("Age =", 3)
	_, err := b.GetPredicateRelation()

	assert.Error(t, err)
}

func TestPredicateBuilder_In_Works(t *testing.T) {
	b := C("Name IN", []string{"Christy", "Tina"})
	rel, err := b.GetPredicateRelation()

	assert.Nil(t, err)
	if !assert.Equal(t, 1, len(rel.PredOrRels), "there should be 1 predicate") {
		return
	}

	p := rel.PredOrRels[0].(*Predicate)
	assert.Equal(t, "Name", p.Field)
	assert.Equal(t, PredicateCondIN, p.Cond)
	if p2, ok := p.Value.([]string); ok {
		if assert.Equal(t, 2, len(p2)) {
			assert.Equal(t, "Christy", p2[0])
			assert.Equal(t, "Tina", p2[1])
		}
	}
}

func TestPredicateBuilder_whenGivenAndTwoPredicates_HasProperPredicateRelations(t *testing.T) {
	b := C("Name =", "Christy").And("Age >=", 20)
	rel, err := b.GetPredicateRelation()

	assert.Nil(t, err)
	assert.Equal(t, 2, len(rel.PredOrRels))
	assert.Equal(t, 1, len(rel.Logics))

	p := rel.PredOrRels[0].(*Predicate)
	assert.Equal(t, p.Field, "Name")
	assert.Equal(t, p.Cond, PredicateCondEQ)
	assert.Equal(t, p.Value, "Christy")
	p2 := rel.PredOrRels[1].(*Predicate)
	assert.Equal(t, p2.Field, "Age")
	assert.Equal(t, p2.Cond, PredicateCondGTEQ)
	assert.Equal(t, p2.Value, 20)
	assert.Equal(t, rel.Logics[0], PredicateLogicAND)
}

func TestPredicateBuilder_whenGivenOrTwoPredicates_HasProperPredicateRelations(t *testing.T) {
	b := C("Name =", "Christy").Or("Age >=", 20)
	rel, err := b.GetPredicateRelation()

	assert.Nil(t, err)
	assert.Equal(t, 2, len(rel.PredOrRels))
	assert.Equal(t, 1, len(rel.Logics))

	p := rel.PredOrRels[0].(*Predicate)
	assert.Equal(t, p.Field, "Name")
	assert.Equal(t, p.Cond, PredicateCondEQ)
	assert.Equal(t, p.Value, "Christy")
	p2 := rel.PredOrRels[1].(*Predicate)
	assert.Equal(t, p2.Field, "Age")
	assert.Equal(t, p2.Cond, PredicateCondGTEQ)
	assert.Equal(t, p2.Value, 20)
	assert.Equal(t, rel.Logics[0], PredicateLogicOR)
}

func TestPredicateBuilder_whenGivenAndThreePredicates_HasProperPredicateRelations(t *testing.T) {
	b := C("Name =", "Christy").Or("Age >=", 20).And("Age <", 60)
	rel, err := b.GetPredicateRelation()

	assert.Nil(t, err)
	assert.Equal(t, 3, len(rel.PredOrRels))
	p := rel.PredOrRels[0].(*Predicate)
	assert.Equal(t, p.Field, "Name")
	assert.Equal(t, p.Cond, PredicateCondEQ)
	assert.Equal(t, p.Value, "Christy")
	p2 := rel.PredOrRels[1].(*Predicate)
	assert.Equal(t, p2.Field, "Age")
	assert.Equal(t, p2.Cond, PredicateCondGTEQ)
	assert.Equal(t, p2.Value, 20)
	p3 := rel.PredOrRels[2].(*Predicate)
	assert.Equal(t, p3.Field, "Age")
	assert.Equal(t, p3.Cond, PredicateCondLT)
	assert.Equal(t, p3.Value, 60)
	assert.Equal(t, rel.Logics[0], PredicateLogicOR)
	assert.Equal(t, rel.Logics[1], PredicateLogicAND)
}

func TestPredicateBuilder_whenFirstWithNestedBuilder_HasProperPredicateRelations(t *testing.T) {
	b := C(C("Age <=", 20).Or("Age >=", 80)).And("Name =", "Christy")
	rel, err := b.GetPredicateRelation()

	assert.Nil(t, err)
	if assert.Equal(t, 2, len(rel.PredOrRels)) {
		// 1: C("Age <=", 20).Or("Age >=", 80)
		firstCriteria := rel.PredOrRels[0].(*PredicateRelation)

		{
			// inner of 1
			firstCriteriaOfFirst := firstCriteria.PredOrRels[0].(*Predicate)
			assert.Equal(t, "Age", firstCriteriaOfFirst.Field)
			assert.Equal(t, PredicateCondLTEQ, firstCriteriaOfFirst.Cond)
			assert.Equal(t, 20, firstCriteriaOfFirst.Value)

			if assert.Equal(t, 1, len(firstCriteria.Logics)) {
				assert.Equal(t, PredicateLogicOR, firstCriteria.Logics[0])
			}

			secondCriteriaOfFirst := firstCriteria.PredOrRels[1].(*Predicate)
			assert.Equal(t, "Age", secondCriteriaOfFirst.Field)
			assert.Equal(t, PredicateCondGTEQ, secondCriteriaOfFirst.Cond)
			assert.Equal(t, 80, secondCriteriaOfFirst.Value)

			// 2: And
			if assert.Equal(t, 1, len(rel.Logics)) {
				assert.Equal(t, PredicateLogicAND, rel.Logics[0])
			}
		}

		// 3: Name = Christy
		secondCriteria := rel.PredOrRels[1].(*Predicate)
		assert.Equal(t, "Name", secondCriteria.Field)
		assert.Equal(t, PredicateCondEQ, secondCriteria.Cond)
		assert.Equal(t, "Christy", secondCriteria.Value)
	}
}

func TestPredicateBuilder_whenFirstWithNestedModel_HasProperPredicateRelations(t *testing.T) {
	b := C("Dogs.DogToys.ToyName =", "DogToySameName")
	rel, err := b.GetPredicateRelation()
	if !assert.Nil(t, err) {
		return
	}
	s, vals, err := rel.BuildQueryStringAndValues(&TestModel{})
	if !assert.Nil(t, err) {
		return
	}

	assert.Equal(t, "\"dog_toy\".toy_name = ?", s) // table name + field name
	if assert.Equal(t, 1, len(vals)) {
		assert.Equal(t, "DogToySameName", vals[0])
	}
}
