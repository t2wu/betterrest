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
	"github.com/t2wu/betterrest/datamapper/hfetcher"
	"github.com/t2wu/betterrest/db"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/rest"
	"github.com/t2wu/betterrest/hook/tools"
	"github.com/t2wu/betterrest/hook/userrole"
	"github.com/t2wu/betterrest/libs/settings"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/lifecycle"
	"github.com/t2wu/betterrest/mdlutil"
	"github.com/t2wu/qry/mdl"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/render"
)

// ------------------------------------------------------
type TransIDLogger struct {
}

func (t *TransIDLogger) Log(tx *gorm.DB, method, url, cardinality string) {
	if settings.Log {
		if tx == nil {
			log.Println(fmt.Sprintf("[BetterREST]: %s %s (%s), transact: n/a", method, url, cardinality))
			return
		}

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

func OrderByFromQueryString(values *url.Values) *string {
	defer delete(*values, string(urlparam.ParamOrderBy))

	if orderby := values.Get(string(urlparam.ParamOrderBy)); orderby != "" {
		return &orderby
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

func modelObjsToJSON(modelObjs []mdl.IModel, roles []userrole.UserRole, who mdlutil.UserIDFetchable) (string, error) {
	arr := make([]string, len(modelObjs))
	for i, v := range modelObjs {
		if j, err := tools.ToJSON(v, roles[i], who); err != nil {
			return "", err
		} else {
			arr[i] = string(j)
		}
	}

	content := "[" + strings.Join(arr, ",") + "]"
	return content, nil
}

func RenderModelSlice(c *gin.Context, data *hook.Data, ep *hook.EndPoint, total *int, hf *hfetcher.HandlerFetcher) {
	// Custom rendering if any
	handlers := hf.FetchHandlersForOpAndHook(ep.Op, "R")
	for _, handler := range handlers {
		if renderHook, ok := handler.(hook.IRender); ok {
			if renderHook.Render(c, data, ep, total) {
				return // maximum of one handler at a time, the hook writer has to make sure they are mutally exclusive
			}
		}
	}

	// no custom rendering
	jsonString, err := modelObjsToJSON(data.Ms, data.Roles, ep.Who)
	if err != nil {
		log.Println("Error in RenderModelSlice:", err)
		render.Render(c.Writer, c.Request, webrender.NewErrGenJSON(err))
		return
	}

	var content string
	if total != nil {
		content = fmt.Sprintf(`{ "code": 0, "total": %d, "content": %s }`, *total, jsonString)
	} else {
		content = fmt.Sprintf(`{ "code": 0, "content": %s }`, jsonString)
	}

	bytes := []byte(content)
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-store")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	c.Writer.Write(bytes)

	// If using the following method, is byte getting sent?
	// content := gin.H{"code": 0, "total": *total, "content": jsonString}
	// if total != nil {
	// 	content["total"] = *total
	// }

	// c.Header("Cache-Control", "no-store")
	// c.Header("Content-Length", strconv.Itoa(len(bytes)))
	// c.JSON(http.StatusOK, content)
}

func RenderModel(c *gin.Context, data *hook.Data, ep *hook.EndPoint, total *int, hf *hfetcher.HandlerFetcher) {
	// Custom rendering if any
	handlers := hf.FetchHandlersForOpAndHook(ep.Op, "R")
	for _, handler := range handlers {
		if renderHook, ok := handler.(hook.IRender); ok {
			if renderHook.Render(c, data, ep, total) {
				return // maximum of one handler at a time, the hook writer has to make sure they are mutally exclusive
			}
		}
	}

	RenderJSONForModel(c, data.Ms[0], data, ep)
}

func RenderJSONForModel(c *gin.Context, modelObj mdl.IModel, data *hook.Data, ep *hook.EndPoint) {
	// render.JSON(w, r, modelObj) // cannot use this since no picking the field we need
	jsonBytes, err := tools.ToJSON(modelObj, data.Roles[0], ep.Who)
	if err != nil {
		log.Println("Error in RenderModel:", err)
		render.Render(c.Writer, c.Request, webrender.NewErrGenJSON(err))
		return
	}

	content := fmt.Sprintf(`{"code": 0, "content": %s }`, string(jsonBytes))

	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-store")
	c.Writer.Write([]byte(content))
}

func CustomRender(c *gin.Context, data *hook.Data, ep *hook.EndPoint, total *int, hf *hfetcher.HandlerFetcher) bool {
	// Custom rendering if any
	handlers := hf.FetchHandlersForOpAndHook(ep.Op, "R")
	for _, handler := range handlers {
		if renderHook, ok := handler.(hook.IRender); ok {
			if renderHook.Render(c, data, ep, total) {
				return true // maximum of one handler at a time, the hook writer has to make sure they are mutally exclusive
			}
		}
	}
	return false
}

func RenderCodeAndMsg(c *gin.Context, code int, total int, msg *string, err *string) {
	output := make(map[string]interface{})
	output["code"] = code
	output["total"] = total
	if msg != nil {
		output["msg"] = msg
	}
	if err != nil {
		output["err"] = err
	}

	data, _ := json.Marshal(output)
	// content = fmt.Sprintf(`{"code": %d,  "msg": %s}`, code, msg)

	// data := []byte(content)
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-store")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(data)))
	c.Writer.Write(data)
}

// ---------------------------------------------

func GetOptionByParsingURL(r *http.Request) (map[urlparam.Param]interface{}, error) {
	options := make(map[urlparam.Param]interface{})

	values := r.URL.Query()
	if o, l, err := LimitAndOffsetFromQueryString(&values); err == nil && o != nil && l != nil {
		options[urlparam.ParamOffset], options[urlparam.ParamLimit] = *o, *l
	} else if err != nil {
		return nil, err
	}

	if order := OrderFromQueryString(&values); order != nil {
		options[urlparam.ParamOrder] = *order
	}

	if orderby := OrderByFromQueryString(&values); orderby != nil {
		options[urlparam.ParamOrderBy] = *orderby
		log.Println("orderby:", orderby)
	}

	if latestn := LatestnFromQueryString(&values); latestn != nil {
		options[urlparam.ParamLatestN] = *latestn
	}

	if latestnon := LatestnOnFromQueryString(&values); latestnon != nil {
		options[urlparam.ParamLatestNOn] = latestnon
	}

	options[urlparam.ParamOtherQueries] = values

	if cstart, cstop, err := CreatedTimeRangeFromQueryString(&values); err == nil && cstart != nil && cstop != nil {
		options[urlparam.ParamCstart], options[urlparam.ParamCstop] = *cstart, *cstop
	} else if err != nil {
		return nil, err
	}

	options[urlparam.ParamHasTotalCount] = hasTotalCountFromQueryString(&values)

	return options, nil
}

func w(handler func(c *gin.Context)) func(c *gin.Context) {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				debug.PrintStack()
				render.Render(c.Writer, c.Request, webrender.NewErrInternalServerError(nil))
				fmt.Println("Panic in webhandler", r)
			}
		}()

		handler(c)
	}
}

// ---------------------------------------------
// reflection stuff
// https://stackoverflow.com/questions/7850140/how-do-you-create-a-new-instance-of-a-struct-from-its-type-at-run-time-in-go
// https://stackoverflow.com/questions/23030884/is-there-a-way-to-create-an-instance-of-a-struct-from-a-string

// CreateHandler creates a resource

func CreateHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		ep := hook.EndPoint{
			URL:        c.Request.URL.String(),
			Op:         rest.OpCreate,
			TypeString: typeString,
			URLParams:  OptionFromContext(r),
			Who:        WhoFromContext(r),
		}

		modelObjs, isBatch, httperr := ModelOrModelsFromJSONBody(r, typeString, ep.Who)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		if *isBatch {
			ep.Cardinality = rest.CardinalityMany
			data, handlerFetcher, errRenderer := lifecycle.CreateMany(db.Shared(), mapper, modelObjs, &ep, &TransIDLogger{})
			if errRenderer != nil {
				render.Render(w, c.Request, errRenderer)
				return
			}

			// Render
			// batchRenderHelper(c, typeString, data, &ep, nil, handlerFetcher)
			RenderModelSlice(c, data, &ep, nil, handlerFetcher)
		} else {
			ep.Cardinality = rest.CardinalityOne
			data, handlerFetcher, errRenderer := lifecycle.CreateOne(db.Shared(), mapper, modelObjs[0], &ep, &TransIDLogger{})
			if errRenderer != nil {
				render.Render(w, c.Request, errRenderer)
				return
			}

			// singleRenderHelper(c, typeString, data, &ep, handlerFetcher)
			RenderModel(c, data, &ep, nil, handlerFetcher)
		}
	}
}

// ReadManyHandler returns a Gin handler which fetch multiple records of a resource
func ReadManyHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		if settings.Log {
			log.Printf("[BetterREST]: %s %s (n), transact: n/a", c.Request.Method, c.Request.URL.String())
		}

		ep := hook.EndPoint{
			URL:         c.Request.URL.String(),
			Op:          rest.OpRead,
			Cardinality: rest.CardinalityMany,
			TypeString:  typeString,
			URLParams:   OptionFromContext(r),
			Who:         WhoFromContext(r),
		}

		data, no, handlerFetcher, errRenderer := lifecycle.ReadMany(db.Shared(), mapper, &ep, &TransIDLogger{})
		if errRenderer != nil {
			render.Render(w, r, errRenderer)
			return
		}

		// batchRenderHelper(c, typeString, data, &ep, no, handlerFetcher)
		RenderModelSlice(c, data, &ep, no, handlerFetcher)
	}
}

// ReadOneHandler returns a http.Handler which read one resource
func ReadOneHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	// return func(next http.Handler) http.Handler {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		if settings.Log {
			log.Printf("[BetterREST]: %s %s (1), transact: n/a\n", c.Request.Method, c.Request.URL.String())
		}

		ep := hook.EndPoint{
			URL:         c.Request.URL.String(),
			Op:          rest.OpRead,
			Cardinality: rest.CardinalityOne,
			TypeString:  typeString,
			URLParams:   OptionFromContext(r),
			Who:         WhoFromContext(r),
		}
		data, handlerFetcher, errRenderer := lifecycle.ReadOne(db.Shared(), mapper, id, &ep, &TransIDLogger{})
		if errRenderer != nil {
			render.Render(w, c.Request, errRenderer)
			return
		}

		// singleRenderHelper(c, typeString, data, &ep, handlerFetcher)
		RenderModel(c, data, &ep, nil, handlerFetcher)
	}
}

// UpdateManyHandler returns a Gin handler which updates many records
func UpdateManyHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		ep := hook.EndPoint{
			URL:         c.Request.URL.String(),
			Op:          rest.OpUpdate,
			Cardinality: rest.CardinalityMany,
			TypeString:  typeString,
			URLParams:   OptionFromContext(r),
			Who:         WhoFromContext(r),
		}

		modelObjs, httperr := ModelsFromJSONBody(r, typeString, ep.Who, true)
		if httperr != nil {
			log.Println("Error in ModelsFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		data, handlerFetcher, errRenderer := lifecycle.UpdateMany(db.Shared(), mapper, modelObjs, &ep, &TransIDLogger{})
		if errRenderer != nil {
			render.Render(w, r, errRenderer)
			return
		}

		// batchRenderHelper(c, typeString, data, &ep, nil, handlerFetcher)
		RenderModelSlice(c, data, &ep, nil, handlerFetcher)
	}
}

// UpdateOneHandler returns a http.Handler which updates a resource
func UpdateOneHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		ep := hook.EndPoint{
			URL:         c.Request.URL.String(),
			Op:          rest.OpUpdate,
			Cardinality: rest.CardinalityOne,
			TypeString:  typeString,
			URLParams:   OptionFromContext(r),
			Who:         WhoFromContext(r),
		}

		modelObj, httperr := ModelFromJSONBody(r, typeString, ep.Who)
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

		data, handlerFetcher, errRenderer := lifecycle.UpdateOne(db.Shared(), mapper, modelObj, id, &ep, &TransIDLogger{})
		if errRenderer != nil {
			render.Render(w, r, errRenderer)
			return
		}

		// singleRenderHelper(c, typeString, data, &ep, handlerFetcher)
		RenderModel(c, data, &ep, nil, handlerFetcher)
	}
}

// PatchManyHandler returns a Gin handler which patch (partial update) many records
func PatchManyHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		jsonIDPatches, httperr := JSONPatchesFromJSONBody(r)
		if httperr != nil {
			log.Println("Error in JSONPatchesFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		ep := hook.EndPoint{
			URL:         c.Request.URL.String(),
			Op:          rest.OpPatch,
			Cardinality: rest.CardinalityMany,
			TypeString:  typeString,
			URLParams:   OptionFromContext(r),
			Who:         WhoFromContext(r),
		}

		data, handlerFetcher, errRenderer := lifecycle.PatchMany(db.Shared(), mapper, jsonIDPatches, &ep, &TransIDLogger{})
		if errRenderer != nil {
			render.Render(w, r, errRenderer)
			return
		}
		// batchRenderHelper(c, typeString, data, &ep, nil, handlerFetcher)
		RenderModelSlice(c, data, &ep, nil, handlerFetcher)
	}
}

// PatchOneHandler returns a Gin handler which patch (partial update) one record
func PatchOneHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

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

		ep := hook.EndPoint{
			URL:         c.Request.URL.String(),
			Op:          rest.OpPatch,
			Cardinality: rest.CardinalityOne,
			TypeString:  typeString,
			URLParams:   OptionFromContext(r),
			Who:         WhoFromContext(r),
		}

		data, handlerFetcher, errRenderer := lifecycle.PatchOne(db.Shared(), mapper, jsonPatch, id, &ep, &TransIDLogger{})
		if errRenderer != nil {
			render.Render(w, r, errRenderer)
			return
		}

		RenderModel(c, data, &ep, nil, handlerFetcher)
	}
}

// DeleteManyHandler returns a Gin handler which delete many records
func DeleteManyHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		ep := hook.EndPoint{
			URL:         c.Request.URL.String(),
			Op:          rest.OpDelete,
			Cardinality: rest.CardinalityMany,
			TypeString:  typeString,
			URLParams:   OptionFromContext(r),
			Who:         WhoFromContext(r),
		}

		// No validation on delete, all we have to do is to check the model ID.
		modelObjs, httperr := ModelsFromJSONBody(r, typeString, ep.Who, false)
		if httperr != nil {
			log.Println("Error in ModelsFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		// Chekc that ID exists
		for _, modelObj := range modelObjs {
			if modelObj.GetID() == nil {
				render.Render(w, r, webrender.NewErrParsingJSON(fmt.Errorf("id cannot be empty")))
				return
			}
		}

		// if len(modelObjs) != 0 {
		data, handlerFetcher, errRenderer := lifecycle.DeleteMany(db.Shared(), mapper, modelObjs, &ep, &TransIDLogger{})
		if errRenderer != nil {
			render.Render(w, r, errRenderer)
			return
		}

		if !CustomRender(c, data, &ep, nil, handlerFetcher) {
			RenderCodeAndMsg(c, 0, len(data.Ms), nil, nil)
		}
	}
}

// DeleteOneHandler returns a Gin handler which delete one record
func DeleteOneHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		id, httperr := IDFromURLQueryString(c)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		ep := hook.EndPoint{
			URL:         c.Request.URL.String(),
			Op:          rest.OpDelete,
			Cardinality: rest.CardinalityOne,
			TypeString:  typeString,
			URLParams:   OptionFromContext(r),
			Who:         WhoFromContext(r),
		}

		data, handlerFetcher, errRenderer := lifecycle.DeleteOne(db.Shared(), mapper, id, &ep, &TransIDLogger{})
		if errRenderer != nil {
			render.Render(w, r, errRenderer)
			return
		}

		if !CustomRender(c, data, &ep, nil, handlerFetcher) {
			RenderCodeAndMsg(c, 0, 1, nil, nil)
		}
	}
}
