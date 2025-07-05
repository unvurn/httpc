package httpc

import (
	"errors"
	"io"
	"net/http"
)

// HTTPレスポンスを処理するためのレスポンダー
//
// ただし何も変換せず、[]byte型のまま返します。
var defaultResponders = []*ResponderFunc[[]byte]{
	{
		Condition: func(res *http.Response) bool {
			return res.StatusCode == http.StatusOK
		},
		Responder: func(res *http.Response) ([]byte, error) {
			defer func() { _ = res.Body.Close() }()
			data, err := io.ReadAll(res.Body)
			if err != nil {
				return nil, err
			}
			return data, nil
		},
	},
}

// NewRequest はHTTPリクエストを生成
//
// defaultRespondersを使用して、HTTPレスポンスを処理するための Request[[]byte] を生成します。
func NewRequest() Request[[]byte] {
	return NewRequestSliceFunc(defaultResponders)
}

// do HTTPリクエストを実行する
//
// reqはhttp.Requestを表し、respondersはレスポンスを処理するための関数のスライスです。
// レスポンスの型Tを返し、エラーが発生した場合はエラーを返します。
//
// note: build と Do はそれぞれ http.Request を引数とすることから [http] への依存を起こしています。
// 当該依存関係が正当なものかの再検討により、今後この関数は再設計の対象となりえます。
func do[T any](client *http.Client, req *http.Request, responders []*ResponderFunc[T]) (T, error) {
	var zero T

	if client == nil {
		client = http.DefaultClient
	}

	res, err := client.Do(req)
	if err != nil {
		return zero, err
	}

	if res.StatusCode != http.StatusOK {
		return zero, errors.New(res.Status)
	}

	for _, responder := range responders {
		if responder.Condition(res) {
			return responder.Responder(res)
		}
	}

	return zero, errors.New("no responders")
}
