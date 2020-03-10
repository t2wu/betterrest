package routes

import (
	"betterrest/datamapper"
	"betterrest/db"
	"betterrest/libs/security"
	"betterrest/models"
	"betterrest/models/tools"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/jinzhu/gorm"
)

// ---------------------------------------------
func limitAndOffsetFromQueryString(w http.ResponseWriter, r *http.Request) (int, int, error) {
	// Can I do the following in one statement?
	if offset, limit := r.URL.Query().Get("offset"), r.URL.Query().Get("limit"); offset != "" && limit != "" {
		var o, l int
		var err error
		if o, err = strconv.Atoi(offset); err != nil {
			return 0, 0, err
		}
		if l, err = strconv.Atoi(limit); err != nil {
			return 0, 0, err
		}
		return o, l, nil
	}
	return 0, 0, nil // It's ok to pass 0 limit, it'll be interpreted as an all.
}

func createdTimeRangeFromQueryString(w http.ResponseWriter, r *http.Request) (int, int, error) {
	cstart, cstop := r.URL.Query().Get("cstart"), r.URL.Query().Get("cstop")

	if cstart == "" && cstop == "" { // not specified at all
		return 0, 0, errors.New("NoTimeSpecified")
	}

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

func modelObjsToJSON(typeString string, modelObjs []models.IModel) (string, error) {

	arr := make([]string, len(modelObjs))
	for i, v := range modelObjs {
		if j, err := tools.ToJSON(typeString, v, models.Admin); err != nil {
			return "", err
		} else {
			arr[i] = string(j)
		}
	}

	content := "[" + strings.Join(arr, ",\n") + "]"
	return content, nil
}

func renderModel(w http.ResponseWriter, r *http.Request, typeString string, modelObj models.IModel) {
	// render.JSON(w, r, modelObj) // cannot use this since no picking the field we need
	jsonBytes, err := tools.ToJSON(typeString, modelObj, models.Admin)
	if err != nil {
		log.Println("ERRRR:", err)
		render.Render(w, r, ErrGenJSON)
		return
	}

	content := fmt.Sprintf("{ \"code\": 0, \"content\": %s }", string(jsonBytes))

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(content))
}

func renderModelSlice(w http.ResponseWriter, r *http.Request, typeString string, modelObjs []models.IModel) {
	jsonString, err := modelObjsToJSON(typeString, modelObjs)
	if err != nil {
		render.Render(w, r, ErrGenJSON)
		return
	}

	content := fmt.Sprintf("{ \"code\": 0, \"content\": %s }", jsonString)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write([]byte(content))
}

// ---------------------------------------------

// authUser authenticates the user
func getAuthUser(userModel models.User) (*models.User, bool) {
	// Query database
	// This is a value type
	userModel2 := &models.User{}
	err := db.Shared().Where("email = ?", userModel.Email).First(userModel2).Error
	if gorm.IsRecordNotFoundError(err) {
		return nil, false // User doesn't exists with this email
	} else if err != nil {
		// Some other unknown error
		return nil, false
	}

	if !security.IsSamePassword(userModel.Password, userModel2.PasswordHash) {
		// Password doesn't match
		return nil, false
	}

	return userModel2, true
}

// ---------------------------------------------

// UserLoginHandler logs in the user. Effectively creates a JWT token for the user
func UserLoginHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("UserLoginHandler")

		m, httperr := ModelFromJSONBody(r, "users")
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		m2, _ := m.(*models.User)
		authUser, authorized := getAuthUser(*m2)
		if !authorized {
			// unable to login user. maybe doesn't exist?
			// or username, password wrong
			render.Render(w, r, ErrLoginUser)
			return
		}

		// login success, return jwt token2
		// If oauth
		//{ acceess_token: acces_token, token_type: "Bearer", refresh_token: refresh_token, scope: ""}
		token, err := security.CreateJWTToken(authUser.ID)

		if err != nil {
			render.Render(w, r, ErrGeneratingToken)
			return
		}

		retval := map[string]string{
			"access_token": token,
			"token_type":   "Bearer",
			// "refresh_token": "None", // TODO: to be done, need another key
		}

		var jsn []byte
		if jsn, err = json.Marshal(retval); err != nil {
			render.Render(w, r, ErrGenJSON)
			return
		}

		w.Write(jsn)
	}
}

// ---------------------------------------------
// reflection stuff
// https://stackoverflow.com/questions/7850140/how-do-you-create-a-new-instance-of-a-struct-from-its-type-at-run-time-in-go
// https://stackoverflow.com/questions/23030884/is-there-a-way-to-create-an-instance-of-a-struct-from-a-string

// ReadAllHandler returns a http handler which fetch multiple records of a resource
func ReadAllHandler(typeString string, mapper datamapper.IGetAllMapper) func(http.ResponseWriter, *http.Request) { // e.g. GET /classes
	return func(w http.ResponseWriter, r *http.Request) {
		options := make(map[string]int)
		var err error
		var modelObjs []models.IModel

		if o, l, err := limitAndOffsetFromQueryString(w, r); err == nil {
			options["offset"], options["limit"] = o, l
		}

		if cstart, cstop, err := createdTimeRangeFromQueryString(w, r); err == nil {
			options["cstart"], options["cstop"] = cstart, cstop
		}

		tx := db.Shared().Begin()
		if modelObjs, err = mapper.GetAll(tx, OwnerIDFromContext(r), typeString, options); err != nil {
			tx.Rollback()
			render.Render(w, r, ErrNotFound)
			return
		}
		if tx.Commit().Error != nil {
			render.Render(w, r, ErrDBError)
			return
		}

		renderModelSlice(w, r, typeString, modelObjs)
		return
	}
}

// CreateOneHandler creates a resource
func CreateOneHandler(typeString string, mapper datamapper.ICreateOneMapper) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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
			render.Render(w, r, ErrCreate)
			return
		}
		if tx.Commit().Error != nil {
			render.Render(w, r, ErrDBError)
			return
		}

		renderModel(w, r, typeString, modelObj)
		return
	}
}

// ReadOneHandler returns a http.Handler which read one resource
func ReadOneHandler(typeString string, mapper datamapper.IGetOneWithIDMapper) func(http.ResponseWriter, *http.Request) {
	// return func(next http.Handler) http.Handler {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("ReadOneHandler")

		id, httperr := IDFromURLQueryString(r)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin()
		modelObj, err := mapper.GetOneWithID(tx, OwnerIDFromContext(r), typeString, id)

		log.Println("modleObj:", modelObj)

		if err != nil {
			tx.Rollback()
			render.Render(w, r, ErrNotFound)
			return
		}
		if tx.Commit().Error != nil {
			render.Render(w, r, ErrDBError)
			return
		}

		renderModel(w, r, typeString, modelObj)
		return
	}
}

// UpdateOneHandler returns a http.Handler which updates a resource
func UpdateOneHandler(typeString string, mapper datamapper.IUpdateOneWithIDMapper) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// var err error
		// var model models.DomainModel

		log.Println("UpdateOneHandler called")

		id, httperr := IDFromURLQueryString(r)
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
		modelObj, err := mapper.UpdateOneWithID(tx, OwnerIDFromContext(r), typeString, modelObj, id)
		if err != nil {
			tx.Rollback()
			render.Render(w, r, ErrUpdate) // FIXEME could be more specific
			return
		}
		if tx.Commit().Error != nil {
			render.Render(w, r, ErrDBError)
			return
		}

		renderModel(w, r, typeString, modelObj)
		return
	}
}

// UpdateManyHandler returns a http handler which updates many records
func UpdateManyHandler(typeString string, mapper datamapper.IUpdateManyMapper) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("UpdateManyHandler called")

		modelObjs, httperr := ModelsFromJSONBody(r, typeString)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin()
		modelObjs, err := mapper.UpdateMany(tx, OwnerIDFromContext(r), typeString, modelObjs)
		if err != nil {
			tx.Rollback()
			render.Render(w, r, ErrUpdate) // FIXEME could be more specific
			return
		}
		if tx.Commit().Error != nil {
			render.Render(w, r, ErrDBError)
			return
		}

		renderModelSlice(w, r, typeString, modelObjs)
		return
	}
}

// DeleteOneHandler returns a http handler which delete one record
func DeleteOneHandler(typeString string, mapper datamapper.IDeleteOneWithID) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("DeleteOneHandler")

		id, httperr := IDFromURLQueryString(r)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin()
		modelObj, err := mapper.DeleteOneWithID(tx, OwnerIDFromContext(r), typeString, id)
		if err != nil {
			tx.Rollback()
			log.Println("error:", err)
			render.Render(w, r, ErrDelete) // FIXME: some other error
			return
		}
		if tx.Commit().Error != nil {
			render.Render(w, r, ErrDBError)
			return
		}

		renderModel(w, r, typeString, modelObj)
		return
	}
}

// DeleteManyHandler returns a http handler which delete many records
func DeleteManyHandler(typeString string, mapper datamapper.IDeleteMany) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		// var err error
		// var model models.DomainModel

		log.Println("DeleteManyHandler called")
		var err error

		modelObjs, httperr := ModelsFromJSONBody(r, typeString)
		if httperr != nil {
			render.Render(w, r, httperr)
			return
		}

		tx := db.Shared().Begin() // transaction
		modelObjs, err = mapper.DeleteMany(tx, OwnerIDFromContext(r), typeString, modelObjs)
		if err != nil {
			tx.Rollback()
			render.Render(w, r, ErrDelete) // FIXEME could be more specific
			return
		}
		if tx.Commit().Error != nil {
			render.Render(w, r, ErrDBError)
			return
		}

		renderModelSlice(w, r, typeString, modelObjs)
		return
	}
}
