package httpc

import (
	"io"
	"net/http"
)

type Error struct {
	response *http.Response
}

func NewError(response *http.Response) *Error {
	return &Error{response}
}

func (e *Error) Error() string {
	return e.response.Status
}

func (e *Error) StatusCode() int {
	return e.response.StatusCode
}

func (e *Error) ResponseBody() ([]byte, error) {
	defer func() { _ = e.response.Body.Close() }()
	body, err := io.ReadAll(e.response.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
