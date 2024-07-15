package errs

func ErrUnauthorized() *Error {
	return &Error{
		code:    "unauthorized",
		message: "Not authorized to access the resource",
	}
}
func ErrUnauthenticated() *Error {
	return &Error{
		code:    "unauthenticated",
		message: "Not authenticated/logged in",
	}
}
func ErrBadParameter() *Error {
	return &Error{
		code:    "bad_parameter",
		message: "Bad request parameter",
	}
}
func ErrServerError() *Error {
	return &Error{
		code:    "server_error",
		message: "System Error",
	}
}
func ErrInvalidUser() *Error {
	return &Error{
		code:    "invalid_user",
		message: "Invalid user data",
	}
}
func ErrPreexist() *Error {
	return &Error{
		code:    "preexist",
		message: "Resource already exist",
	}
}
func ErrNotExist() *Error {
	return &Error{
		code:    "notexist",
		message: "Resource does not exist",
	}
}
func ErrRouteNotFound() *Error {
	return &Error{
		code:    "route_not_found",
		message: "URL tidak ditemukan",
	}
}
