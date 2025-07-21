package httpc

import (
	"errors"
	"io"
	"net/http"
)

var ErrNoAvailableDecoder = errors.New("no available decoder")
var ErrUnexpectedType = errors.New("unexpected type")

type Error struct {
	response *http.Response

	body []byte
}

func newError(response *http.Response) error {
	return &Error{response: response}
}

func (e *Error) Error() string {
	return e.response.Status
}

func (e *Error) StatusCode() int {
	return e.response.StatusCode
}

func (e *Error) ResponseBody() ([]byte, error) {
	if e.body == nil {
		defer func() { _ = e.response.Body.Close() }()

		var err error
		e.body, err = io.ReadAll(e.response.Body)
		if err != nil {
			return nil, err
		}
	}
	return e.body, nil
}
