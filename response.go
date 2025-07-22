package httpc

import "net/http"

type ResponseResult[T any] struct {
	httpResponse *http.Response

	decoder DecoderFunc[T]
}

func NewResponseResult[T any](httpResponse *http.Response, decoder DecoderFunc[T]) *ResponseResult[T] {
	return &ResponseResult[T]{
		httpResponse: httpResponse,
		decoder:      decoder,
	}
}

func (r *ResponseResult[T]) As(value any) error {
	defer func() { _ = r.httpResponse.Body.Close() }()

	d, err := r.decoder(r.httpResponse.Body)
	if err != nil {
		return err
	}

	p, ok := value.(*T)
	if !ok {
		return ErrUnexpectedType
	}

	*p = d
	return nil
}
