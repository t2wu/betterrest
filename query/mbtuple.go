package query

import (
	"sort"
	"strings"

	"github.com/t2wu/betterrest/models"
)

type ModelAndBuilder struct {
	modelObj     models.IModel // THe model this predicate relation applies to
	builderInfos []BuilderInfo // each builder is responsible for one-level of object for one model stack
}

// type byNestingLevel []BuilderInfo
// func (b *byNestingLevel)

// top level table has to be joined before more-nested table are joined
func (mb *ModelAndBuilder) SortBuilderInfosByLevel() {
	sort.SliceStable(mb.builderInfos, func(i, j int) bool {
		// sort from least to greatest
		return mb.builderInfos[i].builder.Rel.GetNestedLevel() < mb.builderInfos[j].builder.Rel.GetNestedLevel()
	})
}

func (mb *ModelAndBuilder) GetAllPotentialJoinStructDesignators() ([]string, error) {
	levelNested := make(map[int][]string)
	for _, builderInfo := range mb.builderInfos {
		rel, err := builderInfo.builder.GetPredicateRelation()
		if err != nil {
			return nil, err
		}

		for key := range rel.GetAllUnqueStructFieldDesignator() {
			c := strings.Count(key, ".")
			levelNested[c] = append(levelNested[c], key) // don't worry about duplicated here
		}
		// return allPotentialJoins, nil
		// return retvals, nil
	}

	levels := make([]int, 0)
	for level := range levelNested {
		levels = append(levels, level)
	}

	retvals := make([]string, 0)
	// allPotentialJoins := make(map[string]interface{})

	sort.Ints(levels)
	for level := range levels {
		retvals = append(retvals, levelNested[level]...)
	}

	return retvals, nil
}

type BuilderInfo struct {
	builder   *PredicateRelationBuilder
	processed bool
}
