package httpc

import (
	"net/http"
)

type HttpResult[T any] struct {
	Response *http.Response

	bytes   []byte
	decoder DecoderFunc[T]
}

func newHttpResult[T any](response *http.Response, bytes []byte, decoder DecoderFunc[T]) *HttpResult[T] {
	return &HttpResult[T]{
		Response: response,
		bytes:    bytes,
		decoder:  decoder,
	}
}

func (r *HttpResult[T]) As(value any) error {
	switch v := value.(type) {
	case *[]byte:
		*v = r.bytes
		return nil
	case *T:
		if r.decoder == nil {
			return ErrNoAvailableDecoder
		}
		d, err := r.decoder(r.bytes)
		if err != nil {
			return err
		}
		*v = d
		return nil
	default:
		return ErrUnexpectedType
	}
}
