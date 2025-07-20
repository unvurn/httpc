package httpc

import (
	"errors"
	"io"
	"net/http"
	"strings"
)

type DecoderFunc[T any] func(io.Reader) (T, error)

// do HTTPリクエストを実行する
//
// reqはhttp.Requestを表し、respondersはレスポンスを処理するための関数のスライスです。
// レスポンスの型Tを返し、エラーが発生した場合はエラーを返します。
//
// note: build と Do はそれぞれ http.Request を引数とすることから [http] への依存を起こしています。
// 当該依存関係が正当なものかの再検討により、今後この関数は再設計の対象となりえます。
func do[T any](client *http.Client, req *http.Request, decoders map[string]DecoderFunc[T]) (T, error) {
	var zero T

	if client == nil {
		client = http.DefaultClient
	}

	res, err := client.Do(req)
	if err != nil {
		return zero, err
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return zero, NewError(res)
	}

	var decoder DecoderFunc[T]
	for t, f := range decoders {
		h := res.Header.Get("Content-Type")
		if strings.Contains(h, t) {
			decoder = f
		}
	}
	if decoder == nil {
		return zero, errors.New("no responders")
	}

	return decoder(res.Body)
}
