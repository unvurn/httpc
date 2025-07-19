package httpc

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/schema"
)

// ResponderFunc HTTPレスポンスを処理するための関数
//
// Condition はHTTPレスポンス [*http.Response] がこの関数で定義された条件を満たすかどうかを判断します。
// Responder はレスポンスの実際の処理を担当し、型Tの値を生成して返します。
type ResponderFunc[T any] func(*http.Response) (T, error)

type ResponderOrNextFunc[T any] func(*http.Response, ResponderFunc[T]) (T, error)

// NewRequestFunc Request[T]を生成する関数
//
// レスポンダー関数を指定してHTTPレスポンスボディを処理する方法を決定します。
func NewRequestFunc[T any](responder ResponderFunc[T]) *Request[T] {
	return &Request[T]{
		headers:   make(http.Header),
		responder: responder,
	}
}

func NewRequest[T any]() *Request[T] {
	return NewRequestFunc(func(res *http.Response) (T, error) {
		var zero T

		var b []byte
		var err error
		if res.StatusCode == http.StatusOK {
			b, err = io.ReadAll(res.Body)
		}
		if err != nil {
			return zero, err
		}
		r, ok := any(b).(T)
		if !ok {
			return zero, nil
		}
		return r, err
	})
}

// NewRequestSliceFunc Request[T]を生成する関数(Tをスライス型にする時専用)
//
// レスポンダー関数を指定してHTTPレスポンスボディを処理する方法を決定します。
// Tはスライス型である必要があります。
// Deprecated: NewRequestFunc に統合予定。
/*func NewRequestSliceFunc[T ~[]E, E any](responder ResponderFunc[T]) *Request[T] {
	return &Request[T]{
		responder: responder,
	}
}*/

// Request HTTPリクエストの実装
//
// Tはレスポンスの型を表します。
type Request[T any] struct {
	method string
	url    *url.URL
	values url.Values

	headers           http.Header
	basicAuthUsername string
	basicAuthPassword string
	keepAlive         bool

	body io.Reader

	responder ResponderFunc[T]

	// HttpClient HTTPクライアントを返すメソッド
	httpClient *http.Client
}

func (r *Request[T]) WithResponder(responder ResponderOrNextFunc[T]) *Request[T] {
	r.responder = func(resp *http.Response) (T, error) {
		return responder(resp, r.responder)
	}
	return r
}

func (r *Request[T]) HTTPClient(c *http.Client) *Request[T] {
	r.httpClient = c
	return r
}

// Method HTTPリクエストのメソッドを設定
func (r *Request[T]) Method(m string) *Request[T] {
	r.method = m
	return r
}

// Headers HTTPリクエストのヘッダーを設定
func (r *Request[T]) Headers(headers any) *Request[T] {
	if h, ok := headers.(http.Header); ok {
		if r.headers == nil {
			r.headers = h
		} else {
			for key, values := range h {
				for _, value := range values {
					r.headers.Add(key, value)
				}
			}
		}
	} else if h, ok := headers.(map[string]string); ok {
		for key, value := range h {
			r.headers.Add(key, value)
		}
	} else {
		panic("invalid header type, expected http.Headers")
	}
	return r
}

// Header HTTPリクエストのヘッダーを設定(key, valueによるstringペア)
func (r *Request[T]) Header(key, value string) *Request[T] {
	r.headers.Add(key, value)
	return r
}

func (r *Request[T]) BasicAuth(username, password string) *Request[T] {
	r.basicAuthUsername = username
	r.basicAuthPassword = password
	return r
}

func (r *Request[T]) KeepAlive(keepAlive bool) *Request[T] {
	r.keepAlive = keepAlive
	return r
}

// Query HTTPリクエストのクエリパラメータを設定
func (r *Request[T]) Query(params ...any) *Request[T] {
	if len(params) == 1 {
		v := url.Values{}
		err := schema.NewEncoder().Encode(params[0], v)
		if err == nil {
			r.values = v
		}
	} else if len(params) == 2 {
		key := params[0].(string)
		value := params[1].(string)

		r.values.Set(key, value)
	} else {
		panic("invalid number of parameters for Query method, expected 1 or 2")
	}
	return r
}

// Get HTTP GETリクエストを実行
func (r *Request[T]) Get(ctx context.Context, u string, params ...any) (T, error) {
	return r.DoFunc(ctx, http.MethodGet, u, "", func() (io.Reader, error) {
		if len(params) > 0 {
			r.Query(params...)
		}
		return nil, nil
	})
}

// Post HTTP POSTリクエストを実行
func (r *Request[T]) Post(ctx context.Context, u string, params any, attachments ...MultipartFormData) (T, error) {
	var zero T

	v := url.Values{}
	if err := schema.NewEncoder().Encode(params, v); err != nil {
		return zero, err
	}

	if len(attachments) == 0 {
		ve := v.Encode()
		r.headers.Set("Content-Length", strconv.Itoa(len(ve)))
		r.body = strings.NewReader(ve)

		return r.DoFunc(ctx, http.MethodPost, u, "application/x-www-form-urlencoded", func() (io.Reader, error) {
			return strings.NewReader(ve), nil
		})
	} else {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)

		for k, vv := range v {
			for _, v := range vv {
				if err := mw.WriteField(k, v); err != nil {
					return zero, err
				}
			}
		}

		for _, a := range attachments {
			if err := a.AttachTo(mw); err != nil {
				return zero, err
			}
		}

		if err := mw.Close(); err != nil {
			return zero, err
		}

		return r.DoFunc(ctx, http.MethodPost, u, mw.FormDataContentType(), func() (io.Reader, error) {
			return &buf, nil
		})
	}
}

// DoFunc JSONエンコードされたデータをリクエストボディに含むHTTP POSTリクエストを実行
func (r *Request[T]) DoFunc(ctx context.Context, method, u, contentType string, payloadFunc func() (io.Reader, error)) (T, error) {
	var zero T

	r.method = method

	body, err := payloadFunc()
	if err != nil {
		return zero, err
	}

	if method != http.MethodGet {
		r.headers.Set("Cache-Control", "no-cache")
	}
	if contentType != "" && body != nil {
		r.headers.Set("Content-Type", contentType)
		r.body = body
	}

	err = r.loadURL(u)
	if err != nil {
		return zero, err
	}

	req, err := r.build(ctx)
	if err != nil {
		return zero, err
	}
	return do[T](r.httpClient, req, r.responder)
}

// Put HTTP PUTリクエストを実行
//
// note: このメソッドは未実装です。
func (r *Request[T]) Put(ctx context.Context) (T, error) {
	panic("implement me")
}

// Delete HTTP DELETEリクエストを実行
//
// note: このメソッドは未実装です。
func (r *Request[T]) Delete(ctx context.Context) (T, error) {
	panic("implement me")
}

// loadURL URLを分解して保持
func (r *Request[T]) loadURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	r.url = u

	if r.values == nil {
		r.values = make(url.Values)
	}
	for k, v := range r.url.Query() {
		for _, val := range v {
			r.values.Add(k, val)
		}
	}
	return nil
}

// build http.Requestインスタンスを構築
//
// コンテキストを付与してHTTPリクエスト [http.Request] を作成し、ヘッダーとボディを設定します。
// エラーが発生した場合はnilとエラーを返します。
//
// note: build と Do はそれぞれ http.Request を引数とすることから [http] への依存を起こしています。
// 当該依存関係が正当なものかの再検討により、今後この関数は再設計の対象となりえます。
func (r *Request[T]) build(ctx context.Context) (*http.Request, error) {
	r.url.RawQuery = r.values.Encode()
	req, err := http.NewRequestWithContext(ctx, r.method, r.url.String(), r.body)
	if err != nil {
		return nil, err
	}

	if r.headers != nil {
		req.Header = r.headers
	}
	if r.basicAuthUsername != "" && r.basicAuthPassword != "" {
		req.SetBasicAuth(r.basicAuthUsername, r.basicAuthPassword)
	}
	req.Close = !r.keepAlive
	return req, nil
}
