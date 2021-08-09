package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPredicateBuilder_whenGiven_HasPredicate(t *testing.T) {
	// builder := NewPredicateRelationBuilder()
	b := C("name =", "Christy")
	rel, err := b.GetPredicateRelation()

	assert.Nil(t, err)
	assert.Equal(t, 1, len(rel.PredOrRel), "there should be 1 predicate")
	p := rel.PredOrRel[0].(*Predicate)
	assert.Equal(t, p.Field, "name")
	assert.Equal(t, p.Cond, PredicateCondEQ)
	assert.Equal(t, p.Value, "Christy")
}

func TestPredicateBuilder_whenCallingCTwice_shouldHaveError(t *testing.T) {
	// builder := NewPredicateRelationBuilder()
	b := C("name =", "Christy")
	b.C("age =", 3)
	_, err := b.GetPredicateRelation()

	assert.Error(t, err)
}

func TestPredicateBuilder_whenGivenAndTwoPredicates_HasProperPredicateRelations(t *testing.T) {
	// builder := NewPredicateRelationBuilder()
	b := C("name =", "Christy").And("age >=", 20)
	rel, err := b.GetPredicateRelation()

	assert.Nil(t, err)
	assert.Equal(t, 2, len(rel.PredOrRel))
	assert.Equal(t, 1, len(rel.Logics))

	p := rel.PredOrRel[0].(*Predicate)
	assert.Equal(t, p.Field, "name")
	assert.Equal(t, p.Cond, PredicateCondEQ)
	assert.Equal(t, p.Value, "Christy")
	p2 := rel.PredOrRel[1].(*Predicate)
	assert.Equal(t, p2.Field, "age")
	assert.Equal(t, p2.Cond, PredicateCondGTEQ)
	assert.Equal(t, p2.Value, 20)
	assert.Equal(t, rel.Logics[0], PredicateLogicAND)
}

func TestPredicateBuilder_whenGivenOrTwoPredicates_HasProperPredicateRelations(t *testing.T) {
	// builder := NewPredicateRelationBuilder()
	b := C("name =", "Christy").Or("age >=", 20)
	rel, err := b.GetPredicateRelation()

	assert.Nil(t, err)
	assert.Equal(t, 2, len(rel.PredOrRel))
	assert.Equal(t, 1, len(rel.Logics))

	p := rel.PredOrRel[0].(*Predicate)
	assert.Equal(t, p.Field, "name")
	assert.Equal(t, p.Cond, PredicateCondEQ)
	assert.Equal(t, p.Value, "Christy")
	p2 := rel.PredOrRel[1].(*Predicate)
	assert.Equal(t, p2.Field, "age")
	assert.Equal(t, p2.Cond, PredicateCondGTEQ)
	assert.Equal(t, p2.Value, 20)
	assert.Equal(t, rel.Logics[0], PredicateLogicOR)
}

func TestPredicateBuilder_whenGivenAndThreePredicates_HasProperPredicateRelations(t *testing.T) {
	// builder := NewPredicateRelationBuilder()
	b := C("name =", "Christy").Or("age >=", 20).And("age <", 60)
	rel, err := b.GetPredicateRelation()

	assert.Nil(t, err)
	assert.Equal(t, 3, len(rel.PredOrRel))
	p := rel.PredOrRel[0].(*Predicate)
	assert.Equal(t, p.Field, "name")
	assert.Equal(t, p.Cond, PredicateCondEQ)
	assert.Equal(t, p.Value, "Christy")
	p2 := rel.PredOrRel[1].(*Predicate)
	assert.Equal(t, p2.Field, "age")
	assert.Equal(t, p2.Cond, PredicateCondGTEQ)
	assert.Equal(t, p2.Value, 20)
	p3 := rel.PredOrRel[2].(*Predicate)
	assert.Equal(t, p3.Field, "age")
	assert.Equal(t, p3.Cond, PredicateCondLT)
	assert.Equal(t, p3.Value, 60)
	assert.Equal(t, rel.Logics[0], PredicateLogicOR)
	assert.Equal(t, rel.Logics[1], PredicateLogicAND)
}

func TestPredicateBuilder_whenFirstWithNestedBuilder_HasProperPredicateRelations(t *testing.T) {
	bNested := C("name =", "Christy")
	b := C(bNested).Or("age <=", 100)
	rel, err := b.GetPredicateRelation()

	assert.Nil(t, err)
	assert.Equal(t, 2, len(rel.PredOrRel))

	rel2 := rel.PredOrRel[0].(*PredicateRelation)
	assert.Equal(t, 0, len(rel2.Logics))
	assert.Equal(t, 1, len(rel2.PredOrRel))
	nestedP, _ := rel2.PredOrRel[0].(*Predicate)
	assert.Equal(t, "name", nestedP.Field)
	assert.Equal(t, PredicateCondEQ, nestedP.Cond)
	assert.Equal(t, "Christy", nestedP.Value)

	p := rel.PredOrRel[1].(*Predicate)
	assert.Equal(t, "age", p.Field)
	assert.Equal(t, PredicateCondLTEQ, p.Cond)
	assert.Equal(t, 100, p.Value)
}

// func TestPredicateBuilder_nestedPredicates_works(t *testing.T) {
// 	// Christy less than 20 years old or Amy greater tan 60 years gold
// 	b1 := C("name =", "Christy").And("age <", 20)
// 	b2 := C("name =", "Amy").And("age >", 60)
// 	b3 := C(b1).And(b2)
// 	rel, err := b3.GetPredicateRelation()
// }

// func foo(){
// 	/// Predicate is used to represent something like age < 20
// // type Predicate struct {
// // 	Field string        // e.g. age
// // 	Cond  PredicateCond // e.g. <
// // 	Value interface{}   // e.g. 20
// // }

// 	.And("age =", 23)
// 	b2 := NewPredicateRelationBuilder().Build("name =", "Jenny").And("age <", 30)

// 	And(Or("age =", 23, "age <", 3), "name =", "Jenny")
// 	And(Or("age =", 23, "age <", 3), Or("name =", "Jenny", "name =", "Christy")).And()

// 	b1 := C("age =", 23).Or().C("age <", 3) // age = 23 or age < 3
// 	b2 := C("gender =", "female")           // gender = female
// 	C(b1).And(b2)                           // (age = 23 or age < 3) AND (gender = "female")
// 	C(b1).And(b2).Or(b3)                    // (age = 23 or age < 3) AND (gender = "female") OR xxx // But AND has higher precedence
// 	c(b1).And(c(b2).Or(b3))                 // equivalent
// }
