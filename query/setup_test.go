package query

import (
	"log"
	"os"
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

const (
	host     = "127.0.0.1"
	port     = "5432"
	username = "postgres"
	password = "12345678"
	dbname   = "localdb"

	uuid1 = "1e98bfc3-2721-492a-bfd3-09f7dd3c1565"
	uuid2 = "d113ed09-cfc5-47a5-b35c-6f60c49cbd08"
	uuid3 = "608a717a-bb4c-4a89-9038-457c3e4fc5e0"
	uuid4 = "bc3eedae-21a5-478f-93d1-a54dc5ad7559"

	doguuid1 = "a048824f-8728-4c0a-b091-ed8d59390542"
	doguuid2 = "f126d9b3-c09c-4857-9a88-cf2e34c1024d"
	doguuid3 = "537455a7-c2a9-488a-b671-672c27e47217"
)

// Use a real db for tests, better than nothing XD
type TestModel struct {
	models.BaseModel

	Name string `gorm:"column:real_name_column" json:"name"`
	Age  int    `json:"age"`

	Dogs []Dog `betterrest:"peg" json:"dogs"`
}

type Dog struct {
	models.BaseModel

	Name        string          `json:"name"`
	TestModelID *datatypes.UUID `gorm:"type:uuid;index;not null;" json:"_"`
}

var db *gorm.DB

// Package level setup
func TestMain(m *testing.M) {

	dsn := "host=" + host + " port=" + port + " user=" + username +
		" dbname=" + dbname + " password=" + password + " sslmode=disable"

	var err error
	db, err = gorm.Open("postgres", dsn)
	db.LogMode(true)
	if err != nil {
		panic("failed to connect database:" + err.Error())
	}

	if err := db.AutoMigrate(&TestModel{}).AutoMigrate(&Dog{}).Error; err != nil {
		panic("failed to automigrate TestModel:" + err.Error())
	}

	// Delete everything
	if err := db.Delete(&TestModel{}).Error; err != nil {
		panic("failed delete everything" + err.Error())
	}

	dog1 := Dog{
		BaseModel:   models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
		Name:        "Doggie1",
		TestModelID: datatypes.NewUUIDFromStringNoErr(uuid1),
	}

	dog2 := Dog{
		BaseModel:   models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid2)},
		Name:        "Doggie2",
		TestModelID: datatypes.NewUUIDFromStringNoErr(uuid1),
	}
	dog3 := Dog{
		BaseModel:   models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid3)},
		Name:        "Doggie3",
		TestModelID: datatypes.NewUUIDFromStringNoErr(uuid1),
	}

	tm1 := TestModel{BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)}, Name: "first", Age: 1}
	tm2 := TestModel{BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid2)}, Name: "second", Age: 3}
	tm3 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid3)},
		Name:      "same", Age: 3,
		Dogs: []Dog{dog1, dog2},
	}
	tm4 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid4)},
		Name:      "same", Age: 4, Dogs: []Dog{dog3},
	}

	if err := db.Create(&tm1).Create(&tm2).Create(&tm3).Create(&tm4).Error; err != nil {
		panic("something wrong with populating the db:" + err.Error())
	}

	log.Println("before run")

	exitVal := m.Run()

	log.Println("running delete")

	// Teardown
	if err := db.Unscoped().Delete(&tm1).Delete(&tm2).Delete(&tm3).Delete(&tm4).Delete(&dog1).Delete(&dog2).Error; err != nil {
		panic("something wrong with removing data from the db:" + err.Error())
	}

	os.Exit(exitVal)
}
