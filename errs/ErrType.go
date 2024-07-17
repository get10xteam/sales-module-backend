package errs

import (
	"errors"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/rs/zerolog"
)

var errorDetailMarshallable bool

func SetErrorDetailMarshallable(marshallable bool) {
	errorDetailMarshallable = marshallable
}

// Error should NOT be created outside this package, even if its exported.
//
// Because the `code` field is not settable outside this package.
//
// Instead, use the functions that return this type, such as
// the exported ServerError and call WithMessage() or WithDetail() instead
type Error struct {
	code    string
	message string
	detail  interface{}
}

func (err Error) Error() string {
	if len(err.message) > 0 {
		return err.code + ": " + err.message
	}
	return err.code
}
func (err *Error) MarshalZerologObject(e *zerolog.Event) {
	e.Str("code", err.code).Str("message", err.message)
	if err.detail != nil {
		switch d := err.detail.(type) {
		case *Error:
			e.Object("detail", d)
		case error:
			e.AnErr("detail", d)
		default:
			e.Any("detail", d)
		}
	}
}
func (err *Error) MarshalJSON() ([]byte, error) {
	if errorDetailMarshallable && err.detail != nil {
		errDetailAsErr, detailIsErr := err.detail.(error)
		if detailIsErr {
			return json.Marshal(map[string]interface{}{
				"error":   true,
				"code":    err.code,
				"message": err.message,
				"detail":  errDetailAsErr.Error(),
			})
		}
		return json.Marshal(map[string]interface{}{
			"error":   true,
			"code":    err.code,
			"message": err.message,
			"detail":  err.detail,
		})
	}
	return json.Marshal(map[string]interface{}{
		"error":   true,
		"code":    err.code,
		"message": err.message,
	})
}
func (err *Error) WithMessage(message string) *Error {
	err.message = message
	return err
}

// if detail is an Error with the same code with err, the passed detail's message and detail overrides the outer err
func (err *Error) WithDetail(detail interface{}) *Error {
	if detailErr, ok := detail.(*Error); ok && detailErr.code == err.code {
		if err.detail != nil {
			err.detail = detailErr.detail
		}
		err.message = detailErr.message
		return err
	}
	err.detail = detail
	return err
}
func (err *Error) ShouldLog() bool {
	switch err.code {
	case "unauthorized":
		return false
	case "unauthenticated":
		return false
	case "route_not_found":
		return false
	default:
		return true
	}
}
func Is(errToCheck, target error) bool {
	return errors.Is(errToCheck, target)
}
func (err *Error) WithHTTPStatus() bool {
	switch err.code {
	case "unauthorized":
		return false
	case "unauthenticated":
		return false
	case "route_not_found":
		return false
	default:
		return true
	}
}

/*
	TODO

[1]
The problem with using this method is,
The error does NOT get forwarded into the logger error chain (httpServer.go#143)
As a result, errors might not be catched by the global HTTP error logger.

[2]
If you actually want to use HTTP response codes on errors,
You might probably want use this function signature

	func (err *Error) FiberStatus() int

And use this signature on errHandler (in httpServer.go#188)

[3]
With this current code implementation, the identification of error just get a new criteria

	Check if 200 <= HTTP status code <= 400
		and/or
	Check if error = true

What benefit do you have in mind in doing this?

[4]
With this way, the responsibility of calling this method
now lies in every controller/handler.
A very easy thing to miss.
*/

func (err *Error) FiberStatus() int {
	switch err.code {
	case "unauthorized":
		return http.StatusUnauthorized
	case "unauthenticated":
		return http.StatusForbidden
	case "bad_parameter":
		return http.StatusBadRequest
	case "server_error":
		return http.StatusInternalServerError
	case "invalid_user":
		return http.StatusBadRequest
	case "preexist":
		return http.StatusConflict
	case "notexist":
		return http.StatusNotFound
	case "route_not_found":
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// coba dokumentasi based on ts
//
