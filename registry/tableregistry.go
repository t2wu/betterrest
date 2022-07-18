package registry

import (
	"log"
	"strings"
	"time"

	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/qry/datatype"
	"github.com/t2wu/qry/mdl"
)

// BetterRESTTable store the information on all other mdl
type BetterRESTTable struct {
	ID        *datatype.UUID `gorm:"type:uuid;primary_key;" json:"id"`
	CreatedAt time.Time      `json:"createdAt" json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`

	// Name is table name
	Name string `gorm:"unique_index:name"`
	// Version is table version
	Version string
}

// CreateBetterRESTTable registers mdl
func CreateBetterRESTTable() {
	// db.Shared().Exec("CREATE TABLE IF NOT EXISTS better_rest_table ")
	modelRegistry := ModelRegistry
	db.Shared().AutoMigrate(&BetterRESTTable{})

	for _, reg := range modelRegistry {
		id := datatype.NewUUID()
		createdAt := time.Now()
		updatedAt := time.Now()
		tableName := mdl.GetTableNameFromType(reg.Typ)

		if reg.TypVersion == "" {
			reg.TypVersion = "1.0.0"
		}

		var sb strings.Builder
		sb.WriteString("INSERT INTO better_rest_table (id, created_at, updated_at, name, version) ")
		sb.WriteString("VALUES (?, ?, ?, ?, ?) ON CONFLICT (name) DO UPDATE SET version = ?")
		if err := db.Shared().Exec(sb.String(), id, createdAt, updatedAt, tableName, reg.TypVersion, reg.TypVersion).Error; err != nil {
			log.Println("err:", err)
		}
	}
}
