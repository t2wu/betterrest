package lifecycle

import (
	"log"
	"strings"

	"github.com/go-chi/render"
	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/controller"
	"github.com/t2wu/betterrest/datamapper"
	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/utils/transact"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
)

// ------------------------------------------------------
// func LogTransID(tx *gorm.DB, method, url, cardinality string) {
// 	if settings.Log {
// 		res := struct {
// 			TxidCurrent int
// 		}{}
// 		if err := tx.Raw("SELECT txid_current()").Scan(&res).Error; err != nil {
// 			s := fmt.Sprintf("[BetterREST]: Error in HTTP method: %s, Endpoint: %s, cardinality: %s", method, url, cardinality)
// 			log.Println(s)
// 			// ignore error
// 			return
// 		}

// 		s := fmt.Sprintf("[BetterREST]: %s %s (%s), transact: %d", method, url, cardinality, res.TxidCurrent)
// 		log.Println(s)
// 	}
// }

type Logger interface {
	Log(tx *gorm.DB, method, url, cardinality string)
}

func callOldBatchTransact(data *controller.Data, info *controller.EndPointInfo) {
	oldBatchCargo := models.BatchHookCargo{Payload: data.Cargo.Payload}
	bhpData := models.BatchHookPointData{Ms: data.Ms, DB: nil, Who: data.Who,
		TypeString: data.TypeString, Roles: data.Roles, URLParams: data.URLParams, Cargo: &oldBatchCargo}

	var op models.CRUPDOp
	switch info.Op {
	case controller.RESTOpRead:
		op = models.CRUPDOpRead
	case controller.RESTOpCreate:
		op = models.CRUPDOpCreate
	case controller.RESTOpUpdate:
		op = models.CRUPDOpUpdate
	case controller.RESTOpPatch:
		op = models.CRUPDOpPatch
	case controller.RESTOpDelete:
		op = models.CRUPDOpDelete
	}

	// the batch afterTransact hookpoint
	if afterTransact := models.ModelRegistry[data.TypeString].AfterTransact; afterTransact != nil {
		afterTransact(bhpData, op)
	}

	data.Cargo.Payload = oldBatchCargo.Payload
}

func callOldOneTransact(data *controller.Data, info *controller.EndPointInfo) {
	oldSingleCargo := models.ModelCargo{Payload: data.Cargo.Payload}
	hpdata := models.HookPointData{DB: nil, Who: data.Who, TypeString: data.TypeString,
		URLParams: data.URLParams, Role: &data.Roles[0], Cargo: &oldSingleCargo}

	var op models.CRUPDOp
	switch info.Op {
	case controller.RESTOpRead:
		op = models.CRUPDOpRead
	case controller.RESTOpCreate:
		op = models.CRUPDOpCreate
	case controller.RESTOpUpdate:
		op = models.CRUPDOpUpdate
	case controller.RESTOpPatch:
		op = models.CRUPDOpPatch
	case controller.RESTOpDelete:
		op = models.CRUPDOpDelete
	}

	// the single afterTransact hookpoint
	if v, ok := data.Ms[0].(models.IAfterTransact); ok {
		v.AfterTransact(hpdata, op)
	}
	data.Cargo.Payload = hpdata.Cargo.Payload
}

func CreateMany(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, modelObjs []models.IModel, options map[urlparam.Param]interface{}, logger Logger) (*controller.Data, *controller.EndPointInfo, render.Renderer) {
	cargo := controller.Cargo{}

	retval := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retval *webrender.RetVal) {
		if logger != nil {
			logger.Log(tx, "POST", strings.ToLower(typeString), "n")
		}

		if modelObjs, retval = mapper.CreateMany(tx, who, typeString, modelObjs, options, &cargo); retval != nil {
			log.Printf("Error in lifecycle.CreateMany: %s, %+v\n", typeString, retval)
			return retval
		}
		return nil
	}, "lifecycle.CreateMany")

	if retval != nil {
		if retval.Error != nil {
			return nil, nil, webrender.NewErrCreate(retval.Error)
		} else {
			return nil, nil, retval.CustomRenderer
		}
	}

	roles := make([]models.UserRole, len(modelObjs))
	// admin is 0 so it's ok
	for i := 0; i < len(modelObjs); i++ {
		roles[i] = models.UserRoleAdmin
	}

	data := controller.Data{Ms: modelObjs, DB: nil, Who: who,
		TypeString: typeString, Roles: roles, URLParams: options, Cargo: &cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpCreate,
		Cardinality: controller.APICardinalityMany,
	}

	// Handes transaction
	if models.ModelRegistry[typeString].Controller == nil {
		// It's possible that the user has no controller, even if it's new code
		callOldBatchTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	if ctrl, ok := models.ModelRegistry[data.TypeString].Controller.(controller.IAfterTransact); ok {
		ctrl.AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func CreateOne(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, modelObj models.IModel, options map[urlparam.Param]interface{}, logger Logger) (*controller.Data, *controller.EndPointInfo, render.Renderer) {
	cargo := controller.Cargo{}
	retval := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retval *webrender.RetVal) {
		if logger != nil {
			logger.Log(tx, "POST", strings.ToLower(typeString), "1")
		}

		if modelObj, retval = mapper.CreateOne(tx, who, typeString, modelObj, options, &cargo); retval != nil {
			log.Printf("Error in lifecycle.CreateOne: %s, %+v\n", typeString, retval)
			return retval
		}
		return nil
	}, "lifecycle.CreateOne")

	if retval != nil {
		if retval.Error != nil {
			return nil, nil, webrender.NewErrCreate(retval.Error)
		} else {
			return nil, nil, *retval.CustomRenderer
		}
	}

	roles := []models.UserRole{models.UserRoleAdmin} // just one item
	ms := []models.IModel{modelObj}

	data := controller.Data{Ms: ms, DB: nil, Who: who,
		TypeString: typeString, Roles: roles, URLParams: options, Cargo: &cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpCreate,
		Cardinality: controller.APICardinalityMany,
	}

	// Handes transaction
	if models.ModelRegistry[typeString].Controller == nil {
		// It's possible that the user has no controller, even if it's new code
		callOldOneTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	if ctrl, ok := models.ModelRegistry[data.TypeString].Controller.(controller.IAfterTransact); ok {
		ctrl.AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

// ReadMany
func ReadMany(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, options map[urlparam.Param]interface{}, logger Logger) (*controller.Data, *controller.EndPointInfo, *int, render.Renderer) {
	if logger != nil {
		logger.Log(nil, "GET", strings.ToLower(typeString), "n")
	}

	cargo := controller.Cargo{}
	modelObjs, roles, no, retval := mapper.ReadMany(db.Shared(), who, typeString, options, &cargo)
	if retval != nil {
		if retval.Error != nil {
			return nil, nil, no, webrender.NewErrInternalServerError(retval.Error) // TODO, probably should have a READ error
		} else {
			return nil, nil, no, *retval.CustomRenderer
		}
	}

	data := controller.Data{Ms: modelObjs, DB: nil, Who: who,
		TypeString: typeString, Roles: roles, URLParams: options, Cargo: &cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpRead,
		Cardinality: controller.APICardinalityMany,
	}

	// Handes transaction
	if models.ModelRegistry[typeString].Controller == nil {
		// It's possible that the user has no controller, even if it's new code
		callOldBatchTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, no, nil
	}

	if ctrl, ok := models.ModelRegistry[data.TypeString].Controller.(controller.IAfterTransact); ok {
		ctrl.AfterTransact(&data, &info)
	}

	return &data, &info, no, nil
}

func ReadOne(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, id *datatypes.UUID, options map[urlparam.Param]interface{}, logger Logger) (*controller.Data, *controller.EndPointInfo, render.Renderer) {
	if logger != nil {
		logger.Log(nil, "GET", strings.ToLower(typeString), "1")
	}

	cargo := controller.Cargo{}
	modelObj, role, retval := mapper.ReadOne(db.Shared(), who, typeString, id, options, &cargo)
	if retval != nil {
		if retval.Error != nil {
			if gorm.IsRecordNotFoundError(retval.Error) {
				return nil, nil, webrender.NewErrNotFound(retval.Error)
			}
			return nil, nil, webrender.NewErrInternalServerError(retval.Error) // TODO, probably should have a READ error
		} else {
			return nil, nil, retval.CustomRenderer
		}
	}

	data := controller.Data{Ms: []models.IModel{modelObj}, DB: nil, Who: who,
		TypeString: typeString, Roles: []models.UserRole{role}, URLParams: options, Cargo: &cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpRead,
		Cardinality: controller.APICardinalityOne,
	}

	if models.ModelRegistry[typeString].Controller == nil {
		callOldOneTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	if ctrl, ok := models.ModelRegistry[data.TypeString].Controller.(controller.IAfterTransact); ok {
		ctrl.AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func UpdateMany(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, modelObjs []models.IModel, options map[urlparam.Param]interface{}, logger Logger) (*controller.Data, *controller.EndPointInfo, render.Renderer) {
	cargo := controller.Cargo{}
	retval := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retval *webrender.RetVal) {
		if logger != nil {
			logger.Log(tx, "PUT", strings.ToLower(typeString), "n")
		}

		if modelObjs, retval = mapper.UpdateMany(tx, who, typeString, modelObjs, options, &cargo); retval != nil {
			log.Printf("Error in lifecycle.UpdateMany: %s, %+v\n", typeString, retval)
			return retval
		}

		return nil
	}, "lifecycle.UpdateMany")

	if retval != nil {
		if retval.Error != nil {
			return nil, nil, webrender.NewErrUpdate(retval.Error)
		} else {
			return nil, nil, retval.CustomRenderer
		}
	}

	roles := make([]models.UserRole, len(modelObjs))
	for i := 0; i < len(roles); i++ {
		roles[i] = models.UserRoleAdmin
	}

	data := controller.Data{Ms: modelObjs, DB: nil, Who: who,
		TypeString: typeString, Roles: roles, URLParams: options, Cargo: &cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpUpdate,
		Cardinality: controller.APICardinalityMany,
	}

	if models.ModelRegistry[typeString].Controller == nil {
		callOldBatchTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	if ctrl, ok := models.ModelRegistry[data.TypeString].Controller.(controller.IAfterTransact); ok {
		ctrl.AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func UpdateOne(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, modelObj models.IModel, id *datatypes.UUID, options map[urlparam.Param]interface{}, logger Logger) (*controller.Data, *controller.EndPointInfo, render.Renderer) {
	cargo := controller.Cargo{}
	retval := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retval *webrender.RetVal) {
		if logger != nil {
			logger.Log(tx, "PUT", strings.ToLower(typeString), "1")
		}

		if modelObj, retval = mapper.UpdateOne(tx, who, typeString, modelObj, id, options, &cargo); retval != nil {
			log.Printf("Error in lifecycle.UpdateOne: %s, %+v\n", typeString, retval)
			return retval
		}
		return nil
	}, "lifecycle.UpdateOne")

	if retval != nil {
		if retval.Error != nil {
			return nil, nil, webrender.NewErrUpdate(retval.Error)
		} else {
			return nil, nil, retval.CustomRenderer
		}
	}

	role := models.UserRoleAdmin
	data := controller.Data{Ms: []models.IModel{modelObj}, DB: nil, Who: who,
		TypeString: typeString, Roles: []models.UserRole{role}, URLParams: options, Cargo: &cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpUpdate,
		Cardinality: controller.APICardinalityOne,
	}

	if models.ModelRegistry[typeString].Controller == nil {
		callOldOneTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	if ctrl, ok := models.ModelRegistry[data.TypeString].Controller.(controller.IAfterTransact); ok {
		ctrl.AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func PatchMany(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, jsonIDPatches []models.JSONIDPatch, options map[urlparam.Param]interface{}, logger Logger) (*controller.Data, *controller.EndPointInfo, render.Renderer) {
	var modelObjs []models.IModel
	cargo := controller.Cargo{}
	retval := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retval *webrender.RetVal) {
		if logger != nil {
			logger.Log(tx, "PATCH", strings.ToLower(typeString), "n")
		}

		if modelObjs, retval = mapper.PatchMany(tx, who, typeString, jsonIDPatches, options, &cargo); retval != nil {
			log.Printf("Error in lifecycle.PatchMany: %s, %+v\n", typeString, retval)
			return retval
		}
		return nil
	}, "lifecycle.PatchMany")

	if retval != nil {
		if retval.Error != nil {
			return nil, nil, webrender.NewErrPatch(retval.Error)
		} else {
			return nil, nil, retval.CustomRenderer
		}
	}

	roles := make([]models.UserRole, len(modelObjs))
	for i := 0; i < len(roles); i++ {
		roles[i] = models.UserRoleAdmin
	}

	data := controller.Data{Ms: modelObjs, DB: nil, Who: who,
		TypeString: typeString, Roles: roles, URLParams: options, Cargo: &cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpPatch,
		Cardinality: controller.APICardinalityMany,
	}

	if models.ModelRegistry[typeString].Controller == nil {
		callOldBatchTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	if ctrl, ok := models.ModelRegistry[data.TypeString].Controller.(controller.IAfterTransact); ok {
		ctrl.AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func PatchOne(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, jsonPatch []byte, id *datatypes.UUID, options map[urlparam.Param]interface{}, logger Logger) (*controller.Data, *controller.EndPointInfo, render.Renderer) {
	cargo := controller.Cargo{}
	var modelObj models.IModel
	retval := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retval *webrender.RetVal) {
		if logger != nil {
			logger.Log(tx, "PATCH", strings.ToLower(typeString), "1")
		}

		if modelObj, retval = mapper.PatchOne(tx, who, typeString, jsonPatch, id, options, &cargo); retval != nil {
			log.Printf("Error in lifecycle.PatchOne: %s, %+v\n", typeString, retval)
			return retval
		}

		return nil
	}, "lifecycle.PatchOne")

	if retval != nil {
		if retval.Error != nil {
			return nil, nil, webrender.NewErrPatch(retval.Error)
		} else {
			return nil, nil, retval.CustomRenderer
		}
	}

	role := models.UserRoleAdmin
	data := controller.Data{Ms: []models.IModel{modelObj}, DB: nil, Who: who,
		TypeString: typeString, Roles: []models.UserRole{role}, URLParams: options, Cargo: &cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpPatch,
		Cardinality: controller.APICardinalityOne,
	}

	if models.ModelRegistry[typeString].Controller == nil {
		callOldOneTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	if ctrl, ok := models.ModelRegistry[data.TypeString].Controller.(controller.IAfterTransact); ok {
		ctrl.AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func DeleteMany(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, modelObjs []models.IModel, options map[urlparam.Param]interface{}, logger Logger) (*controller.Data, *controller.EndPointInfo, render.Renderer) {
	cargo := controller.Cargo{}
	retval := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retval *webrender.RetVal) {
		if logger != nil {
			logger.Log(tx, "DELETE", strings.ToLower(typeString), "n")
		}

		if modelObjs, retval = mapper.DeleteMany(tx, who, typeString, modelObjs, nil, &cargo); retval != nil {
			log.Printf("Error in lifecycle.DeleteMany: %s, %+v\n", typeString, retval)
			return retval
		}
		return nil
	}, "lifecycle.DeleteMany")

	if retval != nil {
		if retval.Error != nil {
			return nil, nil, webrender.NewErrDelete(retval.Error)
		} else {
			return nil, nil, retval.CustomRenderer
		}
	}

	roles := make([]models.UserRole, len(modelObjs))
	for i := 0; i < len(roles); i++ {
		roles[i] = models.UserRoleAdmin
	}

	data := controller.Data{Ms: modelObjs, DB: nil, Who: who,
		TypeString: typeString, Roles: roles, URLParams: options, Cargo: &cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpDelete,
		Cardinality: controller.APICardinalityMany,
	}

	if models.ModelRegistry[typeString].Controller == nil {
		callOldBatchTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	if ctrl, ok := models.ModelRegistry[data.TypeString].Controller.(controller.IAfterTransact); ok {
		ctrl.AfterTransact(&data, &info)
	}

	return &data, &info, nil
}

func DeleteOne(mapper datamapper.IDataMapper, who models.UserIDFetchable, typeString string, id *datatypes.UUID, options map[urlparam.Param]interface{}, logger Logger) (*controller.Data, *controller.EndPointInfo, render.Renderer) {
	cargo := controller.Cargo{}
	var modelObj models.IModel

	retval := transact.TransactCustomError(db.Shared(), func(tx *gorm.DB) (retval *webrender.RetVal) {
		logger.Log(tx, "DELETE", strings.ToLower(typeString), "1")

		if modelObj, retval = mapper.DeleteOne(tx, who, typeString, id, options, &cargo); retval != nil {
			log.Printf("Error in lifecycle.DeleteOne: %s %+v\n", typeString, retval)
			return retval
		}
		return
	}, "lifecycle.DeleteOne")

	if retval != nil {
		if retval.Error != nil {
			return nil, nil, webrender.NewErrDelete(retval.Error)
		} else {
			return nil, nil, retval.CustomRenderer
		}
	}

	role := models.UserRoleAdmin
	data := controller.Data{Ms: []models.IModel{modelObj}, DB: nil, Who: who,
		TypeString: typeString, Roles: []models.UserRole{role}, URLParams: options, Cargo: &cargo}
	info := controller.EndPointInfo{
		Op:          controller.RESTOpDelete,
		Cardinality: controller.APICardinalityOne,
	}

	if models.ModelRegistry[typeString].Controller == nil {
		callOldOneTransact(&data, &info) // for backward compatibility, for now
		return &data, &info, nil
	}

	if ctrl, ok := models.ModelRegistry[data.TypeString].Controller.(controller.IAfterTransact); ok {
		ctrl.AfterTransact(&data, &info)
	}

	return &data, &info, nil
}
