package query

import (
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
)

func Test_IQuery_QueryComformsToIQuery_Works(t *testing.T) {
	var db *gorm.DB
	q := DB(db)
	_, ok := q.(IQuery)
	assert.True(t, ok)
}
