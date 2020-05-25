package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/t2wu/betterrest/datamapper"
	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/libs/security"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/models/tools"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
	"github.com/jinzhu/gorm"
)

// ---------------------------------------------
func limitAndOffsetFromQueryString(values *url.Values) (int, int, error) {
	defer delete(*values, "offset")
	defer delete(*values, "limit")

	var o, l int
	var err error

	offset := values.Get("offset")
	limit := values.Get("limit")

	if offset == "" && limit == "" {
		return 0, 0, nil
	}

	if offset == "" {
		return 0, 0, errors.New("limit should be used with offset")
	} else {
		if o, err = strconv.Atoi(offset); err != nil {
			return 0, 0, err
		}
	}

	if limit == "" {
		return 0, 0, errors.New("offset should be used with limit")
	} else {
		if l, err = strconv.Atoi(limit); err != nil {
			return 0, 0, err
		}
	}

	return o, l, nil // It's ok to pass 0 limit, it'll be interpreted as an all.
}

func orderFromQueryString(values *url.Values) string {
	defer delete(*values, "order")

	if order := values.Get("order"); order != "" {
		// Prevent sql injection
		if order != "desc" && order != "asc" {
			return ""
		}
		return order
	}
	return ""
}

func createdTimeRangeFromQueryString(values *url.Values) (int, int, error) {
	defer delete(*values, "cstart")
	defer delete(*values, "cstop")

	if cstart, cstop := values.Get("cstart"), values.Get("cstop"); cstart != "" && cstop != "" {
		var err error
		cStartInt, cStopInt := 0, 0
		if cstart != "" {
			if cStartInt, err = strconv.Atoi(cstart); err != nil {
				return 0, 0, err
			}
		} else {
			cStartInt = 0
		}

		if cstop != "" {
			if cStopInt, err = strconv.Atoi(cstop); err != nil {
				return 0, 0, err
			}
		} else {
			cStopInt = int(time.Now().Unix()) // now
		}

		return cStartInt, cStopInt, nil
	}
	return 0, 0, nil
}

func modelObjsToJSON(typeString string, modelObjs []models.IModel, roles []models.UserRole) (string, error) {

	arr := make([]string, len(modelObjs))
	for i, v := range modelObjs {
		if j, err := tools.ToJSON(typeString, v, roles[i]); err != nil {
			return "", err
		} else {
			arr[i] = string(j)
		}
	}

	content := "[" + strings.Join(arr, ",") + "]"
	return content, nil
}

func renderModel(w http.ResponseWriter, r *http.Request, typeString string, modelObj models.IModel, role models.UserRole) {
	// render.JSON(w, r, modelObj) // cannot use this since no picking the field we need
	jsonBytes, err := tools.ToJSON(typeString, modelObj, role)
	if err != nil {
		log.Println("Error in renderModel:", err)
		render.Render(w, r, NewErrGenJSON(err))
		return
	}

	content := fmt.Sprintf("{ \"code\": 0, \"content\": %s }", string(jsonBytes))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(content))
}

func renderModelSlice(w http.ResponseWriter, r *http.Request, typeString string, modelObjs []models.IModel, roles []models.UserRole) {
	jsonString, err := modelObjsToJSON(typeString, modelObjs, roles)
	if err != nil {
		log.Println("Error in renderModelSlice:", err)
		render.Render(w, r, NewErrGenJSON(err))
		return
	}

	content := fmt.Sprintf("{ \"code\": 0, \"content\": %s }", jsonString)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(content))
}

// ---------------------------------------------

// getVerifiedAuthUser authenticates the user
// getVerifiedAuthUser authenticates the user
func getVerifiedAuthUser(userModel models.IModel) (models.IModel, bool) {
	userModel2 := reflect.New(models.UserTyp).Interface().(models.IModel)

	// TODO: maybe email is not the login, make it more flexible?
	email := reflect.ValueOf(userModel).Elem().FieldByName(("Email")).Interface().(string)
	password := reflect.ValueOf(userModel).Elem().FieldByName(("Password")).Interface().(string)

	err := db.Shared().Where("email = ?", email).First(userModel2).Error
	if gorm.IsRecordNotFoundError(err) {
		return nil, false // User doesn't exists with this email
	} else if err != nil {
		// Some other unknown error
		return nil, false
	}

	passwordHash := reflect.ValueOf(userModel2).Elem().FieldByName("PasswordHash").Interface().(string)
	if !security.IsSamePassword(password, passwordHash) {
		// Password doesn't match
		return nil, false
	}

	return userModel2, true
}

// ---------------------------------------------

// UserLoginHandler logs in the user. Effectively creates a JWT token for the user
func UserLoginHandler() func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request
		log.Println("UserLoginHandler")

		m, httperr := ModelFromJSONBody(r, "users") // m is models.IModel
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		authUser, authorized := getVerifiedAuthUser(m)
		if !authorized {
			// unable to login user. maybe doesn't exist?
			// or username, password wrong
			render.Render(w, r, NewErrLoginUser(nil))
			return
		}

		// login success, return access token
		scope := "owner"
		payload, err := createTokenPayloadForScope(authUser.GetID(), &scope)
		if err != nil {
			render.Render(w, r, NewErrGeneratingToken(err))
			return
		}

		var jsn []byte
		if jsn, err = json.Marshal(payload); err != nil {
			render.Render(w, r, NewErrGenJSON(err))
			return
		}

		w.Write(jsn)
	}
}

// ---------------------------------------------
// reflection stuff
// https://stackoverflow.com/questions/7850140/how-do-you-create-a-new-instance-of-a-struct-from-its-type-at-run-time-in-go
// https://stackoverflow.com/questions/23030884/is-there-a-way-to-create-an-instance-of-a-struct-from-a-string

// ReadAllHandler returns a Gin handler which fetch multiple records of a resource
func ReadAllHandler(typeString string, mapper datamapper.IGetAllMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		options := make(map[string]interface{})
		var err error
		var modelObjs []models.IModel

		values := r.URL.Query()
		if o, l, err := limitAndOffsetFromQueryString(&values); err == nil {
			options["offset"], options["limit"] = o, l
		} else if err != nil {
			render.Render(w, r, NewErrQueryParameter(err))
			return
		}

		options["order"] = orderFromQueryString(&values)
		options["better_otherqueries"] = values

		if cstart, cstop, err := createdTimeRangeFromQueryString(&values); err == nil {
			options["cstart"], options["cstop"] = cstart, cstop
		} else if err != nil {
			render.Render(w, r, NewErrQueryParameter(err))
			return
		}

		// options["fieldName"], options["fieldValue"] = r.URL.Query().Get("fieldName"), r.URL.Query().Get("fieldValue")

		var roles []models.UserRole
		tx := db.Shared().Begin()
		if modelObjs, roles, err = mapper.ReadAll(tx, OwnerIDFromContext(r), typeString, options); err != nil {
			tx.Rollback()
			log.Println("Error in ReadAllHandler ErrNotFound:", typeString, err)
			render.Render(w, r, NewErrNotFound(err))
			return
		}
		err = tx.Commit().Error
		if err != nil {
			log.Println("Error in ReadAllHandler ErrDBError:", typeString, err)
			tx.Rollback() // what if roll back fails??
			render.Render(w, r, NewErrDBError(err))
			return
		}

		renderModelSlice(w, r, typeString, modelObjs, roles)
		return
	}
}

// CreateOneHandler creates a resource
func CreateOneHandler(typeString string, mapper datamapper.ICreateOneMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		ownerID := OwnerIDFromContext(r)

		var err error

		modelObj, httperr := ModelFromJSONBody(r, typeString)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin()
		if modelObj, err = mapper.CreateOne(tx, ownerID, typeString, modelObj); err != nil {
			// FIXME, there is more than one type of error here
			// How do I output more detailed messages by inspecting error?
			tx.Rollback()
			log.Println("Error in CreateOne ErrCreate:", typeString, err)
			render.Render(w, r, NewErrCreate(err))
			return
		}

		err = tx.Commit().Error
		if err != nil {
			log.Println("Error in CreateOne ErrDBError:", typeString, err)
			tx.Rollback() // what if roll back fails??
			render.Render(w, r, NewErrDBError(err))
			return
		}

		renderModel(w, r, typeString, modelObj, models.Admin)
		return
	}
}

// ReadOneHandler returns a http.Handler which read one resource
func ReadOneHandler(typeString string, mapper datamapper.IGetOneWithIDMapper) func(c *gin.Context) {
	// return func(next http.Handler) http.Handler {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request
		log.Println("ReadOneHandler")

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin()
		modelObj, role, err := mapper.GetOneWithID(tx, OwnerIDFromContext(r), typeString, *id)

		if err != nil {
			tx.Rollback()
			log.Println("Error in ReadOneHandler ErrNotFound:", typeString, err)
			render.Render(w, r, NewErrNotFound(err))
			return
		}
		err = tx.Commit().Error
		if err != nil {
			log.Println("Error in ReadOneHandler ErrDBError:", typeString, err)
			tx.Rollback() // what if roll back fails??
			render.Render(w, r, NewErrDBError(err))
			return
		}

		renderModel(w, r, typeString, modelObj, role)
		return
	}
}

// UpdateOneHandler returns a http.Handler which updates a resource
func UpdateOneHandler(typeString string, mapper datamapper.IUpdateOneWithIDMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request
		// var err error
		// var model models.DomainModel

		log.Println("UpdateOneHandler called")

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		modelObj, httperr := ModelFromJSONBody(r, typeString)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin()
		modelObj, err := mapper.UpdateOneWithID(tx, OwnerIDFromContext(r), typeString, modelObj, *id)
		if err != nil {
			tx.Rollback()
			log.Println("Error in UpdateOneHandler ErrUpdate:", typeString, err)
			render.Render(w, r, NewErrUpdate(err))
			return
		}

		err = tx.Commit().Error
		if err != nil {
			log.Println("Error in UpdateOneHandler ErrDBError:", typeString, err)
			tx.Rollback() // what if roll back fails??
			render.Render(w, r, NewErrDBError(err))
			return
		}

		renderModel(w, r, typeString, modelObj, models.Admin)
		return
	}
}

// UpdateManyHandler returns a Gin handler which updates many records
func UpdateManyHandler(typeString string, mapper datamapper.IUpdateManyMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		log.Println("UpdateManyHandler called")
		w, r := c.Writer, c.Request

		modelObjs, httperr := ModelsFromJSONBody(r, typeString)
		if httperr != nil {
			log.Println("Error in ModelsFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin()
		modelObjs, err := mapper.UpdateMany(tx, OwnerIDFromContext(r), typeString, modelObjs)
		if err != nil {
			tx.Rollback()
			log.Println("Error in UpdateManyHandler ErrUpdate:", typeString, err)
			render.Render(w, r, NewErrUpdate(err))
			return
		}

		err = tx.Commit().Error
		if err != nil {
			log.Println("Error in UpdateManyHandler ErrDBError:", typeString, err)
			tx.Rollback() // what if roll back fails??
			render.Render(w, r, NewErrDBError(err))
			return
		}

		roles := make([]models.UserRole, len(modelObjs), len(modelObjs))
		for i := 0; i < len(roles); i++ {
			roles[i] = models.Admin
		}

		renderModelSlice(w, r, typeString, modelObjs, roles)
		return
	}
}

// PatchOneHandler returns a Gin handler which patch (partial update) one record
func PatchOneHandler(typeString string, mapper datamapper.IPatchOneWithIDMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		log.Println("PatchOneHandler")
		w, r := c.Writer, c.Request

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		var jsonPatch []byte
		var err error
		if jsonPatch, err = ioutil.ReadAll(r.Body); err != nil {
			render.Render(w, r, NewErrReadingBody(err))
			return
		}

		tx := db.Shared().Begin()
		modelObj, err := mapper.PatchOneWithID(tx, OwnerIDFromContext(r), typeString, jsonPatch, *id)
		if err != nil {
			tx.Rollback()
			log.Println("Error in PatchOneHandler ErrUpdate:", typeString, err)
			render.Render(w, r, NewErrPatch(err))
			return
		}

		err = tx.Commit().Error
		if err != nil {
			log.Println("Error in PatchOneHandler ErrDBError:", typeString, err)
			tx.Rollback() // what if roll back fails??
			render.Render(w, r, NewErrDBError(err))
			return
		}

		renderModel(w, r, typeString, modelObj, models.Admin)
		return

		// type JSONPatch struct {
		// 	Op    string
		// 	Path  string
		// 	Value interface{}
		// }
	}
}

// DeleteOneHandler returns a Gin handler which delete one record
func DeleteOneHandler(typeString string, mapper datamapper.IDeleteOneWithID) func(c *gin.Context) {
	return func(c *gin.Context) {
		log.Println("DeleteOneHandler")
		w, r := c.Writer, c.Request

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin()
		modelObj, err := mapper.DeleteOneWithID(tx, OwnerIDFromContext(r), typeString, *id)
		if err != nil {
			tx.Rollback()
			log.Println("Error in DeleteOneHandler ErrDelete:", typeString, err)
			render.Render(w, r, NewErrDelete(err))
			return
		}

		err = tx.Commit().Error
		if err != nil {
			log.Println("Error in DeleteOneHandler ErrDBError:", typeString, err)
			tx.Rollback() // what if roll back fails??
			render.Render(w, r, NewErrDBError(err))
			return
		}

		renderModel(w, r, typeString, modelObj, models.Admin)
		return
	}
}

// DeleteManyHandler returns a Gin handler which delete many records
func DeleteManyHandler(typeString string, mapper datamapper.IDeleteMany) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request
		log.Println("DeleteManyHandler called")
		var err error

		modelObjs, httperr := ModelsFromJSONBody(r, typeString)
		if httperr != nil {
			log.Println("Error in ModelsFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin() // transaction
		modelObjs, err = mapper.DeleteMany(tx, OwnerIDFromContext(r), typeString, modelObjs)
		if err != nil {
			tx.Rollback()
			log.Println("Error in DeleteOneHandler ErrDelete:", typeString, err)
			render.Render(w, r, NewErrDelete(err))
			return
		}

		err = tx.Commit().Error
		if err != nil {
			log.Println("Error in DeleteOneHandler ErrDBError:", typeString, err)
			tx.Rollback() // what if roll back fails??
			render.Render(w, r, NewErrDBError(err))
			return
		}

		roles := make([]models.UserRole, len(modelObjs), len(modelObjs))
		for i := 0; i < len(roles); i++ {
			roles[i] = models.Admin
		}

		renderModelSlice(w, r, typeString, modelObjs, roles)
		return
	}
}
