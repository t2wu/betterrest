package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/t2wu/betterrest/datamapper"
	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/libs/security"
	"github.com/t2wu/betterrest/libs/utils/transact"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/models/tools"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
)

// ---------------------------------------------
func limitAndOffsetFromQueryString(values *url.Values) (*int, *int, error) {
	defer delete(*values, string(datamapper.URLParamOffset))
	defer delete(*values, string(datamapper.URLParamLimit))

	var o, l int
	var err error

	offset := values.Get(string(datamapper.URLParamOffset))
	limit := values.Get(string(datamapper.URLParamLimit))

	if offset == "" && limit == "" {
		return nil, nil, nil
	}

	if offset == "" {
		return nil, nil, errors.New("limit should be used with offset")
	} else {
		if o, err = strconv.Atoi(offset); err != nil {
			return nil, nil, err
		}
	}

	if limit == "" {
		return nil, nil, errors.New("offset should be used with limit")
	} else {
		if l, err = strconv.Atoi(limit); err != nil {
			return nil, nil, err
		}
	}

	return &o, &l, nil // It's ok to pass 0 limit, it'll be interpreted as an all.
}

func orderFromQueryString(values *url.Values) *string {
	defer delete(*values, string(datamapper.URLParamOrder))

	if order := values.Get(string(datamapper.URLParamOrder)); order != "" {
		// Prevent sql injection
		if order != "desc" && order != "asc" {
			return nil
		}
		return &order
	}
	return nil
}

func latestnFromQueryString(values *url.Values) *string {
	defer delete(*values, string(datamapper.URLParamLatestN))

	if latestn := values.Get(string(datamapper.URLParamLatestN)); latestn != "" {
		// Prevent sql injection
		_, err := strconv.Atoi(latestn)
		if err != nil {
			return nil
		}
		return &latestn
	}
	return nil
}

func createdTimeRangeFromQueryString(values *url.Values) (*int, *int, error) {
	defer delete(*values, string(datamapper.URLParamCstart))
	defer delete(*values, string(datamapper.URLParamCstop))

	if cstart, cstop := values.Get(string(datamapper.URLParamCstart)),
		values.Get(string(datamapper.URLParamCstop)); cstart != "" && cstop != "" {
		var err error
		cStartInt, cStopInt := 0, 0
		if cstart != "" {
			if cStartInt, err = strconv.Atoi(cstart); err != nil {
				return nil, nil, err
			}
		} else {
			cStartInt = 0
		}

		if cstop != "" {
			if cStopInt, err = strconv.Atoi(cstop); err != nil {
				return nil, nil, err
			}
		} else {
			cStopInt = int(time.Now().Unix()) // now
		}

		return &cStartInt, &cStopInt, nil
	}
	return nil, nil, nil
}

func hasTotalCountFromQueryString(values *url.Values) bool {
	defer delete(*values, string(datamapper.URLParamHasTotalCount))
	if totalCount := values.Get(string(datamapper.URLParamHasTotalCount)); totalCount != "" && totalCount == "true" {
		return true
	}
	return false
}

func modelObjsToJSON(typeString string, modelObjs []models.IModel, roles []models.UserRole, who models.Who) (string, error) {
	arr := make([]string, len(modelObjs))
	for i, v := range modelObjs {
		if j, err := tools.ToJSON(typeString, v, roles[i], who); err != nil {
			return "", err
		} else {
			arr[i] = string(j)
		}
	}

	content := "[" + strings.Join(arr, ",") + "]"
	return content, nil
}

func renderModel(w http.ResponseWriter, r *http.Request, typeString string, modelObj models.IModel, role models.UserRole, who models.Who) {
	if mrender, ok := modelObj.(models.IHasRenderer); ok {
		w.Write(mrender.Render(role, who))
		return
	}

	// render.JSON(w, r, modelObj) // cannot use this since no picking the field we need
	jsonBytes, err := tools.ToJSON(typeString, modelObj, role, who)
	if err != nil {
		log.Println("Error in renderModel:", err)
		render.Render(w, r, NewErrGenJSON(err))
		return
	}

	content := fmt.Sprintf("{ \"code\": 0, \"content\": %s }", string(jsonBytes))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(content))
}

func renderModelSlice(w http.ResponseWriter, r *http.Request, typeString string, modelObjs []models.IModel, roles []models.UserRole, total *int, who models.Who) {
	if models.ModelRegistry[typeString].BatchRenderer != nil {
		w.Write(models.ModelRegistry[typeString].BatchRenderer(roles, who, modelObjs))
		return
	}

	jsonString, err := modelObjsToJSON(typeString, modelObjs, roles, who)
	if err != nil {
		log.Println("Error in renderModelSlice:", err)
		render.Render(w, r, NewErrGenJSON(err))
		return
	}

	var content string
	if total != nil {
		content = fmt.Sprintf("{ \"code\": 0, \"total\": %d, \"content\": %s }", *total, jsonString)
	} else {
		content = fmt.Sprintf("{ \"code\": 0, \"content\": %s }", jsonString)
	}

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

		tx := db.Shared().Begin()
		defer func(tx *gorm.DB) {
			if r := recover(); r != nil {
				tx.Rollback()
				debug.PrintStack()
				render.Render(w, c.Request, NewErrInternalServerError(nil))
				fmt.Println("Panic in UserLoginHandler", r)
			}
		}(tx)

		tokenHours := TokenHoursFromContext(r)

		client := ClientFromContext(r)
		scope := "owner"

		who := models.Who{
			// Oid: logged in yet
			Client: client,
			Scope:  &scope,
		}

		m, httperr := ModelFromJSONBody(r, "users", who) // m is models.IModel
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		cargo := models.ModelCargo{}
		// Before hook
		if v, ok := m.(models.IBeforeLogin); ok {
			hpdata := models.HookPointData{DB: tx, Who: who, TypeString: typeString, Cargo: &cargo}
			if err := v.BeforeLogin(hpdata); err != nil {
				render.Render(w, r, NewErrInternalServerError(err))
				return
			}
		}

		authUserModel, verifyUserResult := security.GetVerifiedAuthUser(m)
		if verifyUserResult == security.VerifyUserResultPasswordNotMatch {
			// User hookpoing after login
			if v, ok := authUserModel.(models.IAfterLoginFailed); ok {
				who.Oid = authUserModel.GetID()
				hpdata := models.HookPointData{DB: tx, Who: who, TypeString: typeString, Cargo: &cargo}
				if err := v.AfterLoginFailed(hpdata); err != nil {
					// tx.Rollback() // no rollback!!, actually. commit!
					tx.Commit()

					log.Println("Error in UserLogin callign AfterLogin:", typeString, err)
					render.Render(w, r, NewErrLoginUser(err))
					return
				}
			}

			// tx.Rollback() //no rollback, actually. commit
			tx.Commit()

			render.Render(w, r, NewErrLoginUser(nil))
			return
		} else if verifyUserResult != security.VerifyUserResultOK {
			// unable to login user. maybe doesn't exist?
			// or username, password wrong
			// tx.Rollback() no rollback, actually commit
			tx.Commit()

			render.Render(w, r, NewErrLoginUser(nil))
			return
		}

		// login success, return access token
		payload, err := createTokenPayloadForScope(authUserModel.GetID(), &scope, tokenHours)
		if err != nil {
			tx.Rollback()
			render.Render(w, r, NewErrGeneratingToken(err))
			return
		}

		// User hookpoing after login
		if v, ok := authUserModel.(models.IAfterLogin); ok {
			who.Oid = authUserModel.GetID()
			hpdata := models.HookPointData{DB: tx, Who: who, TypeString: typeString, Cargo: &cargo}
			payload, err = v.AfterLogin(hpdata, payload)
			if err != nil {
				// tx.Rollback() no rollback, actually, commit!
				tx.Commit() // technically this can return err, too

				log.Println("Error in UserLogin calling AfterLogin:", typeString, err)
				render.Render(w, r, NewErrNotFound(err))
				return
			}
		}

		if err := tx.Commit().Error; err != nil {
			render.Render(w, c.Request, NewErrInternalServerError(nil))
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

func getOptionByParsingURL(r *http.Request) (map[datamapper.URLParam]interface{}, error) {
	options := make(map[datamapper.URLParam]interface{})

	values := r.URL.Query()
	if o, l, err := limitAndOffsetFromQueryString(&values); err == nil {
		if o != nil && l != nil {
			options[datamapper.URLParamOffset], options[datamapper.URLParamLimit] = o, l
		}
	} else if err != nil {
		return nil, err
	}

	if order := orderFromQueryString(&values); order != nil {
		options[datamapper.URLParamOrder] = order
	}

	if latest := latestnFromQueryString(&values); latest != nil {
		options[datamapper.URLParamLatestN] = latest
	}

	options[datamapper.URLParamOtherQueries] = values

	if cstart, cstop, err := createdTimeRangeFromQueryString(&values); err == nil {
		if cstart != nil && cstop != nil {
			options[datamapper.URLParamCstart], options[datamapper.URLParamCstop] = cstart, cstop
		}
	} else if err != nil {
		return nil, err
	}

	options[datamapper.URLParamHasTotalCount] = hasTotalCountFromQueryString(&values)

	return options, nil
}

// ---------------------------------------------
// reflection stuff
// https://stackoverflow.com/questions/7850140/how-do-you-create-a-new-instance-of-a-struct-from-its-type-at-run-time-in-go
// https://stackoverflow.com/questions/23030884/is-there-a-way-to-create-an-instance-of-a-struct-from-a-string

// ReadAllHandler returns a Gin handler which fetch multiple records of a resource
func ReadAllHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, NewErrInternalServerError(nil))
				fmt.Println("Panic in ReadAllHandler", r)
			}
		}()

		var err error

		who := WhoFromContext(r)

		options := make(map[datamapper.URLParam]interface{})

		if options, err = getOptionByParsingURL(r); err != nil {
			render.Render(w, r, NewErrQueryParameter(err))
			return
		}

		var modelObjs []models.IModel
		var roles []models.UserRole
		var no *int
		err = transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			if modelObjs, roles, no, err = mapper.GetAll(tx, who, typeString, options); err != nil {
				log.Println("Error in ReadAllHandler:", typeString, err)
				return err
			}
			return
		})

		if err != nil {
			render.Render(w, r, NewErrInternalServerError(err))
		} else {
			renderModelSlice(w, r, typeString, modelObjs, roles, no, who)
		}

		return
	}
}

// CreateHandler creates a resource
func CreateHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, NewErrInternalServerError(nil))
				fmt.Println("Panic in CreateHandler", r)
			}
		}()

		var modelObj models.IModel

		who := WhoFromContext(r)

		modelObjs, isBatch, httperr := ModelOrModelsFromJSONBody(r, typeString, who)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		if *isBatch {
			err := transact.Transact(db.Shared(), func(tx *gorm.DB) error {
				var err2 error
				if modelObjs, err2 = mapper.CreateMany(tx, who, typeString, modelObjs); err2 != nil {
					log.Println("Error in CreateMany:", typeString, err2)
					return err2
				}

				// renderModelSlice(w, r, typeString, modelObjs, roles, nil, who)
				return nil
			})
			if err != nil {
				render.Render(w, c.Request, NewErrCreate(err))
			} else {
				roles := make([]models.UserRole, len(modelObjs), len(modelObjs))
				// admin is 0 so it's ok
				for i := 0; i < len(modelObjs); i++ {
					roles[i] = models.UserRoleAdmin
				}
				renderModelSlice(w, r, typeString, modelObjs, roles, nil, who)
			}
		} else {
			var err2 error
			err := transact.Transact(db.Shared(), func(tx *gorm.DB) error {
				if modelObj, err2 = mapper.CreateOne(tx, who, typeString, modelObjs[0]); err2 != nil {
					log.Println("Error in CreateOne:", typeString, err2)
					return err2
				}
				return nil
			})

			if err != nil {
				render.Render(w, c.Request, NewErrCreate(err))
			} else {
				renderModel(w, r, typeString, modelObj, models.UserRoleAdmin, who)
			}
		}
	}
}

// ReadOneHandler returns a http.Handler which read one resource
func ReadOneHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	// return func(next http.Handler) http.Handler {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, NewErrInternalServerError(nil))
				fmt.Println("Panic in ReadOneHandler", r)
			}
		}()

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		who := WhoFromContext(r)

		var modelObj models.IModel
		var role models.UserRole
		err := transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			if modelObj, role, err = mapper.GetOneWithID(tx, who, typeString, id); err != nil {
				log.Println("Error in ReadOneHandler ErrNotFound:", typeString, err)
				return err
			}
			return nil
		})

		if err != nil && gorm.IsRecordNotFoundError(err) {
			render.Render(w, r, NewErrNotFound(err))
		} else if err != nil {
			render.Render(w, r, NewErrInternalServerError(err))
		} else {
			renderModel(w, r, typeString, modelObj, role, who)
		}

		return
	}
}

// UpdateOneHandler returns a http.Handler which updates a resource
func UpdateOneHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, NewErrInternalServerError(nil))
				fmt.Println("Panic in UpdateOneHandler", r)
			}
		}()

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		who := WhoFromContext(r)

		modelObj, httperr := ModelFromJSONBody(r, typeString, who)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		// Before validation this is a temporary check
		// This traps the mistake if "content" and the array is included
		if modelObj.GetID() == nil {
			render.Render(w, r, NewErrValidation(fmt.Errorf("JSON format not expected")))
			return
		}

		var modelObj2 models.IModel
		err := transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			if modelObj2, err = mapper.UpdateOneWithID(tx, who, typeString, modelObj, id); err != nil {
				log.Println("Error in UpdateOneHandler ErrUpdate:", typeString, err)
				return err
			}
			return nil
		})

		if err != nil {
			render.Render(w, r, NewErrUpdate(err))
		} else {
			renderModel(w, r, typeString, modelObj2, models.UserRoleAdmin, who)
		}

		return
	}
}

// UpdateManyHandler returns a Gin handler which updates many records
func UpdateManyHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, NewErrInternalServerError(nil))
				fmt.Println("Panic in UpdateManyHandler", r)
			}
		}()

		who := WhoFromContext(r)

		modelObjs, httperr := ModelsFromJSONBody(r, typeString, who)
		if httperr != nil {
			log.Println("Error in ModelsFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		var modelObjs2 []models.IModel
		err := transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			if modelObjs2, err = mapper.UpdateMany(tx, who, typeString, modelObjs); err != nil {
				log.Println("Error in UpdateManyHandler:", typeString, err)
				return err
			}

			return nil
		})

		if err != nil {
			render.Render(w, r, NewErrUpdate(err))
		} else {
			roles := make([]models.UserRole, len(modelObjs2), len(modelObjs2))
			for i := 0; i < len(roles); i++ {
				roles[i] = models.UserRoleAdmin
			}
			renderModelSlice(w, r, typeString, modelObjs2, roles, nil, who)
		}

		return
	}
}

// PatchOneHandler returns a Gin handler which patch (partial update) one record
func PatchOneHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, NewErrInternalServerError(nil))
				fmt.Println("Panic in PatchOneHandler", r)
			}
		}()

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

		who := WhoFromContext(r)

		var modelObj models.IModel
		err = transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			if modelObj, err = mapper.PatchOneWithID(tx, who, typeString, jsonPatch, id); err != nil {
				log.Println("Error in PatchOneHandler:", typeString, err)
				return err
			}

			return nil
		})

		if err != nil {
			render.Render(w, r, NewErrPatch(err))
		} else {
			renderModel(w, r, typeString, modelObj, models.UserRoleAdmin, who)
		}

		// type JSONPatch struct {
		// 	Op    string
		// 	Path  string
		// 	Value interface{}
		// }
		return
	}
}

// PatchManyHandler returns a Gin handler which patch (partial update) many records
func PatchManyHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, NewErrInternalServerError(nil))
				fmt.Println("Panic in PatchManyHandler", r)
			}
		}()

		who := WhoFromContext(r)

		jsonIDPatches, httperr := JSONPatchesFromJSONBody(r)
		if httperr != nil {
			log.Println("Error in JSONPatchesFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		var modelObjs []models.IModel
		err := transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			if modelObjs, err = mapper.PatchMany(tx, who, typeString, jsonIDPatches); err != nil {
				log.Println("Error in PatchManyHandler:", typeString, err)
				return err
			}
			return nil
		})

		if err != nil {
			render.Render(w, r, NewErrUpdate(err))
		} else {
			roles := make([]models.UserRole, len(modelObjs), len(modelObjs))
			for i := 0; i < len(roles); i++ {
				roles[i] = models.UserRoleAdmin
			}
			renderModelSlice(w, r, typeString, modelObjs, roles, nil, who)
		}

		return
	}
}

// DeleteOneHandler returns a Gin handler which delete one record
func DeleteOneHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, NewErrInternalServerError(nil))
				fmt.Println("Panic in DeleteOneHandler", r)
			}
		}()

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		who := WhoFromContext(r)

		var modelObj models.IModel
		err := transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			if modelObj, err = mapper.DeleteOneWithID(tx, who, typeString, id); err != nil {
				log.Printf("Error in DeleteOneHandler: %s %+v\n", typeString, err)
				return err
			}
			return
		})

		if err != nil {
			render.Render(w, r, NewErrDelete(err))
		} else {
			renderModel(w, r, typeString, modelObj, models.UserRoleAdmin, who)
		}

		return
	}
}

// DeleteManyHandler returns a Gin handler which delete many records
func DeleteManyHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, NewErrInternalServerError(nil))
				fmt.Println("Panic in DeleteManyHandler", r)
			}
		}()

		who := WhoFromContext(r)

		modelObjs, httperr := ModelsFromJSONBody(r, typeString, who)
		if httperr != nil {
			log.Println("Error in ModelsFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		err := transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			if modelObjs, err = mapper.DeleteMany(tx, who, typeString, modelObjs); err != nil {
				log.Println("Error in DeleteOneHandler ErrDelete:", typeString, err)
				return err
			}
			return nil
		})

		if err != nil {
			render.Render(w, r, NewErrDelete(err))
		} else {
			roles := make([]models.UserRole, len(modelObjs), len(modelObjs))
			for i := 0; i < len(roles); i++ {
				roles[i] = models.UserRoleAdmin
			}
			renderModelSlice(w, r, typeString, modelObjs, roles, nil, who)
		}

		return
	}
}

// ChangeEmailPasswordHandler returns a gin handler which changes password
func ChangeEmailPasswordHandler(typeString string, mapper datamapper.IChangeEmailPasswordMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, NewErrInternalServerError(nil))
				fmt.Println("Panic in ChangePasswordHandler", r)
			}
		}()

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		who := WhoFromContext(r)

		modelObj, httperr := ModelFromJSONBody(r, typeString, who)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		var modelObj2 models.IModel
		err := transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			if modelObj2, err = mapper.ChangeEmailPasswordWithID(tx, who, typeString, modelObj, id); err != nil {
				log.Println("Error in ChangeEmailPasswordHandler:", typeString, err)
				return err
			}
			return err
		})

		if err != nil {
			render.Render(w, r, NewErrUpdate(err))
		} else {
			renderModel(w, r, typeString, modelObj2, models.UserRoleAdmin, who)
		}

		return
	}
}
