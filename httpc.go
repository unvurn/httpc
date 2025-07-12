package httpc

import (
	"errors"
	"net/http"
	"reflect"
)

// do HTTPリクエストを実行する
//
// reqはhttp.Requestを表し、respondersはレスポンスを処理するための関数のスライスです。
// レスポンスの型Tを返し、エラーが発生した場合はエラーを返します。
//
// note: build と Do はそれぞれ http.Request を引数とすることから [http] への依存を起こしています。
// 当該依存関係が正当なものかの再検討により、今後この関数は再設計の対象となりえます。
func do[T any](client *http.Client, req *http.Request, responder ResponderFunc[T]) (T, error) {
	var zero T

	if client == nil {
		client = http.DefaultClient
	}

	res, err := client.Do(req)
	if err != nil {
		return zero, err
	}

	defer func() { _ = res.Body.Close() }()
	response, err := responder(res)
	if err != nil {
		return zero, err
	}
	if !isNilOrZero(response) {
		return response, nil
	}

	// as default error responder
	if res.StatusCode != http.StatusOK {
		return zero, NewError(res)
	}

	return zero, errors.New("no responders")
}

// Deprecated
func isNilOrZero[T any](v T) bool {
	switch (reflect.TypeOf(v)).Kind() {
	case reflect.Slice, reflect.Map:
		return reflect.ValueOf(v).Len() == 0
	case reflect.Pointer:
		return reflect.ValueOf(v).IsNil() || reflect.ValueOf(v).Elem().IsZero()
	default:
		return reflect.ValueOf(v).IsZero()
	}
}
