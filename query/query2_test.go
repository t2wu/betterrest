package query

import (
	"errors"
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/models"
)

// Create and Delete

func TestCreate_Works_ForPegArray(t *testing.T) {
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

func TestDelete_PeggedArray_ShouldRemoveAllNestedFields(t *testing.T) {
	uuid := "046bcadb-7127-47b1-9c1e-ff92ccea44b8"
	doguuid := "faea91d5-f376-400e-ac93-0109886db336"
	tm := TestModel{BaseModel: models.BaseModel{
		ID: datatypes.NewUUIDFromStringNoErr(uuid)},
		Name: "MyTestModel",
		Age:  1,
		Dogs: []Dog{
			{
				BaseModel: models.BaseModel{
					ID: datatypes.NewUUIDFromStringNoErr(doguuid),
				},
				Name:  "Buddy",
				Color: "black",
			},
		},
	}

	tx := db.Begin()
	defer tx.Rollback()

	if err := Q(tx).Create(&tm).Error(); !assert.Nil(t, err) {
		return
	}

	// Test delete by itself
	if err := DB(tx).Delete(&tm).Error(); !assert.Nil(t, err) {
		return
	}

	loadedTestModel := TestModel{}
	err := Q(tx, C("ID =", uuid)).First(&loadedTestModel).Error()
	if assert.Error(t, err) {
		assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
	}

	loadedDogModel := Dog{}
	err = Q(tx, C("ID =", doguuid)).First(&loadedDogModel).Error()
	if assert.Error(t, err) {
		assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
	}
}

func TestDelete_PeggedAssoc_Should_Leave_it_intact(t *testing.T) {
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

	err = Q(tx).Create(&tm).Delete(&tm).Error()
	if !assert.Nil(t, err) {
		return
	}

	loadedTestModel := TestModel{}
	err = Q(tx, C("ID =", uuid)).First(&loadedTestModel).Error()
	if assert.Error(t, err) {
		assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
	}

	loadedCatModel := Cat{}
	err = Q(tx, C("ID =", catuuid)).First(&loadedCatModel).Error()
	if assert.Nil(t, err) {
		assert.Equal(t, catuuid, loadedCatModel.GetID().String())
		assert.Nil(t, loadedCatModel.TestModelID)
	}
}

func TestDelete_criteria_works(t *testing.T) {
	id1 := "046bcadb-7127-47b1-9c1e-ff92ccea44b8"
	id2 := "395857f2-8d15-4808-a45e-76eca2d07994"
	id3 := "2a8332b8-42a9-4115-8be7-55ba625fe574"
	tm1 := TestModel{BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(id1)}, Name: "MyTestModel", Age: 1}
	tm2 := TestModel{BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(id2)}, Name: "MyTestModel", Age: 1}
	tm3 := TestModel{BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(id3)}, Name: "MyTestModel", Age: 3}

	tx := db.Begin()
	defer tx.Rollback()

	err := Q(tx).Create(&tm1).Create(&tm2).Create(&tm3).Error()
	if !assert.Nil(t, err) {
		return
	}

	tms := make([]TestModel, 0)
	err = Q(tx, C("Age =", 3)).Find(&tms).Error()
	if !assert.Nil(t, err) {
		return
	}

	assert.Equal(t, 3, len(tms), "initial condition should be 3")

	err = Q(tx, C("Age =", 3).And("Name =", "MyTestModel")).Delete(&TestModel{}).Error()
	if !assert.Nil(t, err) {
		return
	}

	tms = make([]TestModel, 0)
	if err := Q(tx, C("Name =", "MyTestModel")).Find(&tms).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	if assert.Equal(t, 2, len(tms), "Should still have 2 left after one is deleted") {
		assert.Equal(t, id2, tms[0].ID.String())
		assert.Equal(t, id1, tms[1].ID.String())
	}

	tms = make([]TestModel, 0)
	err = Q(tx, C("Age =", 3).And("Name =", "same")).Find(&tms).Error()
	if assert.Nil(t, err) {
		return
	}

	assert.Equal(t, 1, len(tms), "The one in setup() should still be left intact")
}

func TestSave_Works(t *testing.T) {
	id2 := "d113ed09-cfc5-47a5-b35c-6f60c49cbd08"

	tx := db.Begin()
	defer tx.Rollback()

	tm := TestModel{}
	if err := Q(tx, C("ID =", id2)).First(&tm).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	// Change the name to something else
	tm.Name = "TestSave_Works"
	if err := Q(tx).Save(&tm).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	// Find it back to make sure it has been changed
	searched := TestModel{}
	if err := Q(tx, C("ID =", id2)).First(&searched).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	assert.Equal(t, "TestSave_Works", searched.Name)
}

func TestUpdate_Field_Works(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()
	if err := Q(tx, C("Name =", "second")).Update(&TestModel{}, C("Age =", 120)).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	check := TestModel{}
	if err := Q(tx, C("Name =", "second")).First(&check).Error(); err != nil {
		assert.Nil(t, err)
		return
	}

	assert.Equal(t, 120, check.Age)
}

func TestUpdate_NestedField_ShouldGiveWarning(t *testing.T) {
	// Name:        "Doggie2",
	if err := Q(db, C("Name =", "second")).Update(&TestModel{}, C("Dogs.Color =", "purple")).Error(); err != nil {
		assert.Equal(t, "dot notation in update", err.Error())
		return
	}

	assert.Fail(t, "should not be here")
}

// Batch create and delete operations

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

func TestBatchCreate_PeggAssociateArray_shouldAssociateCorrectly(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := "57403d17-01c7-40d2-ade3-6f8e8a27d786"
	uuid2 := "95a71d20-e508-41b0-a6ea-901f96c2e721"
	catuuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"
	catuuid2 := "673bd527-1af8-4f3b-b0d1-8158ee6f5e51"

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

func TestDelete_PeggedStruct_ShouldBeDeleted(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := "57403d17-01c7-40d2-ade3-6f8e8a27d786"
	doguuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"

	tm1 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:      "first",
		Age:       1,
		FavoriteDog: Dog{
			BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
			Name:      "Buddy",
			Color:     "black",
		},
	}

	if err := DB(tx).Create(&tm1).Error(); !assert.Nil(t, err) {
		return
	}

	if err := DB(tx).Delete(&tm1).Error(); !assert.Nil(t, err) {
		return
	}

	err := Q(tx, C("ID =", doguuid1)).Find(&Dog{}).Error()
	if assert.Error(t, err) {
		assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
	}
}

func TestDelete_PeggedStructPtr_ShouldBeDeleted(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := "57403d17-01c7-40d2-ade3-6f8e8a27d786"
	doguuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"

	tm1 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:      "first",
		Age:       1,
		EvilDog: &Dog{
			BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(doguuid1)},
			Name:      "Buddy",
			Color:     "black",
		},
	}

	if err := DB(tx).Create(&tm1).Error(); !assert.Nil(t, err) {
		return
	}

	if err := DB(tx).Delete(&tm1).Error(); !assert.Nil(t, err) {
		return
	}

	err := Q(tx, C("ID =", doguuid1)).First(&Dog{}).Error()
	if assert.Error(t, err) {
		assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
	}
}

func TestDelete_PeggedAssocStruct_ShouldLeftIntact(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := "57403d17-01c7-40d2-ade3-6f8e8a27d786"
	catuuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"

	cat := Cat{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(catuuid1)},
		Name:      "Buddy",
		Color:     "black",
	}

	if err := DB(tx).Create(&cat).Error(); !assert.Nil(t, err) {
		return
	}

	tm1 := TestModel{
		BaseModel:   models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:        "first",
		Age:         1,
		FavoriteCat: cat,
	}

	if err := DB(tx).Create(&tm1).Error(); !assert.Nil(t, err) {
		return
	}

	if err := DB(tx).Delete(&tm1).Error(); !assert.Nil(t, err) {
		return
	}

	searched := Cat{}
	err := Q(tx, C("ID =", catuuid1)).First(&searched).Error()
	assert.Nil(t, err)
	assert.Nil(t, searched.TestModelID)
}

func TestDelete_PeggedAssocStructPtr_ShouldLeftIntact(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := "57403d17-01c7-40d2-ade3-6f8e8a27d786"
	catuuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"

	cat := Cat{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(catuuid1)},
		Name:      "Buddy",
		Color:     "black",
	}

	if err := DB(tx).Create(&cat).Error(); !assert.Nil(t, err) {
		return
	}

	tm1 := TestModel{
		BaseModel: models.BaseModel{ID: datatypes.NewUUIDFromStringNoErr(uuid1)},
		Name:      "first",
		Age:       1,
		EvilCat:   &cat,
	}

	if err := DB(tx).Create(&tm1).Error(); !assert.Nil(t, err) {
		return
	}

	if err := DB(tx).Delete(&tm1).Error(); !assert.Nil(t, err) {
		return
	}

	searched := Cat{}
	err := Q(tx, C("ID =", catuuid1)).First(&searched).Error()
	assert.Nil(t, err)
	assert.Nil(t, searched.TestModelID)
}

func TestBatchDelete_PeggedArray(t *testing.T) {
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

	err := DB(tx).CreateMany(models).DeleteMany(models).Error()
	if assert.Nil(t, err) {
		searched := make([]TestModel, 0)
		err := Q(tx, C("ID IN", []*datatypes.UUID{
			datatypes.NewUUIDFromStringNoErr(uuid1),
			datatypes.NewUUIDFromStringNoErr(uuid2),
		})).Find(&searched).Error()
		assert.Nil(t, err)
		assert.Equal(t, 0, len(searched))
		assert.Len(t, searched, 0)

		dogSearched := make([]Dog, 0)
		err = Q(tx, C("ID IN", []*datatypes.UUID{
			datatypes.NewUUIDFromStringNoErr(doguuid1),
			datatypes.NewUUIDFromStringNoErr(doguuid2),
		})).Find(&dogSearched).Error()

		assert.Nil(t, err)
		assert.Equal(t, 0, len(dogSearched))
	}
}

func TestBatchDelete_PegAssocArray_ShouldLeaveItIntact(t *testing.T) {
	tx := db.Begin()
	defer tx.Rollback()

	uuid1 := "57403d17-01c7-40d2-ade3-6f8e8a27d786"
	uuid2 := "95a71d20-e508-41b0-a6ea-901f96c2e721"
	catuuid1 := "919b7d4b-35fd-43a9-b707-78a874870f16"
	catuuid2 := "673bd527-1af8-4f3b-b0d1-8158ee6f5e51"

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

	err = DB(tx).CreateMany(models).DeleteMany(models).Error()
	if assert.Nil(t, err) {
		searched := make([]TestModel, 0)
		err := Q(tx, C("ID IN", []*datatypes.UUID{
			datatypes.NewUUIDFromStringNoErr(uuid1),
			datatypes.NewUUIDFromStringNoErr(uuid2),
		})).Find(&searched).Error()
		assert.Nil(t, err)
		assert.Equal(t, 0, len(searched))
		assert.Len(t, searched, 0)

		catSearched := make([]Cat, 0)
		err = Q(tx, C("ID IN", []*datatypes.UUID{
			datatypes.NewUUIDFromStringNoErr(catuuid1),
			datatypes.NewUUIDFromStringNoErr(catuuid2),
		})).Find(&catSearched).Error()

		assert.Nil(t, err)
		if assert.Equal(t, 2, len(catSearched)) {
			assert.Nil(t, catSearched[0].TestModelID)
			assert.Nil(t, catSearched[1].TestModelID)
		}
	}
}
