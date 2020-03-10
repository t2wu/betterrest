package routes

import (
	"net/http"

	"github.com/go-chi/render"
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

/* // https://blog.golang.org/error-handling-and-go
Custom error like this:
type NegativeSqrtError float64

func (f NegativeSqrtError) Error() string {
    return fmt.Sprintf("math: square root of negative number %g", float64(f))
}

type SyntaxError struct {
    msg    string // description of error
    Offset int64  // error occurred after reading Offset bytes
}

func (e *SyntaxError) Error() string { return e.msg }

*/

// What's the difference between this method and the other?
// http.Error(w, err.Error(), http.StatusBadRequest)

// For a table of HTTP status codes (400, 401, etc) see here
// https://golang.org/pkg/net/http/

// ErrClientNotAuthorized is when client id or secret is not registered or wrong
var ErrClientNotAuthorized = &ErrResponse{
	HTTPStatusCode: http.StatusUnauthorized,
	AppCode:        1,
	ErrorText:      "Client is not authorized.",
}

// ErrTokenInvalid token is not valid or expired
var ErrTokenInvalid = &ErrResponse{
	HTTPStatusCode: http.StatusUnauthorized,
	AppCode:        2,
	ErrorText:      "Token not given, invalid or expired.",
}

// ErrBadRequest is on all type of errors
var ErrBadRequest = &ErrResponse{
	HTTPStatusCode: http.StatusBadRequest,
	AppCode:        3,
	ErrorText:      "General bad request.",
}

// ErrReadingBody reading HTTP body
var ErrReadingBody = &ErrResponse{
	HTTPStatusCode: http.StatusBadRequest,
	AppCode:        4,
	ErrorText:      "Error reading HTTP body",
}

// ErrParsingJSON is error parsing JSON and creating structs
var ErrParsingJSON = &ErrResponse{
	HTTPStatusCode: http.StatusBadRequest,
	AppCode:        5,
	ErrorText:      "Error parsing JSON",
}

// ErrGenJSON is error generating JSON from db
var ErrGenJSON = &ErrResponse{
	HTTPStatusCode: http.StatusBadRequest,
	AppCode:        6,
	ErrorText:      "Error generating JSON",
}

// ErrLoginUser problem login user. Maybe the user doesn't exists.
var ErrLoginUser = &ErrResponse{
	HTTPStatusCode: http.StatusBadRequest,
	AppCode:        7,
	ErrorText:      "Error login user. Wrong email/password.",
}

// ErrGeneratingToken shows problem with generating key
var ErrGeneratingToken = &ErrResponse{
	HTTPStatusCode: http.StatusBadRequest,
	AppCode:        8,
	ErrorText:      "Error generating token.", // probably problem with private key
}

// ErrParameter on parameter
var ErrParameter = &ErrResponse{
	HTTPStatusCode: http.StatusBadRequest,
	AppCode:        9,
	ErrorText:      "Error on URL parameter",
}

// ErrNotFound resource cannot be found (record doesn't exist)
var ErrNotFound = &ErrResponse{
	HTTPStatusCode: http.StatusNotFound,
	AppCode:        10,
	StatusText:     "Resource not found.",
}

// ErrDBError some problem operating with DB (maybe transaction)
var ErrDBError = &ErrResponse{
	HTTPStatusCode: http.StatusBadRequest,
	AppCode:        11,
	StatusText:     "Problem with the database (internal server error).",
}

// General CRUD errors

// ErrCreate on create (not specific yet)
var ErrCreate = &ErrResponse{
	HTTPStatusCode: http.StatusBadRequest,
	AppCode:        100,
	ErrorText:      "Error occurred when creating resource.",
}

// ErrUpdate some problem updating resource (not specific yet)
var ErrUpdate = &ErrResponse{
	HTTPStatusCode: http.StatusBadRequest,
	AppCode:        101,
	StatusText:     "Problem updating resource.",
}

// ErrPatch some problem patching resource (not specific yet)
// This is not even called right now because not implemented
var ErrPatch = &ErrResponse{
	HTTPStatusCode: http.StatusBadRequest,
	AppCode:        102,
	StatusText:     "Problem updating resource.",
}

// ErrDelete some problem deleting resource (not specific yet)
var ErrDelete = &ErrResponse{
	HTTPStatusCode: http.StatusBadRequest,
	AppCode:        103,
	StatusText:     "Problem deleting resource.",
}

/* Maybe take into consideration my original design
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

	StatusText string `json:"status,omitempty"`   // user-level status message
	AppCode    int64  `json:"code,omitempty"`     // application-specific error code
	ErrorText  string `json:"error,omitempty"`    // application-level error message, for debugging
	MoreInfo   string `json:"moreInfo,omitempty"` // URL link
}

func (e *ErrResponse) Render(w http.ResponseWriter, r *http.Request) error {
	// https://github.com/go-chi/render/blob/master/responder.go
	// Status sets a HTTP response status code hint into request context at any point
	// during the request life-cycle. Before the Responder sends its response header
	// it will check the StatusCtxKey
	render.Status(r, e.HTTPStatusCode)
	// render.JSON
	return nil
}
