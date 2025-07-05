package form

import (
	"bytes"
	"io"
	"mime/multipart"
	"os"
)

// MultipartFormData multipart/form-data形式のPOSTリクエストに添付するための構造体
type MultipartFormData struct {
	fieldName string
	fileName  string
	reader    io.Reader
	closer    io.Closer
}

// Bytes バイトスライスをmultipart/form-data形式で添付するための関数
func Bytes(fieldName, fileName string, data []byte) *MultipartFormData {
	return &MultipartFormData{
		fieldName: fieldName,
		fileName:  fileName,
		reader:    bytes.NewReader(data),
	}
}

// File ファイルをmultipart/form-data形式で添付するための関数
func File(fieldName, fileName string) *MultipartFormData {
	r, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	return &MultipartFormData{
		fieldName: fieldName,
		fileName:  fileName,
		reader:    r,
		closer:    r,
	}
}

// AttachTo multipart/form-data形式でPOSTリクエストにデータを添付
//
// 当該POSTリクエストに紐づけられた multipart.Writer に対して、自らが保持するデータを書き込みます。
func (d *MultipartFormData) AttachTo(mw *multipart.Writer) error {
	part, err := mw.CreateFormFile(d.fieldName, d.fileName)
	if err != nil {
		return err
	}
	if d.closer != nil {
		defer func() {
			_ = d.closer.Close()
		}()
	}

	_, err = io.Copy(part, d.reader)
	return err
}
