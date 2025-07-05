package httpc

import "mime/multipart"

// MultipartFormData multipart/form-data形式のPOSTリクエストに添付するためのインターフェイス
//
// multipart/form-data形式でHTTPリクエストに添付ファイルを追加するためのメソッドを定義します。
type MultipartFormData interface {
	AttachTo(mw *multipart.Writer) error
}
