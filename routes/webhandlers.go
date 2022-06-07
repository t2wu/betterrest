package routes

import (
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
	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/libs/settings"
	"github.com/t2wu/betterrest/libs/urlparam"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/lifecycle"
	"github.com/t2wu/betterrest/models"
	"github.com/t2wu/betterrest/models/tools"
	"github.com/t2wu/betterrest/registry"

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

func modelObjsToJSON(typeString string, modelObjs []models.IModel, roles []models.UserRole, who models.UserIDFetchable) (string, error) {
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

func RenderModelSlice(c *gin.Context, total *int, data *hookhandler.Data, info *hookhandler.EndPointInfo) {
	// Any custom rendering?
	if method := registry.ModelRegistry[data.TypeString].RendererMethod; method != nil {
		if method(c, data, info) { // custom render if true
			return
		}
	}

	// no custom rendering
	jsonString, err := modelObjsToJSON(data.TypeString, data.Ms, data.Roles, data.Who)
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

func RenderModel(c *gin.Context, total *int, data *hookhandler.Data, info *hookhandler.EndPointInfo) {
	// Any custom rendering?
	if method := registry.ModelRegistry[data.TypeString].RendererMethod; method != nil {
		if method(c, data, info) { // custom render if true
			return
		}
	}

	RenderJSONForModel(c, data.Ms[0], data)
}

func RenderJSONForModel(c *gin.Context, modelObj models.IModel, data *hookhandler.Data) {
	// render.JSON(w, r, modelObj) // cannot use this since no picking the field we need
	jsonBytes, err := tools.ToJSON(data.TypeString, modelObj, data.Roles[0], data.Who)
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

func RenderModelSliceOri(c *gin.Context, modelObjs []models.IModel, total *int, bhpdata *models.BatchHookPointData, op models.CRUPDOp) {
	// BatchRenderer
	if renderer := registry.ModelRegistry[bhpdata.TypeString].BatchRenderer; renderer != nil {
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
		content = fmt.Sprintf("{\"code\": 0, \"total\": %d, \"content\": %s}", *total, jsonString)
	} else {
		content = fmt.Sprintf("{\"code\": 0, \"content\": %s}", jsonString)
	}

	data := []byte(content)
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-store")
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(data)))
	c.Writer.Write(data)
}

func RenderModelOri(c *gin.Context, modelObj models.IModel, hpdata *models.HookPointData, op models.CRUPDOp) {
	if mrender, ok := modelObj.(models.IHasRenderer); ok {
		if mrender.Render(c, hpdata, op) {
			return
		}
	}

	RenderJSONForModelOri(c, modelObj, hpdata)
}

func RenderJSONForModelOri(c *gin.Context, modelObj models.IModel, hpdata *models.HookPointData) {
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

func batchRenderHelper(c *gin.Context, typeString string, data *hookhandler.Data, info *hookhandler.EndPointInfo, no *int) {
	// Does old renderer exists?
	if renderer := registry.ModelRegistry[typeString].BatchRenderer; renderer != nil {
		// Re-create it again to remain backward compatible
		oldBatchCargo := models.BatchHookCargo{Payload: data.Cargo.Payload}
		bhpData := models.BatchHookPointData{Ms: data.Ms, DB: nil, Who: data.Who,
			TypeString: data.TypeString, Roles: data.Roles, URLParams: data.URLParams, Cargo: &oldBatchCargo}

		var op models.CRUPDOp
		switch info.Op {
		case hookhandler.RESTOpRead:
			op = models.CRUPDOpRead
		case hookhandler.RESTOpCreate:
			op = models.CRUPDOpCreate
		case hookhandler.RESTOpUpdate:
			op = models.CRUPDOpUpdate
		case hookhandler.RESTOpPatch:
			op = models.CRUPDOpPatch
		case hookhandler.RESTOpDelete:
			op = models.CRUPDOpDelete
		}

		RenderModelSliceOri(c, data.Ms, no, &bhpData, op)
		return
	}

	// Use the new renderer (renderer doesn't have to exist, but call using the new RenderModelSlice)
	RenderModelSlice(c, no, data, info)
}

func singleRenderHelper(c *gin.Context, typeString string, data *hookhandler.Data, info *hookhandler.EndPointInfo) {
	// Does old renderer exists?
	if renderer := registry.ModelRegistry[typeString].BatchRenderer; renderer != nil {
		// Re-create it again to remain backward compatible
		oldBatchCargo := models.ModelCargo{Payload: data.Cargo.Payload}
		hpdata := models.HookPointData{DB: nil, Who: data.Who,
			TypeString: data.TypeString, Role: &data.Roles[0], URLParams: data.URLParams, Cargo: &oldBatchCargo}

		var op models.CRUPDOp
		switch info.Op {
		case hookhandler.RESTOpRead:
			op = models.CRUPDOpRead
		case hookhandler.RESTOpCreate:
			op = models.CRUPDOpCreate
		case hookhandler.RESTOpUpdate:
			op = models.CRUPDOpUpdate
		case hookhandler.RESTOpPatch:
			op = models.CRUPDOpPatch
		case hookhandler.RESTOpDelete:
			op = models.CRUPDOpDelete
		}

		RenderModelOri(c, data.Ms[0], &hpdata, op)
		return
	}

	// Use the new renderer (renderer doesn't have to exist, but call using the new RenderModelSlice)
	RenderModel(c, nil, data, info)
}

// ---------------------------------------------
// reflection stuff
// https://stackoverflow.com/questions/7850140/how-do-you-create-a-new-instance-of-a-struct-from-its-type-at-run-time-in-go
// https://stackoverflow.com/questions/23030884/is-there-a-way-to-create-an-instance-of-a-struct-from-a-string

// CreateHandler creates a resource

func CreateHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		who := WhoFromContext(r)
		options := OptionFromContext(r)

		modelObjs, isBatch, httperr := ModelOrModelsFromJSONBody(r, typeString, who)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		if *isBatch {
			info := hookhandler.EndPointInfo{
				URL:         c.Request.URL.String(),
				Op:          hookhandler.RESTOpCreate,
				Cardinality: hookhandler.APICardinalityMany,
			}
			data, renderer := lifecycle.CreateMany(mapper, who, typeString, modelObjs, &info, options, &TransIDLogger{})
			if renderer != nil {
				render.Render(w, c.Request, renderer)
				return
			}

			// Render
			batchRenderHelper(c, typeString, data, &info, nil)
		} else {
			info := hookhandler.EndPointInfo{
				URL:         c.Request.URL.String(),
				Op:          hookhandler.RESTOpCreate,
				Cardinality: hookhandler.APICardinalityOne,
			}
			data, renderer := lifecycle.CreateOne(mapper, who, typeString, modelObjs[0], &info, options, &TransIDLogger{})
			if renderer != nil {
				render.Render(w, c.Request, renderer)
				return
			}

			singleRenderHelper(c, typeString, data, &info)
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

		who := WhoFromContext(r)
		options := OptionFromContext(r)
		info := hookhandler.EndPointInfo{
			URL:         c.Request.URL.String(),
			Op:          hookhandler.RESTOpRead,
			Cardinality: hookhandler.APICardinalityMany,
		}

		data, no, renderer := lifecycle.ReadMany(mapper, who, typeString, &info, options, &TransIDLogger{})
		if renderer != nil {
			render.Render(w, r, renderer)
			return
		}

		batchRenderHelper(c, typeString, data, &info, no)
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
			log.Println(fmt.Sprintf("[BetterREST]: %s %s (1), transact: n/a", c.Request.Method, c.Request.URL.String()))
		}

		info := hookhandler.EndPointInfo{
			URL:         c.Request.URL.String(),
			Op:          hookhandler.RESTOpRead,
			Cardinality: hookhandler.APICardinalityOne,
		}
		data, renderer := lifecycle.ReadOne(mapper, WhoFromContext(r), typeString, id, &info, OptionFromContext(r), &TransIDLogger{})
		if renderer != nil {
			render.Render(w, c.Request, renderer)
			return
		}

		singleRenderHelper(c, typeString, data, &info)
	}
}

// UpdateManyHandler returns a Gin handler which updates many records
func UpdateManyHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		who := WhoFromContext(r)

		info := hookhandler.EndPointInfo{
			URL:         c.Request.URL.String(),
			Op:          hookhandler.RESTOpUpdate,
			Cardinality: hookhandler.APICardinalityMany,
		}

		modelObjs, httperr := ModelsFromJSONBody(r, typeString, who)
		if httperr != nil {
			log.Println("Error in ModelsFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		data, renderer := lifecycle.UpdateMany(mapper, who, typeString, modelObjs, &info, OptionFromContext(r), &TransIDLogger{})
		if renderer != nil {
			render.Render(w, r, renderer)
			return
		}

		batchRenderHelper(c, typeString, data, &info, nil)
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

		info := hookhandler.EndPointInfo{
			URL:         c.Request.URL.String(),
			Op:          hookhandler.RESTOpUpdate,
			Cardinality: hookhandler.APICardinalityOne,
		}

		data, renderer := lifecycle.UpdateOne(mapper, who, typeString, modelObj, id, &info, OptionFromContext(r), &TransIDLogger{})
		if renderer != nil {
			render.Render(w, r, renderer)
			return
		}

		singleRenderHelper(c, typeString, data, &info)
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

		info := hookhandler.EndPointInfo{
			URL:         c.Request.URL.String(),
			Op:          hookhandler.RESTOpPatch,
			Cardinality: hookhandler.APICardinalityMany,
		}

		data, renderer := lifecycle.PatchMany(mapper, WhoFromContext(r), typeString, jsonIDPatches, &info,
			OptionFromContext(r), &TransIDLogger{})
		if renderer != nil {
			render.Render(w, r, renderer)
			return
		}
		batchRenderHelper(c, typeString, data, &info, nil)
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

		info := hookhandler.EndPointInfo{
			URL:         c.Request.URL.String(),
			Op:          hookhandler.RESTOpPatch,
			Cardinality: hookhandler.APICardinalityOne,
		}

		data, renderer := lifecycle.PatchOne(mapper, WhoFromContext(r), typeString, jsonPatch, id,
			&info, OptionFromContext(r), &TransIDLogger{})
		if renderer != nil {
			render.Render(w, r, renderer)
			return
		}

		singleRenderHelper(c, typeString, data, &info)
	}
}

// DeleteManyHandler returns a Gin handler which delete many records
func DeleteManyHandler(typeString string, mapper datamapper.IDataMapper) func(c *gin.Context) {
	return func(c *gin.Context) {
		w, r := c.Writer, c.Request

		who := WhoFromContext(r)
		modelObjs, httperr := ModelsFromJSONBody(r, typeString, who)
		if httperr != nil {
			log.Println("Error in ModelsFromJSONBody:", typeString, httperr)
			render.Render(w, r, httperr)
			return
		}

		info := hookhandler.EndPointInfo{
			URL:         c.Request.URL.String(),
			Op:          hookhandler.RESTOpDelete,
			Cardinality: hookhandler.APICardinalityMany,
		}

		data, renderer := lifecycle.DeleteMany(mapper, who, typeString, modelObjs, &info, OptionFromContext(r), &TransIDLogger{})
		if renderer != nil {
			render.Render(w, r, renderer)
			return
		}
		batchRenderHelper(c, typeString, data, &info, nil)
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

		info := hookhandler.EndPointInfo{
			URL:         c.Request.URL.String(),
			Op:          hookhandler.RESTOpDelete,
			Cardinality: hookhandler.APICardinalityOne,
		}

		data, renderer := lifecycle.DeleteOne(mapper, WhoFromContext(r), typeString, id, &info,
			OptionFromContext(r), &TransIDLogger{})
		if renderer != nil {
			render.Render(w, r, renderer)
			return
		}
		singleRenderHelper(c, typeString, data, &info)
	}
}
