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
	"github.com/t2wu/betterrest/libs/settings"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/utils/transact"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/models/tools"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
)

// ------------------------------------------------------
func logTransID(tx *gorm.DB, method, url, cardinality string) {
	if settings.Log {
		res := struct {
			TxidCurrent int
		}{}
		if err := tx.Raw("SELECT txid_current()").Scan(&res).Error; err != nil {
			s := fmt.Sprintf("[BetterREST]: Error in HTTP method: %s, Endpoint: %s, cardinality: %s", method, url, cardinality)
			log.Println(s)
			// ignore error
			return
		}

		s := fmt.Sprintf("[BetterREST]: %s %s (%s), transact: %d", method, url, cardinality, res.TxidCurrent)
		log.Println(s)
	}
}

// ---------------------------------------------
func LimitAndOffsetFromQueryString(values *url.Values) (*int, *int, error) {
	defer delete(*values, string(urlparam.ParamOffset))
	defer delete(*values, string(urlparam.ParamLimit))

	var o, l int
	var err error

	offset := values.Get(string(urlparam.ParamOffset))
	limit := values.Get(string(urlparam.ParamLimit))

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

func OrderFromQueryString(values *url.Values) *string {
	defer delete(*values, string(urlparam.ParamOrder))

	if order := values.Get(string(urlparam.ParamOrder)); order != "" {
		// Prevent sql injection
		if order != "desc" && order != "asc" {
			return nil
		}
		return &order
	}
	return nil
}

func LatestnFromQueryString(values *url.Values) *string {
	defer delete(*values, string(urlparam.ParamLatestN))

	if latestn := values.Get(string(urlparam.ParamLatestN)); latestn != "" {
		// Prevent sql injection
		_, err := strconv.Atoi(latestn)
		if err != nil {
			return nil
		}
		return &latestn
	}

	return nil
}

func LatestnOnFromQueryString(values *url.Values) []string {
	defer delete(*values, string(urlparam.ParamLatestNOn))
	return (*values)[string(urlparam.ParamLatestNOn)]
}

func CreatedTimeRangeFromQueryString(values *url.Values) (*int, *int, error) {
	defer delete(*values, string(urlparam.ParamCstart))
	defer delete(*values, string(urlparam.ParamCstop))

	if cstart, cstop := values.Get(string(urlparam.ParamCstart)),
		values.Get(string(urlparam.ParamCstop)); cstart != "" && cstop != "" {
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
	defer delete(*values, string(urlparam.ParamHasTotalCount))
	if totalCount := values.Get(string(urlparam.ParamHasTotalCount)); totalCount != "" && totalCount == "true" {
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

func RenderModel(c *gin.Context, modelObj models.IModel, hpdata *models.HookPointData, op models.CRUPDOp) {
	if mrender, ok := modelObj.(models.IHasRenderer); ok {
		if mrender.Render(c, hpdata, op) {
			return
		}
	}

	RenderJSONForModel(c, modelObj, hpdata)
}

func RenderJSONForModel(c *gin.Context, modelObj models.IModel, hpdata *models.HookPointData) {
	// render.JSON(w, r, modelObj) // cannot use this since no picking the field we need
	jsonBytes, err := tools.ToJSON(hpdata.TypeString, modelObj, *hpdata.Role, hpdata.Who)
	if err != nil {
		log.Println("Error in RenderModel:", err)
		render.Render(c.Writer, c.Request, webrender.NewErrGenJSON(err))
		return
	}

	content := fmt.Sprintf("{ \"code\": 0, \"content\": %s }", string(jsonBytes))

	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-store")
	c.Writer.Write([]byte(content))
}

func RenderModelSlice(c *gin.Context, modelObjs []models.IModel, total *int, bhpdata *models.BatchHookPointData, op models.CRUPDOp) {
	// BatchRenderer
	if renderer := models.ModelRegistry[bhpdata.TypeString].BatchRenderer; renderer != nil {
		if renderer(c, modelObjs, bhpdata, op) {
			return
		}
	}

	jsonString, err := modelObjsToJSON(bhpdata.TypeString, modelObjs, bhpdata.Roles, bhpdata.Who)
	if err != nil {
		log.Println("Error in RenderModelSlice:", err)
		render.Render(c.Writer, c.Request, webrender.NewErrGenJSON(err))
		return
	}

	var content string
	if total != nil {
		content = fmt.Sprintf("{ \"code\": 0, \"total\": %d, \"content\": %s }", *total, jsonString)
	} else {
		content = fmt.Sprintf("{ \"code\": 0, \"content\": %s }", jsonString)
	}

	data := []byte(content)
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-store")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(data)))
	c.Writer.Write(data)
}

// ---------------------------------------------

// UserLoginHandler logs in the user. Effectively creates a JWT token for the user
func UserLoginHandler(typeString string) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in GetAllHandler", r)
			}
		}()

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

		// How to make transact better?!
		// normal, internal-server, login-failed, generate-token-error, not-found
		reason := "normal"
		var realerr error
		var payload map[string]interface{}

		err := transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			cargo := models.ModelCargo{}
			// Before hook
			if v, ok := m.(models.IBeforeLogin); ok {
				hpdata := models.HookPointData{DB: tx, Who: who, TypeString: typeString, Cargo: &cargo}
				if err := v.BeforeLogin(hpdata); err != nil {
					reason = "internal-server"
					return err
				}
			}

			authUserModel, err := security.GetVerifiedAuthUser(tx, m)
			if err != nil {
				realerr = err
				reason = "login-failed"

				if v, ok := authUserModel.(models.IAfterLoginFailed); ok {
					who.Oid = authUserModel.GetID()
					hpdata := models.HookPointData{DB: tx, Who: who, TypeString: typeString, Cargo: &cargo}
					if err := v.AfterLoginFailed(hpdata); err != nil {
						realerr = err
						log.Println("Error in UserLogin callign AfterLogin:", typeString, err)
						return nil // since we don't want to cancel the transaction
					}
				}

				return nil // since we don't want to cancel the transaction
			}

			// login success, return access token
			payload, err = createTokenPayloadForScope(authUserModel.GetID(), &scope, tokenHours)
			if err != nil {
				reason = "generate-token-error"
				return err
			}

			// User hookpoint after login
			if v, ok := authUserModel.(models.IAfterLogin); ok {
				who.Oid = authUserModel.GetID()
				hpdata := models.HookPointData{DB: tx, Who: who, TypeString: typeString, Cargo: &cargo}
				payload, err = v.AfterLogin(hpdata, payload)
				if err != nil {
					realerr = err
					reason = "not-found"

					log.Println("Error in UserLogin calling AfterLogin:", typeString, err)
					return // commit
				}
			}

			return nil
		})

		if err == nil {
			err = realerr
		}

		if err != nil {
			switch reason {
			case "login-failed":
				render.Render(w, r, webrender.NewErrLoginUser(err))
			case "generate-token-error":
				render.Render(w, r, webrender.NewErrGeneratingToken(err))
			case "not-found":
				render.Render(w, r, webrender.NewErrNotFound(err))
			case "internal-server":
				fallthrough
			default:
				render.Render(w, r, webrender.NewErrInternalServerError(err))
			}

			return
		}

		var jsn []byte
		if jsn, err = json.Marshal(payload); err != nil {
			render.Render(w, r, webrender.NewErrGenJSON(err))
			return
		}

		w.Write(jsn)
	}
}

// UserVerifyEmailHandler verifies the user's email
// func UserVerifyEmailHandler(typeString string) func(c *gin.Context) {
// 	return func(c *gin.Context) {
// 		w, r := c.Writer, c.Request
// 		values := r.URL.Query()
// 		email := values.Get("email") // verification email
// 		code := values.Get("code")   // verification code

// 		// Query the library for verification of this email
// 		err := libs.Transact(db.Shared(), func(tx *gorm.DB) error {
// 			model := models.NewFromTypeString(typeString)
// 			if err := tx.Model(&model).Where("email = ? AND code = ? AND status = ?",
// 				email, code, models.UserStatusUnverified).Error; err != nil {
// 				return fmt.Errorf("account verification failed")
// 			}

// 			if err := tx.Model(&model).Update("status", models.UserStatusActive).Error; err != nil {
// 				err := fmt.Errorf("account failed to activate")
// 				return err
// 			}
// 			return nil
// 		})

// 		if err != nil {
// 			render.Render(w, r, NewErrVerify(err))
// 		}

// 		content := fmt.Sprintf("{ \"code\": 0 }")
// 		data := []byte(content)

// 		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
// 		w.Write(data)
// 	}
// }

func GetOptionByParsingURL(r *http.Request) (map[urlparam.Param]interface{}, error) {
	options := make(map[urlparam.Param]interface{})

	values := r.URL.Query()
	if o, l, err := LimitAndOffsetFromQueryString(&values); err == nil {
		if o != nil && l != nil {
			options[urlparam.ParamOffset], options[urlparam.ParamLimit] = o, l
		}
	} else if err != nil {
		return nil, err
	}

	if order := OrderFromQueryString(&values); order != nil {
		options[urlparam.ParamOrder] = order
	}

	if latestn := LatestnFromQueryString(&values); latestn != nil {
		options[urlparam.ParamLatestN] = latestn
	}

	if latestnon := LatestnOnFromQueryString(&values); latestnon != nil {
		options[urlparam.ParamLatestNOn] = latestnon
	}

	options[urlparam.ParamOtherQueries] = values

	if cstart, cstop, err := CreatedTimeRangeFromQueryString(&values); err == nil {
		if cstart != nil && cstop != nil {
			options[urlparam.ParamCstart], options[urlparam.ParamCstop] = cstart, cstop
		}
	} else if err != nil {
		return nil, err
	}

	options[urlparam.ParamHasTotalCount] = hasTotalCountFromQueryString(&values)

	return options, nil
}

// ---------------------------------------------
// reflection stuff
// https://stackoverflow.com/questions/7850140/how-do-you-create-a-new-instance-of-a-struct-from-its-type-at-run-time-in-go
// https://stackoverflow.com/questions/23030884/is-there-a-way-to-create-an-instance-of-a-struct-from-a-string

// GetAllHandler returns a Gin handler which fetch multiple records of a resource
func GetAllHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in GetAllHandler", r)
			}
		}()

		var err error

		options := make(map[urlparam.Param]interface{})
		if options, err = GetOptionByParsingURL(r); err != nil {
			render.Render(w, r, webrender.NewErrQueryParameter(err))
			return
		}

		var modelObjs []models.IModel
		var roles []models.UserRole
		var no *int

		if settings.Log {
			log.Println(fmt.Sprintf("[BetterREST]: %s %s (n), transact: n/a", c.Request.Method, c.Request.URL.String()))
		}

		who := WhoFromContext(r)
		cargo := models.BatchHookCargo{}
		modelObjs, roles, no, err = mapper.ReadMany(db.Shared(), who, typeString, &options, &cargo)

		if err != nil {
			render.Render(w, r, webrender.NewErrInternalServerError(err))
		} else {
			bhpData := models.BatchHookPointData{Ms: modelObjs, DB: nil, Who: who,
				TypeString: typeString, Roles: roles, URLParams: &options, Cargo: &cargo}

			// the batch afterTransact hookpoint
			if afterTransact := models.ModelRegistry[typeString].AfterTransact; afterTransact != nil {
				afterTransact(bhpData, models.CRUPDOpRead)
			}

			RenderModelSlice(c, modelObjs, no, &bhpData, models.CRUPDOpRead)
		}
	}
}

// CreateHandler creates a resource
func CreateHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in CreateHandler", r)
			}
		}()

		options, err := GetOptionByParsingURL(r)
		if err != nil {
			render.Render(w, r, webrender.NewErrQueryParameter(err))
			return
		}

		who := WhoFromContext(r)

		modelObjs, isBatch, httperr := ModelOrModelsFromJSONBody(r, typeString, who)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		var modelObj models.IModel

		if *isBatch {
			cargo := models.BatchHookCargo{}

			err := transact.Transact(db.Shared(), func(tx *gorm.DB) error {
				logTransID(tx, c.Request.Method, c.Request.URL.String(), "n")

				var err2 error
				if modelObjs, err2 = mapper.CreateMany(tx, who, typeString, modelObjs, &options, &cargo); err2 != nil {
					log.Println("Error in CreateMany:", typeString, err2)
					return err2
				}
				return nil
			})

			if err != nil {
				render.Render(w, c.Request, webrender.NewErrCreate(err))
			} else {
				roles := make([]models.UserRole, len(modelObjs))
				// admin is 0 so it's ok
				for i := 0; i < len(modelObjs); i++ {
					roles[i] = models.UserRoleAdmin
				}

				bhpData := models.BatchHookPointData{Ms: modelObjs, DB: nil, Who: who,
					TypeString: typeString, Roles: roles, URLParams: &options, Cargo: &cargo}
				// the batch afterTransact hookpoint
				if afterTransact := models.ModelRegistry[typeString].AfterTransact; afterTransact != nil {
					afterTransact(bhpData, models.CRUPDOpCreate)
				}
				RenderModelSlice(c, modelObjs, nil, &bhpData, models.CRUPDOpCreate)
			}
		} else {
			cargo := models.ModelCargo{}
			var err2 error
			err := transact.Transact(db.Shared(), func(tx *gorm.DB) error {
				logTransID(tx, c.Request.Method, c.Request.URL.String(), "1")

				if modelObj, err2 = mapper.CreateOne(tx, who, typeString, modelObjs[0], &options, &cargo); err2 != nil {
					log.Println("Error in CreateOne:", typeString, err2)
					return err2
				}
				return nil
			})

			if err != nil {
				render.Render(w, c.Request, webrender.NewErrCreate(err))
			} else {
				role := models.UserRoleAdmin
				hpdata := models.HookPointData{DB: nil, Who: who, TypeString: typeString,
					URLParams: &options, Role: &role, Cargo: &cargo}
				if v, ok := modelObj.(models.IAfterTransact); ok {
					v.AfterTransact(hpdata, models.CRUPDOpCreate)
				}
				RenderModel(c, modelObj, &hpdata, models.CRUPDOpCreate)
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
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in ReadOneHandler", r)
			}
		}()

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		var err error
		options, err := GetOptionByParsingURL(r)
		if err != nil {
			render.Render(w, r, webrender.NewErrQueryParameter(err))
			return
		}

		var modelObj models.IModel
		var role models.UserRole

		if settings.Log {
			log.Println(fmt.Sprintf("[BetterREST]: %s %s (1), transact: n/a", c.Request.Method, c.Request.URL.String()))
		}

		who := WhoFromContext(r)
		cargo := models.ModelCargo{}
		modelObj, role, err = mapper.ReadOne(db.Shared(), who, typeString, id, &options, &cargo)

		if err != nil && gorm.IsRecordNotFoundError(err) {
			render.Render(w, r, webrender.NewErrNotFound(err))
		} else if err != nil {
			render.Render(w, r, webrender.NewErrInternalServerError(err))
		} else {
			hpdata := models.HookPointData{DB: nil, Who: who, TypeString: typeString, Cargo: &cargo,
				URLParams: &options, Role: &role}
			if v, ok := modelObj.(models.IAfterTransact); ok {
				v.AfterTransact(hpdata, models.CRUPDOpRead)
			}
			RenderModel(c, modelObj, &hpdata, models.CRUPDOpRead)
		}
	}
}

// UpdateOneHandler returns a http.Handler which updates a resource
func UpdateOneHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in UpdateOneHandler", r)
			}
		}()

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		options, err := GetOptionByParsingURL(r)
		if err != nil {
			render.Render(w, r, webrender.NewErrQueryParameter(err))
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
			render.Render(w, r, webrender.NewErrValidation(fmt.Errorf("JSON format not expected")))
			return
		}

		cargo := models.ModelCargo{}
		var modelObj2 models.IModel
		err = transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			logTransID(tx, c.Request.Method, c.Request.URL.String(), "1")

			if modelObj2, err = mapper.UpdateOne(tx, who, typeString, modelObj, id, &options, &cargo); err != nil {
				log.Println("Error in UpdateOneHandler ErrUpdate:", typeString, err)
				return err
			}
			return nil
		})

		if err != nil {
			render.Render(w, r, webrender.NewErrUpdate(err))
		} else {
			role := models.UserRoleAdmin
			hpdata := models.HookPointData{DB: nil, Who: who, TypeString: typeString,
				URLParams: &options, Role: &role, Cargo: &cargo}
			if v, ok := modelObj2.(models.IAfterTransact); ok {
				v.AfterTransact(hpdata, models.CRUPDOpUpdate)
			}
			RenderModel(c, modelObj2, &hpdata, models.CRUPDOpUpdate)
		}
	}
}

// UpdateManyHandler returns a Gin handler which updates many records
func UpdateManyHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in UpdateManyHandler", r)
			}
		}()

		options, err := GetOptionByParsingURL(r)
		if err != nil {
			render.Render(w, r, webrender.NewErrQueryParameter(err))
			return
		}

		who := WhoFromContext(r)

		modelObjs, httperr := ModelsFromJSONBody(r, typeString, who)
		if httperr != nil {
			log.Println("Error in ModelsFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		cargo := models.BatchHookCargo{}
		var modelObjs2 []models.IModel
		err = transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			logTransID(tx, c.Request.Method, c.Request.URL.String(), "n")

			if modelObjs2, err = mapper.UpdateMany(tx, who, typeString, modelObjs, &options, &cargo); err != nil {
				log.Println("Error in UpdateManyHandler:", typeString, err)
				return err
			}

			return nil
		})

		if err != nil {
			render.Render(w, r, webrender.NewErrUpdate(err))
		} else {
			roles := make([]models.UserRole, len(modelObjs2))
			for i := 0; i < len(roles); i++ {
				roles[i] = models.UserRoleAdmin
			}

			bhpData := models.BatchHookPointData{Ms: modelObjs, DB: nil, Who: who,
				TypeString: typeString, Roles: roles, URLParams: &options, Cargo: &cargo}

			// the batch afterTransact hookpoint
			if afterTransact := models.ModelRegistry[typeString].AfterTransact; afterTransact != nil {
				afterTransact(bhpData, models.CRUPDOpUpdate)
			}

			RenderModelSlice(c, modelObjs2, nil, &bhpData, models.CRUPDOpUpdate)
		}
	}
}

// PatchOneHandler returns a Gin handler which patch (partial update) one record
func PatchOneHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
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
			render.Render(w, r, webrender.NewErrReadingBody(err))
			return
		}

		options, err := GetOptionByParsingURL(r)
		if err != nil {
			render.Render(w, r, webrender.NewErrQueryParameter(err))
			return
		}

		who := WhoFromContext(r)

		cargo := models.ModelCargo{}
		var modelObj models.IModel
		err = transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			logTransID(tx, c.Request.Method, c.Request.URL.String(), "1")

			if modelObj, err = mapper.PatchOne(tx, who, typeString, jsonPatch, id, &options, &cargo); err != nil {
				log.Println("Error in PatchOneHandler:", typeString, err)
				return err
			}

			return nil
		})

		if err != nil {
			render.Render(w, r, webrender.NewErrPatch(err))
		} else {
			role := models.UserRoleAdmin
			hpdata := models.HookPointData{DB: nil, Who: who, TypeString: typeString,
				URLParams: &options, Role: &role, Cargo: &cargo}
			if v, ok := modelObj.(models.IAfterTransact); ok {
				v.AfterTransact(hpdata, models.CRUPDOpPatch)
			}
			RenderModel(c, modelObj, &hpdata, models.CRUPDOpPatch)
		}
	}
}

// PatchManyHandler returns a Gin handler which patch (partial update) many records
func PatchManyHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in PatchManyHandler", r)
			}
		}()

		options, err := GetOptionByParsingURL(r)
		if err != nil {
			render.Render(w, r, webrender.NewErrQueryParameter(err))
			return
		}

		who := WhoFromContext(r)

		jsonIDPatches, httperr := JSONPatchesFromJSONBody(r)
		if httperr != nil {
			log.Println("Error in JSONPatchesFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		cargo := models.BatchHookCargo{}
		var modelObjs []models.IModel
		err = transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			logTransID(tx, c.Request.Method, c.Request.URL.String(), "n")

			if modelObjs, err = mapper.PatchMany(tx, who, typeString, jsonIDPatches, &options, &cargo); err != nil {
				log.Println("Error in PatchManyHandler:", typeString, err)
				return err
			}
			return nil
		})

		if err != nil {
			render.Render(w, r, webrender.NewErrUpdate(err))
		} else {
			roles := make([]models.UserRole, len(modelObjs))
			for i := 0; i < len(roles); i++ {
				roles[i] = models.UserRoleAdmin
			}

			bhpData := models.BatchHookPointData{Ms: modelObjs, DB: nil, Who: who,
				TypeString: typeString, Roles: roles, URLParams: &options, Cargo: &cargo}

			// the batch afterTransact hookpoint
			if afterTransact := models.ModelRegistry[typeString].AfterTransact; afterTransact != nil {
				afterTransact(bhpData, models.CRUPDOpPatch)
			}

			RenderModelSlice(c, modelObjs, nil, &bhpData, models.CRUPDOpPatch)
		}
	}
}

// DeleteOneHandler returns a Gin handler which delete one record
func DeleteOneHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in DeleteOneHandler", r)
			}
		}()

		options, err := GetOptionByParsingURL(r)
		if err != nil {
			render.Render(w, r, webrender.NewErrQueryParameter(err))
			return
		}

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		who := WhoFromContext(r)

		cargo := models.ModelCargo{}
		var modelObj models.IModel
		err = transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			logTransID(tx, c.Request.Method, c.Request.URL.String(), "1")

			if modelObj, err = mapper.DeleteOne(tx, who, typeString, id, &options, &cargo); err != nil {
				log.Printf("Error in DeleteOneHandler: %s %+v\n", typeString, err)
				return err
			}
			return
		})

		if err != nil {
			render.Render(w, r, webrender.NewErrDelete(err))
		} else {
			role := models.UserRoleAdmin
			hpdata := models.HookPointData{DB: nil, Who: who, TypeString: typeString,
				URLParams: &options, Role: &role, Cargo: &cargo}
			if v, ok := modelObj.(models.IAfterTransact); ok {
				v.AfterTransact(hpdata, models.CRUPDOpDelete)
			}
			RenderModel(c, modelObj, &hpdata, models.CRUPDOpDelete)
		}
	}
}

// DeleteManyHandler returns a Gin handler which delete many records
func DeleteManyHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
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

		cargo := models.BatchHookCargo{}
		err := transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			logTransID(tx, c.Request.Method, c.Request.URL.String(), "n")

			if modelObjs, err = mapper.DeleteMany(tx, who, typeString, modelObjs, nil, &cargo); err != nil {
				log.Println("Error in DeleteOneHandler ErrDelete:", typeString, err)
				return err
			}
			return nil
		})

		if err != nil {
			render.Render(w, r, webrender.NewErrDelete(err))
		} else {
			roles := make([]models.UserRole, len(modelObjs))
			for i := 0; i < len(roles); i++ {
				roles[i] = models.UserRoleAdmin
			}

			bhpData := models.BatchHookPointData{Ms: modelObjs, DB: nil, Who: who,
				TypeString: typeString, Roles: roles, URLParams: nil, Cargo: &cargo}

			// the batch afterTransact hookpoint
			if afterTransact := models.ModelRegistry[typeString].AfterTransact; afterTransact != nil {
				afterTransact(bhpData, models.CRUPDOpDelete)
			}

			RenderModelSlice(c, modelObjs, nil, &bhpData, models.CRUPDOpDelete)
		}
	}
}

// EmailChangePasswordHandler returns a gin handler which changes password
func EmailChangePasswordHandler(typeString string, mapper datamapper.IChangeEmailPasswordMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
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
			logTransID(tx, c.Request.Method, c.Request.URL.String(), "1")

			if modelObj2, err = mapper.ChangeEmailPasswordWithID(tx, who, typeString, modelObj, id); err != nil {
				log.Println("Error in ChangeEmailPasswordHandler:", typeString, err)
				return err
			}

			return nil
		})

		if err != nil {
			render.Render(w, r, webrender.NewErrUpdate(err))
		} else {
			role := models.UserRoleAdmin
			hpdata := models.HookPointData{DB: nil, Who: who, TypeString: typeString,
				URLParams: nil, Role: &role, Cargo: nil}

			RenderJSONForModel(c, modelObj2, &hpdata)
		}
	}
}

func SendVerificationEmailHandler(typeString string, mapper datamapper.IEmailVerificationMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in CreateHandler", r)
			}
		}()

		// var modelObj models.IModel

		who := WhoFromContext(r)
		// Is there a ownerID? Probably not...
		modelObj, httperr := ModelFromJSONBody(r, typeString, who)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		// db *gorm.DB, who models.Who,
		// typeString string, modelobj models.IModel, id *datatypes.UUID
		err := transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			logTransID(tx, c.Request.Method, c.Request.URL.String(), "1")

			return mapper.SendEmailVerification(tx, who, typeString, modelObj)
		})

		if err != nil {
			render.Render(w, r, webrender.NewErrBadRequest(err)) // maybe another type of error?
		} else {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Write([]byte("{\"code\": 0}"))
		}
	}
}

// EmailVerificationHandler returns a gin handler which make the account verified and active
func EmailVerificationHandler(typeString string, mapper datamapper.IEmailVerificationMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in ChangePasswordHandler", r)
			}
		}()

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		code := c.Param("code")
		if code == "" {
			render.Render(w, r, webrender.NewErrURLParameter(errors.New("missing verification code")))
			return
		}

		// Remove this code from the db and make this user verified
		err := transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			logTransID(tx, c.Request.Method, c.Request.URL.String(), "1")

			return mapper.VerifyEmail(tx, typeString, id, code)
		})

		if err != nil {
			render.Render(w, r, webrender.NewErrBadRequest(err)) // maybe another type of error?
		} else {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Write([]byte("{\"code\": 0}"))
		}
	}
}

func SendResetPasswordHandler(typeString string, mapper datamapper.IResetPasswordMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in CreateHandler", r)
			}
		}()

		// var modelObj models.IModel

		who := WhoFromContext(r)

		log.Println("model from json body")
		modelObj, httperr := ModelFromJSONBody(r, typeString, who)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		// db *gorm.DB, who models.Who,
		// typeString string, modelobj models.IModel, id *datatypes.UUID
		err := transact.Transact(db.Shared(), func(tx *gorm.DB) (err error) {
			logTransID(tx, c.Request.Method, c.Request.URL.String(), "1")
			return mapper.SendEmailResetPassword(tx, who, typeString, modelObj)
		})

		if err != nil {
			render.Render(w, r, webrender.NewErrBadRequest(err)) // maybe another type of error?
		} else {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Write([]byte("{\"code\": 0}"))
		}
	}
}

// EmailVerificationHandler returns a gin handler which make the account verified and active
func PasswordResetHandler(typeString string, mapper datamapper.IResetPasswordMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(w, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in ChangePasswordHandler", r)
			}
		}()

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		code := c.Param("code")
		if code == "" {
			render.Render(w, r, webrender.NewErrURLParameter(errors.New("missing verification code")))
			return
		}

		modelObj, httperr := ModelFromJSONBodyNoWhoNoCheckPermissionNoTransform(r, typeString)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		// values := r.URL.Query()
		// redirectURL := values.Get(string(urlparam.ParamRedirect))

		// Remove this code from the db and make this user verified
		err := transact.Transact(db.Shared(), func(tx *gorm.DB) error {
			logTransID(tx, c.Request.Method, c.Request.URL.String(), "1")

			return mapper.ResetPassword(tx, typeString, modelObj, id, code)
		})

		if err != nil {
			render.Render(w, r, webrender.NewErrBadRequest(err)) // maybe another type of error?
		} else {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.Write([]byte("{\"code\": 0}"))
		}

		// if err != nil {
		// 	c.Redirect(http.StatusFound, redirectURL+"?error="+err.Error())
		// } else {
		// 	c.Redirect(http.StatusFound, redirectURL)
		// 	return
		// }
	}
}
