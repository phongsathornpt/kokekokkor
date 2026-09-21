package translation

// RequestError marks malformed or unencodable client input before any upstream
// request has been sent.
type RequestError struct {
	Err error
}

func (e RequestError) Error() string { return "translate client request: " + e.Err.Error() }
func (e RequestError) Unwrap() error { return e.Err }

func WrapRequest(err error) error {
	if err == nil {
		return nil
	}
	return RequestError{Err: err}
}
