package webrender

import (
	"fmt"
	"net/http"

	"github.com/go-chi/render"
	"github.com/lib/pq"
)

// https://stackoverflow.com/questions/37863374/whats-the-difference-between-responsewriter-write-and-io-writestring
// How many way to write back to HTTP?
// 1. fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path)) // this does formatting before callign write
// 2. w.Write([]byte("welcome"))     // This writes the bytes directly
// 3. io.WriteString(w, "blabla.\n") // This check if there is a writestring() method which takes string instead of buffer
// otherwise convert to bytes first before write

// http.Error
// func Error(w ResponseWriter, error string, code int) {
// 	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
// 	w.Header().Set("X-Content-Type-Options", "nosniff")
// 	w.WriteHeader(code)
// 	fmt.Fprintln(w, error)
// }

// For a table of HTTP status codes (400, 401, etc) see here
// https://golang.org/pkg/net/http/

// Moved to application

// NewErrClientNotAuthorized creates a new ErrClientNotAuthorized
// func NewErrClientNotAuthorized(err error) render.Renderer {
// 	return &ErrClientNotAuthorized{
// 		ErrResponse{
// 			HTTPStatusCode: http.StatusUnauthorized,
// 			Code:           1,
// 			StatusText:     "client not authorized",
// 			ErrorText:      ErrorToSensibleString(err),
// 		},
// 	}
// }

// // ErrClientNotAuthorized is when client id or secret is not registered or wrong
// type ErrClientNotAuthorized struct {
// 	ErrResponse
// }

// // NewErrTokenInvalid creates a new ErrTokenInvalid
// func NewErrTokenInvalid(err error) render.Renderer {
// 	return &ErrTokenInvalid{
// 		ErrResponse{
// 			HTTPStatusCode: http.StatusUnauthorized,
// 			Code:           2,
// 			StatusText:     "token not given, invalid or expired",
// 			ErrorText:      ErrorToSensibleString(err),
// 		},
// 	}
// }

// // ErrTokenInvalid token is not valid or expired
// type ErrTokenInvalid struct {
// 	ErrResponse
// }

// NewErrBadRequest creates a new ErrBadRequest
func NewErrBadRequest(err error) render.Renderer {
	return &ErrBadRequest{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           3,
			StatusText:     "general bad request",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrBadRequest is on all type of errors
type ErrBadRequest struct {
	ErrResponse
}

// NewErrReadingBody creates a new ErrReadingBody
func NewErrReadingBody(err error) render.Renderer {
	return &ErrReadingBody{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           4,
			StatusText:     "error in reading HTTP body",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrReadingBody reading HTTP body
type ErrReadingBody struct {
	ErrResponse
}

// NewErrParsingJSON creates a new ErrParsingJSON
func NewErrParsingJSON(err error) render.Renderer {
	return &ErrParsingJSON{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           5,
			StatusText:     "error in parsing JSON",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrParsingJSON is error parsing JSON and creating structs
type ErrParsingJSON struct {
	ErrResponse
}

// NewErrGenJSON creates a new ErrGenJSON
func NewErrGenJSON(err error) render.Renderer {
	return &ErrGenJSON{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           6,
			StatusText:     "error generating JSON",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrGenJSON is error parsing JSON and creating structs
type ErrGenJSON struct {
	ErrResponse
}

// The following two has been moved to the application

// // NewErrLoginUser creates a new ErrLoginUser
// func NewErrLoginUser(err error) render.Renderer {
// 	return &ErrLoginUser{
// 		ErrResponse{
// 			HTTPStatusCode: http.StatusBadRequest,
// 			Code:           7,
// 			StatusText:     "error login user (wrong email/password)",
// 			ErrorText:      ErrorToSensibleString(err),
// 		},
// 	}
// }

// // ErrLoginUser problem login user. Maybe the user doesn't exists.
// type ErrLoginUser struct {
// 	ErrResponse
// }

// // NewErrGeneratingToken creates a new ErrGeneratingToken
// func NewErrGeneratingToken(err error) render.Renderer {
// 	return &ErrGeneratingToken{
// 		ErrResponse{
// 			HTTPStatusCode: http.StatusBadRequest,
// 			Code:           8,
// 			StatusText:     "error in generating token", // probably problem with the private key
// 			ErrorText:      ErrorToSensibleString(err),
// 		},
// 	}
// }

// // ErrGeneratingToken shows problem with generating key
// type ErrGeneratingToken struct {
// 	ErrResponse
// }

// NewErrURLParameter creates a new ErrURLParameter
func NewErrURLParameter(err error) render.Renderer {
	return &ErrURLParameter{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           9,
			StatusText:     "error on the URL parameter",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrURLParameter is an error on the URL parameter
type ErrURLParameter struct {
	ErrResponse
}

// NewErrQueryParameter creates a new ErrQueryParameter
func NewErrQueryParameter(err error) render.Renderer {
	return &ErrQueryParameter{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           10,
			StatusText:     "error on the query parameter",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrQueryParameter is an error on the URL parameter
type ErrQueryParameter struct {
	ErrResponse
}

// NewErrNotFound creates a new ErrNotFound
func NewErrNotFound(err error) render.Renderer {
	return &ErrNotFound{
		ErrResponse{
			HTTPStatusCode: http.StatusNotFound,
			Code:           11,
			StatusText:     "resource not found",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrNotFound resource cannot be found (record doesn't exist)
type ErrNotFound struct {
	ErrResponse
}

// NewErrDBError creates a new ErrDBError
func NewErrDBError(err error) render.Renderer {
	return &ErrDBError{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           12,
			StatusText:     "problem with the database",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrDBError some problem operating with DB (maybe transaction)
type ErrDBError struct {
	ErrResponse
}

// NewErrValidation presents validation errors
// This message is different
func NewErrValidation(err error) render.Renderer {
	return &ErrDBError{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           13,

			// This one is special.. we use error at the end
			StatusText: ErrorToSensibleString(err),
		},
	}
}

// // NewErrInvalidRefreshToken presents error when refreshing
// // a token
// func NewErrInvalidRefreshToken(err error) render.Renderer {
// 	return &ErrInvalidRefreshToken{
// 		ErrResponse{
// 			HTTPStatusCode: http.StatusBadRequest,
// 			Code:           14,

// 			// This one is special.. we use error at the end
// 			StatusText: ErrorToSensibleString(err),
// 		},
// 	}
// }

// ErrPermissionDeniedForAPIEndpoint is permission denied for this endpoint
type ErrPermissionDeniedForAPIEndpoint struct {
	ErrResponse
}

// NewErrPermissionDeniedForAPIEndpoint creates a new ErrClientNotAuthorized
func NewErrPermissionDeniedForAPIEndpoint(err error) render.Renderer {
	return &ErrPermissionDeniedForAPIEndpoint{
		ErrResponse{
			HTTPStatusCode: http.StatusUnauthorized,
			Code:           15,
			StatusText:     "permission denied for this endpoint",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// // ErrInvalidRefreshToken some problem refreshing the token (maybe missing)
// type ErrInvalidRefreshToken struct {
// 	ErrResponse
// }

// NewErrCustomRender creates a new ErrCustomRender
// This can be use by custom renderer for the user of this library.
func NewErrCustomRender(err error) render.Renderer {
	return &ErrCustomRender{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           17,
			StatusText:     "error rendering output",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrCustomRender some problem with output custom rendering
type ErrCustomRender struct {
	ErrResponse
}

func NewErrDuplicatedRecord(err error) render.Renderer {
	return &ErrDuplicatedRecordRender{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           18,
			StatusText:     "duplicated record",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrCustomRender some problem with output custom rendering
type ErrDuplicatedRecordRender struct {
	ErrResponse
}

// General CRUD errors

// NewErrCreate creates a new ErrCreate
func NewErrCreate(err error) render.Renderer {
	return &ErrCreate{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           100,
			StatusText:     "error in creating resource",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrCreate on create (not specific yet)
type ErrCreate struct {
	ErrResponse
}

// No ErrRead because that's a ErrNotFound right now.

// NewErrUpdate creates a new ErrUpdate
func NewErrUpdate(err error) render.Renderer {
	return &ErrUpdate{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           101,
			StatusText:     "error in updating resource",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrUpdate some problem updating resource (not specific yet)
type ErrUpdate struct {
	ErrResponse
}

// NewErrPatch creates a new ErrPatch
func NewErrPatch(err error) render.Renderer {
	return &ErrPatch{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           102,
			StatusText:     "error in patching resource",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrPatch some problem patching resource (not specific yet)
// This is not even called right now because not implemented
type ErrPatch struct {
	ErrResponse
}

// NewErrDelete creates a new ErrDelete
func NewErrDelete(err error) render.Renderer {
	return &ErrDelete{
		ErrResponse{
			HTTPStatusCode: http.StatusBadRequest,
			Code:           103,
			StatusText:     "error in deleting resource",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrDelete some problem deleting resource (not specific yet)
type ErrDelete struct {
	ErrResponse
}

/*
 * Internal server error
 */

// NewErrInternalServerError
func NewErrInternalServerError(err error) render.Renderer {
	return &ErrInternalServerError{
		ErrResponse{
			HTTPStatusCode: http.StatusInternalServerError,
			Code:           500,
			StatusText:     "internal server error",
			ErrorText:      ErrorToSensibleString(err),
		},
	}
}

// ErrInternalServerError
type ErrInternalServerError struct {
	ErrResponse
}

/*
Maybe take into consideration my original design

	{
	  "error": "101",
	  "developerMessage": "給開發者看的 message",
	  "userMessage": "給使用者看的簡易 message",
	  "moreInfo": "https://xyz.com/doc/errors/101"
	}
*/
type ErrResponse struct {
	Err            error `json:"-"` // low-level runtime error
	HTTPStatusCode int   `json:"-"` // http response status code

	StatusText string `json:"msg,omitempty"`      // user-level status message
	Code       int64  `json:"code,omitempty"`     // application-specific error code
	ErrorText  string `json:"error,omitempty"`    // application-level error message, for debugging
	MoreInfo   string `json:"moreInfo,omitempty"` // URL link
}

// Render is to satisfy the render.Render interface
func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	// https://github.com/go-chi/render/blob/master/responder.go
	// Status sets a HTTP response status code hint into request context at any point
	// during the request life-cycle. Before the Responder sends its response header
	// it will check the StatusCtxKey
	w.Header().Set("Cache-Control", "no-store")
	render.Status(r, e.HTTPStatusCode)
	// render.JSON
	return nil
}

// ErrorToSensibleString handles SQL error more sensible
// (When I get around to it)
// I don't want it to say
// "error": "Error 1062: Duplicate entry '\\x12\\xF6\\x8B\\xF6b\\xBCF\\x90\\xBC\\xED\\xA0\\xACa\\x066\\x92' for key 'PRIMARY'"
func ErrorToSensibleString(err error) string {
	// me, ok := err.(*mysql.MySQLError)
	// if ok {
	// 	if me.Number == 1062 {
	// 		re := regexp.MustCompile("Duplicate entry '(.*?)'")
	// 		entry := re.FindStringSubmatch(me.Message)[1]
	// 		return fmt.Sprintf("duplicated entry '%s'", entry)
	// 	}
	// }
	if err, ok := err.(*pq.Error); ok {
		return err.Message + err.Code.Name()
	}

	if err != nil {
		return err.Error()
	} else {
		return ""
	}
}

// GuardRetVal is the return value for guard
type GuardRetVal struct {
	// ToPass is false if it is to guard against entry
	ToPass bool

	// CustomRenderer renderers controls HTTP status and return value instead of permission denied, if given.
	// If not set, permission denied by default
	Renderer render.Renderer
}

func NewRetValErrorWithMsg(msg string) *RetError {
	return &RetError{Error: fmt.Errorf(msg)}
}

func NewRetValWithError(err error) *RetError {
	return &RetError{Error: err}
}

func NewRetValWithRendererError(err error, renderer render.Renderer) *RetError {
	return &RetError{Error: err, Renderer: renderer}
}

// RetError is the hookpoint return structure
type RetError struct {
	// Error to return error, should always be set, even when having custom renderer
	// This is so that if we call datamapper or hookpoint, when we aren't using it in the
	// HTTP context, we can still handle error properly
	Error error
	// CustomRenderer if customize HTTP status
	Renderer render.Renderer
}
