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
	uuid5 = "ec2e60a2-ed23-4121-b1b4-133c2d7ca167"

	doguuid0 = "ba023242-asbb-4c02-b134-b32873fabef3"
	doguuid1 = "a048824f-8728-4c0a-b091-ed8d59390542"
	doguuid2 = "5d95fae9-8ca4-4a81-bf15-0e5106ac6aa2"
	doguuid3 = "537455a7-c2a9-488a-b671-672c27e47217"
	doguuid4 = "82df2ace-19c2-4e78-a1c9-b75697fc813f"

	dogtoyuuid1 = "db79acab-5db8-4527-a34f-c494aa4e7a6c"
	dogtoyuuid2 = "a4b295a5-b156-447c-a065-3acd6f198dfe"
)

// Use a real db for tests, better than nothing XD
type TestModel struct {
	models.BaseModel

	Name string `gorm:"column:real_name_column" json:"name"`
	Age  int    `json:"age"`

	Dogs []Dog `betterrest:"peg" json:"dogs"`

	// Any field with pegassoc should have association_autoupdate:false AND
	// foreign key constraint for cat should have SET NULL on delete and update
	Cats []Cat `gorm:"association_autoupdate:false;" betterrest:"pegassoc" json:"cats"`
}

type Cat struct {
	models.BaseModel

	Name  string `json:"name"`
	Color string `json:"color"`

	// Cat should set TestModelID it as a foriegn key (gorm 1 foreign key was weird, at least
	// on MySQL. We should do Automigrate by really automigrating and taking into the tag
	// as well, esp if Gorm 2 isn't doing it yet)
	TestModelID *datatypes.UUID `gorm:"type:uuid;index;" json:"_"`
}

type Dog struct {
	models.BaseModel

	Name    string   `json:"name"`
	Color   string   `json:"color"`
	DogToys []DogToy `json:"dogToy"`

	TestModelID *datatypes.UUID `gorm:"type:uuid;index;not null;" json:"_"`
}

type DogToy struct {
	models.BaseModel

	ToyName string `json:"toyName"`

	DogID *datatypes.UUID `gorm:"type:uuid;index;not null;" json:"_"`
}

type UnNested struct {
	models.BaseModel

	Name          string `json:"name"`
	UnNestedInner UnNestedInner

	TestModelID *datatypes.UUID `gorm:"type:uuid;index;not null;" json:"-"`
}

type UnNestedInner struct {
	models.BaseModel

	Name string `json:"name"`

	UnNestedID *datatypes.UUID `gorm:"type:uuid;index;not null;" json:"-"`
}

var db *gorm.DB

// Package level setup
func TestMain(m *testing.M) {

	dsn := "host=" + host + " port=" + port + " user=" + username +
		" dbname=" + dbname + " password=" + password + " sslmode=disable"

	var err error
	db, err = gorm.Open("postgres", dsn)
	if err != nil {
		panic("failed to connect database:" + err.Error())
	}
	db.SingularTable(true)

	if err := db.AutoMigrate(&TestModel{}).AutoMigrate(&Dog{}).
		AutoMigrate(&DogToy{}).AutoMigrate(&UnNested{}).AutoMigrate(&UnNestedInner{}).
		AutoMigrate(&Cat{}).AddForeignKey("test_model_id", "test_model(id)", "SET NULL", "SET NULL").Error; err != nil {
		panic("failed to automigrate TestModel:" + err.Error())
	}

	// Both dog toys are under green Dogs
	// log.Println("datatypes.NewUUIDFromStringNoErr(dogtoyuuid1):", datatypes.NewUUIDFromStringNoErr(dogtoyuuid1))
	// log.Println("datatypes.NewUUIDFromStringNoErr(doguuid2):", datatypes.NewUUIDFromStringNoErr(doguuid2))
	dogToy1 := DogToy{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(dogtoyuuid1)},
		ToyName:   "DogToySameName",
		DogID:     datatypes.NewUUIDFromStringNoErr(doguuid2),
	}
	dogToy2 := DogToy{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(dogtoyuuid2)},
		ToyName:   "DogToySameName",
		DogID:     datatypes.NewUUIDFromStringNoErr(doguuid4),
	}

	dog0 := Dog{
		BaseModel:   models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid0)},
		Name:        "Doggie0",
		Color:       "purple",
		TestModelID: datatypes.NewUUIDFromStringNoErr(uuid1),
	}
	dog1 := Dog{
		BaseModel:   models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
		Name:        "Doggie1",
		Color:       "red",
		TestModelID: datatypes.NewUUIDFromStringNoErr(uuid1),
	}
	dog2 := Dog{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid2)},
		Name:      "Doggie2",
		Color:     "green",
		// DogToys:     []DogToy{dogToy1},
		TestModelID: datatypes.NewUUIDFromStringNoErr(uuid1),
	}
	dog3 := Dog{
		BaseModel:   models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid3)},
		Name:        "Doggie3",
		Color:       "blue",
		TestModelID: datatypes.NewUUIDFromStringNoErr(uuid1),
	}
	dog4 := Dog{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid4)},
		Name:      "Doggie4",
		Color:     "green",
		// DogToys:     []DogToy{dogToy2},
		TestModelID: datatypes.NewUUIDFromStringNoErr(uuid5),
	}

	tm1 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:      "first",
		Age:       1,
		Dogs:      []Dog{dog0},
	}
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
	tm5 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid5)},
		Name:      "same", Age: 4, Dogs: []Dog{dog4},
	}

	unnesteduuid1 := "7192f73d-e56f-4a33-a7fb-eb9d605bc731"
	unnesteduuid1inner := "2174e7ce-708d-4b46-a1b4-59c41304b46"
	unnested1 := UnNested{
		BaseModel: models.BaseModel{
			ID: datatypes.NewUUIDFromStringNoErr(unnesteduuid1),
		},
		UnNestedInner: UnNestedInner{
			BaseModel: models.BaseModel{
				ID: datatypes.NewUUIDFromStringNoErr(unnesteduuid1inner),
			},
			Name:       "UnNestedInnerSameNameWith1&2",
			UnNestedID: datatypes.NewUUIDFromStringNoErr(unnesteduuid1),
		},
		Name:        "unnested1",
		TestModelID: datatypes.NewUUIDFromStringNoErr(uuid1),
	}
	unnesteduuid2 := "6cdb2b20-b6c6-4f8f-9c2f-632888887865"
	unnesteduuid2inner := "3441e7ce-708d-4b46-a1b4-59c41300cd48"
	unnested2 := UnNested{
		BaseModel: models.BaseModel{
			ID: datatypes.NewUUIDFromStringNoErr(unnesteduuid2),
		},
		UnNestedInner: UnNestedInner{
			BaseModel: models.BaseModel{
				ID: datatypes.NewUUIDFromStringNoErr(unnesteduuid2inner),
			},
			Name:       "UnNestedInnerSameNameWith1&2",
			UnNestedID: datatypes.NewUUIDFromStringNoErr(unnesteduuid2),
		},
		Name:        "unnested2",
		TestModelID: datatypes.NewUUIDFromStringNoErr(uuid2),
	}
	unnesteduuid3 := "e2bf6b2a-127c-491b-b6a2-49d88d217425"
	unnested3 := UnNested{
		BaseModel: models.BaseModel{
			ID: datatypes.NewUUIDFromStringNoErr(unnesteduuid3),
		},
		Name:        "unnested3",
		TestModelID: datatypes.NewUUIDFromStringNoErr(uuid2),
	}

	log.Println(tm1, tm2, tm3, tm4, tm5, unnested1, unnested2, dogToy1, dogToy2)
	db.LogMode(false)
	if err := db.Create(&tm1).Error; err != nil {
		panic("something wrong with populating the db:" + err.Error())
	}
	if err := db.Create(&tm2).Create(&tm3).Error; err != nil {
		panic("something wrong with populating the db:" + err.Error())
	}
	if err := db.Create(&tm4).Create(&tm5).Error; err != nil {
		panic("something wrong with populating the db:" + err.Error())
	}
	if err := db.Create(&unnested1).Create(&unnested2).Create(&unnested3).Error; err != nil {
		panic("something wrong with populating the db:" + err.Error())
	}
	if err := db.Create(&dogToy1).Create(&dogToy2).Error; err != nil {
		panic("something wrong with populating the db:" + err.Error())
	}

	db.LogMode(true)

	exitVal := m.Run()

	db.LogMode(false)

	// Teardown
	if err := db.Unscoped().Delete(&TestModel{}).
		Delete(&Dog{}).Delete(&DogToy{}).Delete(&UnNested{}).Delete(&UnNestedInner{}).Error; err != nil {
		panic("something wrong with removing data from the db:" + err.Error())
	}

	os.Exit(exitVal)
}
