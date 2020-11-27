package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/t2wu/betterrest/datamapper"
	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/libs/datatypes"
	"github.com/t2wu/betterrest/libs/security"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/models/tools"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
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

func latestnFromQueryString(values *url.Values) string {
	defer delete(*values, "latestn")

	if latestn := values.Get("latestn"); latestn != "" {
		// Prevent sql injection
		_, err := strconv.Atoi(latestn)
		if err != nil {
			return ""
		}
		return latestn
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

func modelObjsToJSON(typeString string, modelObjs []models.IModel, roles []models.UserRole, scope *string) (string, error) {
	arr := make([]string, len(modelObjs))
	for i, v := range modelObjs {
		if j, err := tools.ToJSON(typeString, v, roles[i], scope); err != nil {
			return "", err
		} else {
			arr[i] = string(j)
		}
	}

	content := "[" + strings.Join(arr, ",") + "]"
	return content, nil
}

func renderModel(w http.ResponseWriter, r *http.Request, typeString string, modelObj models.IModel, role models.UserRole, scope *string) {
	// render.JSON(w, r, modelObj) // cannot use this since no picking the field we need
	jsonBytes, err := tools.ToJSON(typeString, modelObj, role, scope)
	if err != nil {
		log.Println("Error in renderModel:", err)
		render.Render(w, r, NewErrGenJSON(err))
		return
	}

	content := fmt.Sprintf("{ \"code\": 0, \"content\": %s }", string(jsonBytes))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(content))
}

func renderModelSlice(w http.ResponseWriter, r *http.Request, typeString string, modelObjs []models.IModel, roles []models.UserRole, scope *string) {
	jsonString, err := modelObjsToJSON(typeString, modelObjs, roles, scope)
	if err != nil {
		log.Println("Error in renderModelSlice:", err)
		render.Render(w, r, NewErrGenJSON(err))
		return
	}

	content := fmt.Sprintf("{ \"code\": 0, \"content\": %s }", jsonString)

	data := []byte(content)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

// ---------------------------------------------

// UserLoginHandler logs in the user. Effectively creates a JWT token for the user
func UserLoginHandler(typeString string) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		tokenHours := TokenHoursFromContext(r)
		if tokenHours == -1 {
			tokenHours = 3
		}

		scope := "owner"
		m, httperr := ModelFromJSONBody(r, "users", &scope) // m is models.IModel
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin()

		cargo := models.ModelCargo{}
		// Before hook
		if v, ok := m.(models.IBeforeLogin); ok {
			if err := v.BeforeLogin(tx, &scope, typeString, &cargo); err != nil {
				render.Render(w, r, NewErrInternalServerError(err))
				return
			}
		}

		authUser, authorized := security.GetVerifiedAuthUser(m)
		if !authorized {
			// unable to login user. maybe doesn't exist?
			// or username, password wrong
			render.Render(w, r, NewErrLoginUser(nil))
			return
		}

		// login success, return access token
		payload, err := createTokenPayloadForScope(authUser.GetID(), &scope, tokenHours)
		if err != nil {
			render.Render(w, r, NewErrGeneratingToken(err))
			return
		}

		// User hookpoing after login
		if v, ok := m.(models.IAfterLogin); ok {
			content := payload["content"].(map[string]interface{})
			oid := content["id"].(*datatypes.UUID)
			if err != nil {
				tx.Rollback()
				log.Println("Error in UserLogin creating uuid:", typeString, err)
				render.Render(w, r, NewErrNotFound(err))
			}

			payload, err = v.AfterLogin(tx, oid, &scope, typeString, &cargo, payload)
			if err != nil {
				tx.Rollback()
				log.Println("Error in UserLogin callign AfterLogin:", typeString, err)
				render.Render(w, r, NewErrNotFound(err))
				return
			}

			err = tx.Commit().Error
			if err != nil {
				log.Println("Error in UserLogin commit:", typeString, err)
				tx.Rollback() // what if roll back fails??
				render.Render(w, r, NewErrDBError(err))
				return
			}
		}

		var jsn []byte
		if jsn, err = json.Marshal(payload); err != nil {
			render.Render(w, r, NewErrGenJSON(err))
			return
		}

		w.Write(jsn)
	}
}

func getOptionByParsingURL(r *http.Request) (map[string]interface{}, error) {
	options := make(map[string]interface{})

	values := r.URL.Query()
	if o, l, err := limitAndOffsetFromQueryString(&values); err == nil {
		options["offset"], options["limit"] = o, l
	} else if err != nil {
		return nil, err
	}

	options["order"] = orderFromQueryString(&values)
	options["latestn"] = latestnFromQueryString(&values)
	options["better_otherqueries"] = values

	if cstart, cstop, err := createdTimeRangeFromQueryString(&values); err == nil {
		options["cstart"], options["cstop"] = cstart, cstop
	} else if err != nil {
		return nil, err
	}

	return options, nil
}

// ---------------------------------------------
// reflection stuff
// https://stackoverflow.com/questions/7850140/how-do-you-create-a-new-instance-of-a-struct-from-its-type-at-run-time-in-go
// https://stackoverflow.com/questions/23030884/is-there-a-way-to-create-an-instance-of-a-struct-from-a-string

// ReadAllHandler returns a Gin handler which fetch multiple records of a resource
func ReadAllHandler(typeString string, mapper datamapper.IGetAllMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request
		var err error

		ownerID, scope := OwnerIDFromContext(r), ScopeFromContext(r)

		var modelObjs []models.IModel
		options := make(map[string]interface{})

		if options, err = getOptionByParsingURL(r); err != nil {
			render.Render(w, r, NewErrQueryParameter(err))
			return
		}

		var roles []models.UserRole
		tx := db.Shared().Begin()

		if modelObjs, roles, err = mapper.ReadAll(tx, ownerID, &scope, typeString, options); err != nil {
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

		renderModelSlice(w, r, typeString, modelObjs, roles, &scope)
		return
	}
}

// CreateHandler creates a resource
func CreateHandler(typeString string, mapper datamapper.ICreateMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		var err error
		var modelObj models.IModel

		ownerID, scope := OwnerIDFromContext(r), ScopeFromContext(r)

		modelObjs, isBatch, httperr := ModelOrModelsFromJSONBody(r, typeString, &scope)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin()
		if *isBatch {
			if modelObjs, err = mapper.CreateMany(tx, ownerID, &scope, typeString, modelObjs); err != nil {
				// FIXME, there is more than one type of error here
				// How do I output more detailed messages by inspecting error?
				tx.Rollback()
				log.Println("Error in CreateMany ErrCreate:", typeString, err)
				render.Render(w, r, NewErrCreate(err))
				return
			}

			err = tx.Commit().Error
			if err != nil {
				log.Println("Error in CreateMany ErrDBError:", typeString, err)
				tx.Rollback() // what if roll back fails??
				render.Render(w, r, NewErrDBError(err))
				return
			}

			// admin is 0 so it's ok
			roles := make([]models.UserRole, 0, 20)
			for i := 0; i < len(modelObjs); i++ {
				roles = append(roles, models.Admin)
			}
			renderModelSlice(w, r, typeString, modelObjs, roles, &scope)
		} else {
			if modelObj, err = mapper.CreateOne(tx, ownerID, &scope, typeString, modelObjs[0]); err != nil {
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

			renderModel(w, r, typeString, modelObj, models.Admin, &scope)
		}

		return
	}
}

// ReadOneHandler returns a http.Handler which read one resource
func ReadOneHandler(typeString string, mapper datamapper.IGetOneWithIDMapper) func(c *gin.Context) {
	// return func(next http.Handler) http.Handler {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		ownerID, scope := OwnerIDFromContext(r), ScopeFromContext(r)

		tx := db.Shared().Begin()
		modelObj, role, err := mapper.GetOneWithID(tx, ownerID, &scope, typeString, *id)

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

		renderModel(w, r, typeString, modelObj, role, &scope)
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

		ownerID, scope := OwnerIDFromContext(r), ScopeFromContext(r)

		modelObj, httperr := ModelFromJSONBody(r, typeString, &scope)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin()
		modelObj, err := mapper.UpdateOneWithID(tx, ownerID, &scope, typeString, modelObj, *id)
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

		renderModel(w, r, typeString, modelObj, models.Admin, &scope)
		return
	}
}

// UpdateManyHandler returns a Gin handler which updates many records
func UpdateManyHandler(typeString string, mapper datamapper.IUpdateManyMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		log.Println("UpdateManyHandler called")
		w, r := c.Writer, c.Request
		ownerID, scope := OwnerIDFromContext(r), ScopeFromContext(r)

		modelObjs, httperr := ModelsFromJSONBody(r, typeString, &scope)
		if httperr != nil {
			log.Println("Error in ModelsFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin()
		modelObjs, err := mapper.UpdateMany(tx, ownerID, &scope, typeString, modelObjs)
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

		renderModelSlice(w, r, typeString, modelObjs, roles, &scope)
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

		ownerID, scope := OwnerIDFromContext(r), ScopeFromContext(r)

		tx := db.Shared().Begin()
		modelObj, err := mapper.PatchOneWithID(tx, ownerID, &scope, typeString, jsonPatch, *id)
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

		renderModel(w, r, typeString, modelObj, models.Admin, &scope)
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

		ownerID, scope := OwnerIDFromContext(r), ScopeFromContext(r)

		tx := db.Shared().Begin()
		modelObj, err := mapper.DeleteOneWithID(tx, ownerID, &scope, typeString, *id)
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

		renderModel(w, r, typeString, modelObj, models.Admin, &scope)
		return
	}
}

// DeleteManyHandler returns a Gin handler which delete many records
func DeleteManyHandler(typeString string, mapper datamapper.IDeleteMany) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request
		log.Println("DeleteManyHandler called")
		var err error

		ownerID, scope := OwnerIDFromContext(r), ScopeFromContext(r)

		modelObjs, httperr := ModelsFromJSONBody(r, typeString, &scope)
		if httperr != nil {
			log.Println("Error in ModelsFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin() // transaction
		modelObjs, err = mapper.DeleteMany(tx, ownerID, &scope, typeString, modelObjs)
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

		renderModelSlice(w, r, typeString, modelObjs, roles, &scope)
		return
	}
}
