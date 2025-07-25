package httpc

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/schema"
	. "github.com/unvurn/core"
)

type EncoderFunc func(any) (io.Reader, error)
type DecoderFunc[T any] func([]byte) (T, error)
type ErrorHandlerFunc func(*http.Response, []byte) error

func NewRequest[T any]() *Request[T] {
	return NewRequestFunc[T]()
}

// NewRequestFunc Request[T]を生成する関数
//
// デコーダーを指定してHTTPレスポンスボディを処理する方法を決定します。
func NewRequestFunc[T any]() *Request[T] {
	return &Request[T]{
		headers:             http.Header{},
		decoders:            map[string]DecoderFunc[T]{},
		errorHandlers:       map[string]ErrorHandlerFunc{},
		defaultErrorHandler: newError,
	}
}

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

	encoderContentType  string
	encoder             EncoderFunc
	decoders            map[string]DecoderFunc[T]
	errorHandlers       map[string]ErrorHandlerFunc
	defaultErrorHandler ErrorHandlerFunc

	// HttpClient HTTPクライアントを返すメソッド
	httpClient *http.Client
}

func (r *Request[T]) Encoder(contentType string, encoder EncoderFunc) *Request[T] {
	r.encoderContentType = contentType
	r.encoder = encoder
	return r
}

func (r *Request[T]) Decoder(contentType string, decoder DecoderFunc[T]) *Request[T] {
	r.decoders[contentType] = decoder
	return r
}

func (r *Request[T]) Error(contentType string, errorFunc func(*http.Response, []byte) error) *Request[T] {
	r.errorHandlers[contentType] = errorFunc
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
		panic("invalid header type, expected http.Headers or map[string]string")
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
	var v T

	result, err := r.TryGet(ctx, u, params...)
	if err != nil {
		// as zero value
		return v, err
	}
	err = result.As(&v)
	return v, err
}

// TryGet HTTP GETリクエストを実行
func (r *Request[T]) TryGet(ctx context.Context, u string, params ...any) (Result, error) {
	return r.TryDoFunc(ctx, http.MethodGet, u, "", func() (io.Reader, error) {
		if len(params) > 0 {
			r.Query(params...)
		}
		return nil, nil
	})
}

// Post HTTP GETリクエストを実行
func (r *Request[T]) Post(ctx context.Context, u string, params any) (T, error) {
	var v T

	result, err := r.TryPost(ctx, u, params)
	if err != nil {
		// as zero value
		return v, err
	}
	err = result.As(&v)
	return v, err
}

func (r *Request[T]) TryPost(ctx context.Context, u string, params any) (Result, error) {
	if r.encoder == nil {
		return nil, ErrNoAvailableEncoder
	}
	return r.TryDoFunc(ctx, http.MethodPost, u, r.encoderContentType, func() (io.Reader, error) {
		return r.encoder(params)
	})
}

// PostForm HTTP POSTリクエストを実行
func (r *Request[T]) PostForm(ctx context.Context, u string, params any, attachments ...MultipartFormData) (T, error) {
	var zero T

	result, err := r.TryPostForm(ctx, u, params, attachments...)
	if err != nil {
		return zero, err
	}
	var v T
	err = result.As(&v)
	if err != nil {
		return zero, err
	}
	return v, nil
}

// TryPostForm HTTP POSTリクエストを実行
//
// 返値として
func (r *Request[T]) TryPostForm(ctx context.Context, u string, params any, attachments ...MultipartFormData) (Result, error) {
	v := url.Values{}
	if err := schema.NewEncoder().Encode(params, v); err != nil {
		return nil, err
	}

	if len(attachments) == 0 {
		ve := v.Encode()
		return r.TryDoFunc(ctx, http.MethodPost, u, "application/x-www-form-urlencoded", func() (io.Reader, error) {
			return strings.NewReader(ve), nil
		})
	} else {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)

		for k, vv := range v {
			for _, v := range vv {
				if err := mw.WriteField(k, v); err != nil {
					return nil, err
				}
			}
		}

		for _, a := range attachments {
			if err := a.AttachTo(mw); err != nil {
				return nil, err
			}
		}

		if err := mw.Close(); err != nil {
			return nil, err
		}

		return r.TryDoFunc(ctx, http.MethodPost, u, mw.FormDataContentType(), func() (io.Reader, error) {
			return &buf, nil
		})
	}
}

// DoFunc JSONエンコードされたデータをリクエストボディに含むHTTP POSTリクエストを実行
func (r *Request[T]) TryDoFunc(ctx context.Context, method, u, contentType string, payloadFunc func() (io.Reader, error)) (Result, error) {
	r.method = method

	body, err := payloadFunc()
	if err != nil {
		return nil, err
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
		return nil, err
	}

	req, err := r.build(ctx)
	if err != nil {
		return nil, err
	}
	return r.do(req)
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

// do HTTPリクエストを実行する
//
// reqはhttp.Requestを表し、respondersはレスポンスを処理するための関数のスライスです。
// レスポンスの型Tを返し、エラーが発生した場合はエラーを返します。
func (r *Request[T]) do(req *http.Request) (Result, error) {
	client := r.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, r.handleErrorResponse(res, b)
	}

	return r.handleResponse(res, b)
}

func (r *Request[T]) handleResponse(res *http.Response, b []byte) (Result, error) {
	ct := contentType(res.Header.Get("Content-Type"))
	decoder := r.decoders[ct]

	return newHttpResult[T](res, b, decoder), nil
}

func (r *Request[T]) handleErrorResponse(res *http.Response, b []byte) error {
	ct := contentType(res.Header.Get("Content-Type"))
	handler := r.errorHandlers[ct]
	if handler == nil {
		handler = r.defaultErrorHandler
	}

	return handler(res, b)
}

func contentType(value string) string {
	if value == "" {
		return ""
	}
	return strings.Split(strings.TrimSpace(value), ";")[0]
}
