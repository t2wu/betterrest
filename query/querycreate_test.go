package query

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// Create and Delete

func TestCreate_PeggedArray(t *testing.T) {
	uuid := "046bcadb-7127-47b1-9c1e-ff92ccea44b8"
	tm := TestModel{BaseModel: models.BaseModel{
		ID: datatypes.NewUUIDFromStringNoErr(uuid)},
		Name: "MyTestModel",
		Age:  1,
		Dogs: []Dog{
			{
				Name:  "Buddy",
				Color: "black",
			},
		},
	}
	tx := db.Begin()
	defer tx.Rollback()

	err := Q(tx).Create(&tm).Error()
	if !assert.Nil(t, err) {
		return
	}

	searched := TestModel{}
	if err := Q(tx, C("ID =", uuid)).First(&searched).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	assert.Equal(t, uuid, searched.ID.String())
	if assert.Equal(t, 1, len(searched.Dogs)) {
		assert.Equal(t, "Buddy", searched.Dogs[0].Name)
		assert.Equal(t, "black", searched.Dogs[0].Color)
	}
}

func TestCreate_PegAssocArray_ShouldAssociateCorrectly(t *testing.T) {
	// First create a cat, and while creating TestModel, associate it with the cat
	// Then, when you load it, you should see the cat
	catuuid := "6a53ab29-72c9-4746-8e12-cb670d289231"
	cat := Cat{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(catuuid)},
		Name:      "Buddy",
		Color:     "black",
	}

	tx := db.Begin()
	defer tx.Rollback()

	err := Q(tx).Create(&cat).Error()
	if !assert.Nil(t, err) {
		return
	}

	uuid := "046bcadb-7127-47b1-9c1e-ff92ccea44b8"
	tm := TestModel{BaseModel: models.BaseModel{
		ID: datatypes.NewUUIDFromStringNoErr(uuid)},
		Name: "MyTestModel",
		Age:  1,
		Cats: []Cat{cat},
	}

	err = Q(tx).Create(&tm).Error()
	if !assert.Nil(t, err) {
		return
	}

	searched := TestModel{}
	if err := Q(tx, C("ID =", uuid)).First(&searched).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	assert.Equal(t, uuid, searched.ID.String())
	if assert.Equal(t, 1, len(searched.Cats)) { // should be associated
		assert.Equal(t, catuuid, searched.Cats[0].ID.String())
		assert.Equal(t, "Buddy", searched.Cats[0].Name)
		assert.Equal(t, "black", searched.Cats[0].Color)
	}
}

func TestCreate_PeggedStruct(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := "57403d17-01c7-40d2-ade3-6f8e8a27d786"
	doguuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"

	testModel1 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:      "TestModel1",
		FavoriteDog: Dog{
			BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
			Name:      "Buddy",
			Color:     "black",
		},
	}

	err := DB(tx).Create(&testModel1).Error()
	if !assert.Nil(t, err) {
		return
	}

	searched := TestModel{}
	err = Q(tx, C("ID =", uuid1)).First(&searched).Error()
	if !assert.Nil(t, err) {
		return
	}

	assert.Equal(t, uuid1, searched.ID.String())
	assert.Equal(t, doguuid1, searched.FavoriteDog.GetID().String())
}

func TestCreate_PeggedStructPtr(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := "57403d17-01c7-40d2-ade3-6f8e8a27d786"
	doguuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"

	testModel1 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:      "TestModel1",
		EvilDog: &Dog{
			BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
			Name:      "Buddy",
			Color:     "black",
		},
	}

	err := DB(tx).Create(&testModel1).Error()
	if !assert.Nil(t, err) {
		return
	}

	searched := TestModel{}
	err = Q(tx, C("ID =", uuid1)).First(&searched).Error()
	if !assert.Nil(t, err) {
		return
	}

	assert.Equal(t, uuid1, searched.ID.String())
	assert.Equal(t, doguuid1, searched.EvilDog.GetID().String())
}

func TestCreate_PeggedAssocStruct(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := "57403d17-01c7-40d2-ade3-6f8e8a27d786"
	catuuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"
	catuuid2 := "418d986b-72f7-462c-ab20-e7d5a2491b8c"

	cat := Cat{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(catuuid1)},
		Name:      "Kiddy",
		Color:     "black",
	}

	// Unrelated at shouldn't be affected
	cat2 := Cat{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(catuuid2)},
		Name:      "Kiddy",
		Color:     "black",
	}

	testModel1 := TestModel{
		BaseModel:   models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:        "TestModel1",
		FavoriteCat: cat,
	}

	err := DB(tx).Create(&cat).Create(&cat2).Create(&testModel1).Error()
	if !assert.Nil(t, err) {
		return
	}

	searched := TestModel{}
	err = Q(tx, C("ID =", uuid1)).First(&searched).Error()
	if !assert.Nil(t, err) {
		return
	}

	assert.Equal(t, uuid1, searched.ID.String())
	assert.Equal(t, catuuid1, searched.FavoriteCat.GetID().String())

	// Unrelated at shouldn't be affected
	othercat := Cat{}
	err = Q(tx, C("ID =", catuuid2)).First(&othercat).Error()
	if !assert.Nil(t, err) {
		return
	}
	assert.Equal(t, catuuid2, othercat.GetID().String())
	assert.Nil(t, othercat.TestModelID)
}

func TestCreate_PeggedAssocStructPtr(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := "57403d17-01c7-40d2-ade3-6f8e8a27d786"
	catuuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"
	catuuid2 := "418d986b-72f7-462c-ab20-e7d5a2491b8c"

	cat := Cat{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(catuuid1)},
		Name:      "Kiddy",
		Color:     "black",
	}

	// Unrelated at shouldn't be affected
	cat2 := Cat{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(catuuid2)},
		Name:      "Kiddy",
		Color:     "black",
	}

	testModel1 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:      "TestModel1",
		EvilCat:   &cat,
	}

	err := DB(tx).Create(&cat).Create(&cat2).Create(&testModel1).Error()
	if !assert.Nil(t, err) {
		return
	}

	searched := TestModel{}
	err = Q(tx, C("ID =", uuid1)).First(&searched).Error()
	if !assert.Nil(t, err) {
		return
	}

	assert.Equal(t, uuid1, searched.ID.String())
	if assert.NotNil(t, searched.EvilCat) {
		assert.Equal(t, catuuid1, searched.EvilCat.GetID().String())
	}

	// Unrelated at shouldn't be affected
	othercat := Cat{}
	err = Q(tx, C("ID =", catuuid2)).First(&othercat).Error()
	if !assert.Nil(t, err) {
		return
	}
	assert.Equal(t, catuuid2, othercat.GetID().String())
	assert.Nil(t, othercat.TestModelID)
}

func TestCreate_PeggedArray_WithExistingID_ShouldGiveAnError(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid := datatypes.NewUUID()
	doguuid := datatypes.NewUUID()
	tm1 := TestModel{BaseModel: models.BaseModel{
		ID: uuid},
		Name: "MyTestModel",
		Age:  1,
		Dogs: []Dog{
			{
				BaseModel: models.BaseModel{ID: doguuid},
				Name:      "Buddy",
				Color:     "black",
			},
		},
	}

	err := Q(tx).Create(&tm1).Error()
	if !assert.Nil(t, err) {
		return
	}

	log.Println("=============================tim")

	tm2 := TestModel{BaseModel: models.BaseModel{
		ID: uuid},
		Name: "MyTestModel",
		Age:  1,
		Dogs: []Dog{
			{
				BaseModel: models.BaseModel{ID: doguuid},
				Name:      "Buddy",
				Color:     "black",
			},
		},
	}

	err = Q(tx).Create(&tm2).Error()
	assert.Error(t, err)
}

func TestCreate_PeggedStruct_WithExistingID_ShouldGiveAnError(t *testing.T) {
	doguuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"

	testModel1 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUID()},
		Name:      "TestModel1",
		FavoriteDog: Dog{
			BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
			Name:      "Buddy",
			Color:     "black",
		},
	}

	tx := db.Begin()
	defer tx.Rollback()

	if err := Q(tx).Create(&testModel1).Error(); !assert.Nil(t, err) {
		return
	}

	// Same doguuid1, and that should give an error
	testModel2 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUID()},
		Name:      "TestModel2",
		FavoriteDog: Dog{
			BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
			Name:      "Buddy",
			Color:     "black",
		},
	}

	err := Q(tx).Create(&testModel2).Error()
	assert.Error(t, err)
}

func TestCreate_PeggedStructPtr_WithExistingID_ShouldGiveAnError(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	doguuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"

	testModel1 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUID()},
		Name:      "TestModel1",
		EvilDog: &Dog{
			BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
			Name:      "Buddy",
			Color:     "black",
		},
	}

	err := DB(tx).Create(&testModel1).Error()
	if !assert.Nil(t, err) {
		return
	}

	testModel2 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUID()},
		Name:      "TestModel2",
		EvilDog: &Dog{
			BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
			Name:      "Buddy",
			Color:     "black",
		},
	}

	err = DB(tx).Create(&testModel2).Error()
	assert.Error(t, err)
}

func TestBatchCreate_PeggedArray(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := "57403d17-01c7-40d2-ade3-6f8e8a27d786"
	uuid2 := "95a71d20-e508-41b0-a6ea-901f96c2e721"
	doguuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"
	doguuid2 := "673bd527-1af8-4f3b-b0d1-8158ee6f5e51"

	testModel1 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:      "TestModel1",
		Dogs: []Dog{
			{
				BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
				Name:      "Buddy",
				Color:     "black",
			},
		},
	}
	testModel2 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid2)},
		Name:      "TestModel2",
		Dogs: []Dog{
			{
				BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid2)},
				Name:      "Happy",
				Color:     "red",
			},
		},
	}

	models := []models.IModel{&testModel1, &testModel2}

	searched := make([]TestModel, 0)

	err := DB(tx).CreateMany(models).Error()
	if assert.Nil(t, err) {
		err := Q(tx, C("ID IN", []*datatypes.UUID{
			datatypes.NewUUIDFromStringNoErr(uuid1),
			datatypes.NewUUIDFromStringNoErr(uuid2),
		})).Find(&searched).Error()
		if assert.Nil(t, err) {
			assert.Len(t, searched, 2)
			if assert.Len(t, searched[0].Dogs, 1) {
				assert.Equal(t, "Happy", searched[0].Dogs[0].Name)
			}
			if assert.Len(t, searched[1].Dogs, 1) {
				assert.Equal(t, "Buddy", searched[1].Dogs[0].Name)
			}
		}
	}
}

func TestBatchCreate_PeggedArray_WithExistingID_ShouldGiveError(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := "57403d17-01c7-40d2-ade3-6f8e8a27d786"
	uuid2 := "95a71d20-e508-41b0-a6ea-901f96c2e721"
	doguuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"
	doguuid2 := "673bd527-1af8-4f3b-b0d1-8158ee6f5e51"

	testModel1 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:      "TestModel1",
		Dogs: []Dog{
			{
				BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
				Name:      "Buddy",
				Color:     "black",
			},
		},
	}
	testModel2 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid2)},
		Name:      "TestModel2",
		Dogs: []Dog{
			{
				BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid2)},
				Name:      "Happy",
				Color:     "red",
			},
		},
	}

	testModels := []models.IModel{&testModel1, &testModel2}

	err := DB(tx).CreateMany(testModels).Error()
	assert.Nil(t, err)

	testModel3 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:      "TestModel3",
		Dogs: []Dog{
			{
				BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
				Name:      "Buddy",
				Color:     "black",
			},
		},
	}
	testModel4 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid2)},
		Name:      "TestModel4",
		Dogs: []Dog{
			{
				BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid2)},
				Name:      "Happy",
				Color:     "red",
			},
		},
	}

	testModels = []models.IModel{&testModel3, &testModel4}
	err = DB(tx).CreateMany(testModels).Error()
	assert.Error(t, err)
}

func TestBatchCreate_PeggAssociateArray_shouldAssociateCorrectly(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := datatypes.NewUUID().String()
	uuid2 := datatypes.NewUUID().String()
	catuuid1 := datatypes.NewUUID().String()
	catuuid2 := datatypes.NewUUID().String()

	cat1 := Cat{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(catuuid1)},
		Name:      "Kiddy1",
		Color:     "black",
	}

	cat2 := Cat{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(catuuid2)},
		Name:      "Kiddy2",
		Color:     "black",
	}

	err := DB(tx).CreateMany([]models.IModel{&cat1, &cat2}).Error()
	if !assert.Nil(t, err) {
		return
	}

	testModel1 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:      "TestModel1",
		Cats:      []Cat{cat1},
	}
	testModel2 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid2)},
		Name:      "TestModel2",
		Cats:      []Cat{cat2},
	}

	models := []models.IModel{&testModel1, &testModel2}

	searched := make([]TestModel, 0)

	err = DB(tx).CreateMany(models).Error()
	if assert.Nil(t, err) {
		err := Q(tx, C("ID IN", []*datatypes.UUID{
			datatypes.NewUUIDFromStringNoErr(uuid1),
			datatypes.NewUUIDFromStringNoErr(uuid2),
		})).Find(&searched).Error()
		if assert.Nil(t, err) {
			assert.Len(t, searched, 2)
			if assert.Len(t, searched[0].Cats, 1) {
				assert.Equal(t, "Kiddy2", searched[0].Cats[0].Name)
			}
			if assert.Len(t, searched[1].Cats, 1) {
				assert.Equal(t, "Kiddy1", searched[1].Cats[0].Name)
			}
		}
	}
}
