package query

import (
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
)

func Test_IQuery_QueryComformsToIQuery_Works(t *testing.T) {
	var db *gorm.DB
	q := DB(db)
	var iquery IQuery
	iquery = q // well, if not conforms this already fails.
	_, ok := iquery.(IQuery)
	assert.True(t, ok)
}
