package httpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/schema"
)

// Request HTTPリクエストを表すインターフェイス
//
// Tはレスポンスの型を表します。
type Request[T any] interface {
	HTTPClient(c *http.Client) Request[T]                                                        // Set the HTTP client to use for requests
	Method(m string) Request[T]                                                                  // Set the HTTP method (GET, POST, etc.)
	Headers(headers any) Request[T]                                                              // Set headers for the request
	Header(key, value string) Request[T]                                                         // Set headers for the request
	Query(params ...any) Request[T]                                                              // Set query parameters for the request
	Get(ctx context.Context, u string, params ...any) (T, error)                                 // Execute a GET request
	Post(ctx context.Context, u string, params any, attachments ...MultipartFormData) (T, error) // Execute a POST request
	PostJSON(ctx context.Context, u string, params any) (T, error)
	//Put(ctx context.Context) (T, error)                 // Execute a PUT request
	//Delete(ctx context.Context) (T, error)              // Execute a DELETE request
}

// ResponderFunc HTTPレスポンスを処理するための関数
//
// Condition はHTTPレスポンス [*http.Response] がこの関数で定義された条件を満たすかどうかを判断します。
// Responder はレスポンスの実際の処理を担当し、型Tの値を生成して返します。
type ResponderFunc[T any] struct {
	Condition func(res *http.Response) bool
	Responder func(res *http.Response) (T, error)
}

// NewRequestFunc Request[T]を生成する関数
//
// レスポンダー関数を指定してHTTPレスポンスボディを処理する方法を決定します。
func NewRequestFunc[T *U, U any](responders []*ResponderFunc[T]) Request[T] {
	return &requestImpl[T]{
		responders: responders,
	}
}

// NewRequestSliceFunc Request[T]を生成する関数(Tをスライス型にする時専用)
//
// レスポンダー関数を指定してHTTPレスポンスボディを処理する方法を決定します。
// Tはスライス型である必要があります。
// Deprecated: NewRequestFunc に統合予定。
func NewRequestSliceFunc[T ~[]E, E any](responders []*ResponderFunc[T]) Request[T] {
	return &requestImpl[T]{
		responders: responders,
	}
}

// requestImpl HTTPリクエストの実装
//
// Tはレスポンスの型を表します。
type requestImpl[T any] struct {
	method     string
	url        *url.URL
	values     url.Values
	headers    http.Header
	body       io.Reader
	responders []*ResponderFunc[T]

	// HttpClient HTTPクライアントを返すメソッド
	httpClient *http.Client
}

func (r *requestImpl[T]) HTTPClient(c *http.Client) Request[T] {
	r.httpClient = c
	return r
}

// Method HTTPリクエストのメソッドを設定
func (r *requestImpl[T]) Method(m string) Request[T] {
	r.method = m
	return r
}

// Headers HTTPリクエストのヘッダーを設定
func (r *requestImpl[T]) Headers(headers any) Request[T] {
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
		if r.headers == nil {
			r.headers = make(http.Header)
		}
		for key, value := range h {
			r.headers.Add(key, value)
		}
	} else {
		panic("invalid header type, expected http.Headers")
	}
	return r
}

// Header HTTPリクエストのヘッダーを設定(key, valueによるstringペア)
func (r *requestImpl[T]) Header(key, value string) Request[T] {
	if r.headers == nil {
		r.headers = make(http.Header)
	}
	r.headers.Add(key, value)
	return r
}

// Query HTTPリクエストのクエリパラメータを設定
func (r *requestImpl[T]) Query(params ...any) Request[T] {
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
func (r *requestImpl[T]) Get(ctx context.Context, u string, params ...any) (T, error) {
	var zero T

	r.method = http.MethodGet

	if len(params) > 0 {
		r.Query(params...)
	}

	var err error
	r.url, err = url.Parse(u)
	if err != nil {
		return zero, err
	}

	req, err := r.build(ctx)
	if err != nil {
		return zero, err
	}
	return do[T](r.httpClient, req, r.responders)
}

// Post HTTP POSTリクエストを実行
func (r *requestImpl[T]) Post(ctx context.Context, u string, params any, attachments ...MultipartFormData) (T, error) {
	var zero T

	r.method = http.MethodPost

	v := url.Values{}
	if err := schema.NewEncoder().Encode(params, v); err != nil {
		return zero, err
	}

	if len(attachments) == 0 {
		if r.headers == nil {
			r.headers = make(http.Header)
		}
		ve := v.Encode()
		r.headers.Set("Content-Type", "application/x-www-form-urlencoded")
		r.headers.Set("Cache-Control", "no-cache")
		r.headers.Set("Content-Length", strconv.Itoa(len(ve)))
		r.body = strings.NewReader(ve)
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

		if r.headers == nil {
			r.headers = make(http.Header)
		}
		r.headers.Set("Content-Type", mw.FormDataContentType())
		r.headers.Set("Cache-Control", "no-cache")
		r.body = &buf
	}

	var err error
	r.url, err = url.Parse(u)
	if err != nil {
		return zero, err
	}

	req, err := r.build(ctx)
	if err != nil {
		return zero, err
	}
	return do[T](r.httpClient, req, r.responders)
}

// PostJSON JSONエンコードされたデータをリクエストボディに含むHTTP POSTリクエストを実行
func (r *requestImpl[T]) PostJSON(ctx context.Context, u string, params any) (T, error) {
	var zero T

	r.method = http.MethodPost

	var buf bytes.Buffer
	var err error
	if err = json.NewEncoder(&buf).Encode(params); err != nil {
		return zero, err
	}

	if r.headers == nil {
		r.headers = make(http.Header)
	}
	r.headers.Set("Content-Type", "application/json")
	r.headers.Set("Cache-Control", "no-cache")
	r.body = &buf

	r.url, err = url.Parse(u)
	if err != nil {
		return zero, err
	}

	req, err := r.build(ctx)
	if err != nil {
		return zero, err
	}
	return do[T](r.httpClient, req, r.responders)
}

// Put HTTP PUTリクエストを実行
//
// note: このメソッドは未実装です。
func (r *requestImpl[T]) Put(ctx context.Context) (T, error) {
	panic("implement me")
}

// Delete HTTP DELETEリクエストを実行
//
// note: このメソッドは未実装です。
func (r *requestImpl[T]) Delete(ctx context.Context) (T, error) {
	panic("implement me")
}

// parseURL URLを分解して保持
func (r *requestImpl[T]) parseURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	r.url = u

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
func (r *requestImpl[T]) build(ctx context.Context) (*http.Request, error) {
	r.url.RawQuery = r.values.Encode()
	req, err := http.NewRequestWithContext(ctx, r.method, r.url.String(), r.body)
	if err != nil {
		return nil, err
	}

	req.Header = r.headers
	return req, nil
}
