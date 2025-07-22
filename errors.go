package httpc

import (
	"errors"
	"net/http"
)

var ErrNoAvailableEncoder = errors.New("no available encoder")
var ErrNoAvailableDecoder = errors.New("no available decoder")
var ErrUnexpectedType = errors.New("unexpected type")

type Error struct {
	response *http.Response

	body []byte
}

func newError(response *http.Response, body []byte) error {
	return &Error{response: response, body: body}
}

func (e *Error) Error() string {
	return e.response.Status
}

func (e *Error) StatusCode() int {
	return e.response.StatusCode
}

func (e *Error) Body() []byte {
	return e.body
}
