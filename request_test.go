package httpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/unvurn/httpc"
	"github.com/unvurn/httpc/form"
)

type httpbinResponse struct {
	Args    map[string]string `json:"args"`
	Headers map[string]string `json:"headers"`
	Origin  string            `json:"origin"`
	URL     string            `json:"url"`
}

type httpbinGetResponse[T any] struct {
	Args    T                 `json:"args"`
	Headers map[string]string `json:"headers"`
	Origin  string            `json:"origin"`
	URL     string            `json:"url"`
}

type httpbinPostResponse struct {
	Args    map[string]string `json:"args"`
	Data    string            `json:"data"`
	Headers map[string]string `json:"headers"`
	Origin  string            `json:"origin"`
	URL     string            `json:"url"`
}

type httpbinPostFormResponse[T any] struct {
	httpbinPostResponse
	Form  T                 `json:"form"`
	Files map[string]string `json:"files"`
}

type params struct {
	Name         string  `schema:"name" json:"name"`
	Age          int     `schema:"age" json:"age"`
	Scores       []int   `schema:"scores" json:"scores"`
	Description  string  `schema:"description" json:"description"`
	Description2 string  `schema:"description_2,omitempty" json:"description_2,omitempty"`
	ExtraNote    *string `schema:"extra_note" json:"extra_note"`
	ExtraNote2   *string `schema:"extra_note_2,omitempty" json:"extra_note_2,omitempty"`
}

func (p *params) UnmarshalJSON(b []byte) error {
	var temp struct {
		Name         string   `json:"name"`
		Age          string   `json:"age"`
		Scores       []string `json:"scores"`
		Description  string   `json:"description"`
		Description2 string   `json:"description_2,omitempty"`
		ExtraNote    *string  `json:"extra_note"`
		ExtraNote2   *string  `json:"extra_note_2,omitempty"`
	}

	err := json.Unmarshal(b, &temp)
	if err != nil {
		return err
	}

	p.Name = temp.Name
	p.Age, err = strconv.Atoi(temp.Age)
	if err != nil {
		return err
	}
	for _, score := range temp.Scores {
		s, err := strconv.Atoi(score)
		if err != nil {
			return err
		}
		p.Scores = append(p.Scores, s)
	}
	p.Description = temp.Description
	p.Description2 = temp.Description2
	if temp.ExtraNote == nil || *temp.ExtraNote == "null" {
		p.ExtraNote = nil
	} else {
		p.ExtraNote = temp.ExtraNote
	}
	if temp.ExtraNote2 == nil || *temp.ExtraNote2 == "null" {
		p.ExtraNote2 = nil
	} else {
		p.ExtraNote2 = temp.ExtraNote2
	}
	return nil
}

const httpbinEndpoint = "https://httpbin.org"

/////

func TestHttpbin_Get(t *testing.T) {
	u, _ := url.JoinPath(httpbinEndpoint, "get")
	resp, err := httpc.NewRequest[[]byte]().Get(context.Background(), u)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	r := httpbinResponse{}
	err = json.Unmarshal(resp, &r)
	assert.NoError(t, err)
}

func TestHttpbin_GetQuery(t *testing.T) {
	u, _ := url.JoinPath(httpbinEndpoint, "get")
	p := params{
		Name:   "John Doe",
		Age:    25,
		Scores: []int{100, 90, 80},
	}
	b, err := httpc.NewRequest[[]byte]().Get(context.Background(), u, p)
	assert.NoError(t, err)

	var resp httpbinGetResponse[params]
	err = json.Unmarshal(b, &resp)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "John Doe", resp.Args.Name)
	assert.Equal(t, 25, resp.Args.Age)
	assert.Equal(t, []int{100, 90, 80}, resp.Args.Scores)
	assert.Empty(t, resp.Args.Description)
	assert.Empty(t, resp.Args.Description2)
	assert.Nil(t, resp.Args.ExtraNote)
	assert.Nil(t, resp.Args.ExtraNote2)
	assert.Len(t, resp.Args.Scores, 3)
	assert.NotEmpty(t, resp.Headers)
	assert.NotEmpty(t, resp.Origin)
	u2, _ := url.Parse(resp.URL)
	u2.RawQuery = ""
	assert.Equal(t, u, u2.String())
	assert.Contains(t, resp.Headers["User-Agent"], "Go-http-client/2.0")
}

func TestHttpbin_GetQuery_NullableString(t *testing.T) {
	u, _ := url.JoinPath(httpbinEndpoint, "get")
	empty := ""
	p := params{
		Name:       "John Doe",
		Age:        25,
		Scores:     []int{100, 90, 80},
		ExtraNote:  &empty,
		ExtraNote2: &empty,
	}
	b, err := httpc.NewRequest[[]byte]().Get(context.Background(), u, p)
	assert.NoError(t, err)

	var resp httpbinGetResponse[params]
	err = json.Unmarshal(b, &resp)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "John Doe", resp.Args.Name)
	assert.Equal(t, 25, resp.Args.Age)
	assert.Equal(t, []int{100, 90, 80}, resp.Args.Scores)
	assert.Empty(t, resp.Args.Description)
	assert.Empty(t, resp.Args.Description2)
	assert.Empty(t, resp.Args.ExtraNote)
	assert.Empty(t, resp.Args.ExtraNote2)
	assert.Len(t, resp.Args.Scores, 3)
	assert.NotEmpty(t, resp.Headers)
	assert.NotEmpty(t, resp.Origin)
	u2, _ := url.Parse(resp.URL)
	u2.RawQuery = ""
	assert.Equal(t, u, u2.String())
	assert.Contains(t, resp.Headers["User-Agent"], "Go-http-client/2.0")
}

func TestHttpbin_Get_NotFound(t *testing.T) {
	u, _ := url.JoinPath(httpbinEndpoint, "status", "404")
	resp, err := httpc.NewRequest[[]byte]().Get(context.Background(), u)
	assert.Error(t, err)
	var e *httpc.Error
	if errors.As(err, &e) {
		assert.Equal(t, http.StatusNotFound, e.StatusCode())
	}
	assert.Zero(t, resp)
}

/////

func TestHttpbin_PostForm(t *testing.T) {
	u, _ := url.JoinPath(httpbinEndpoint, "post")
	p := params{
		Name:   "Jane Doe",
		Age:    25,
		Scores: []int{100, 90, 80},
	}
	b, err := httpc.NewRequest[[]byte]().Post(context.Background(), u, p)
	assert.NoError(t, err)

	var resp httpbinPostFormResponse[params]
	err = json.Unmarshal(b, &resp)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Jane Doe", resp.Form.Name)
	assert.Equal(t, 25, resp.Form.Age)
	assert.Equal(t, []int{100, 90, 80}, resp.Form.Scores)
	assert.Len(t, resp.Form.Scores, 3)
	assert.NotEmpty(t, resp.Headers)
	assert.Equal(t, "80", resp.Headers["Content-Length"])
	assert.NotEmpty(t, resp.Origin)
	assert.Equal(t, u, resp.URL)
	assert.Contains(t, resp.Headers["User-Agent"], "Go-http-client/2.0")
}

func TestHttpbin_PostFileUpload(t *testing.T) {
	u, _ := url.JoinPath(httpbinEndpoint, "post")
	p := params{
		Name:   "Jane Doe",
		Age:    25,
		Scores: []int{100, 90, 80},
	}
	b, err := httpc.NewRequest[[]byte]().Post(context.Background(), u, p,
		form.Bytes("data1", "data1.txt", []byte("This is data1 content.")),
		form.File("data2", "testdata/samples/dummy.pdf"))
	assert.NoError(t, err)

	var resp httpbinPostFormResponse[params]
	err = json.Unmarshal(b, &resp)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "Jane Doe", resp.Form.Name)
	assert.Equal(t, 25, resp.Form.Age)
	assert.Len(t, resp.Form.Scores, 3)
	assert.Equal(t, []int{100, 90, 80}, resp.Form.Scores)
	assert.Len(t, resp.Files, 2)
	assert.Equal(t, "This is data1 content.", resp.Files["data1"])
	assert.Len(t, resp.Files["data2"], 17725)
	assert.NotEmpty(t, resp.Headers)
	assert.NotEmpty(t, resp.Origin)
	assert.Equal(t, u, resp.URL)
	assert.Contains(t, resp.Headers["User-Agent"], "Go-http-client/2.0")
}

/////

func TestHttpbin_BasicAuth(t *testing.T) {
	u, _ := url.JoinPath(httpbinEndpoint, "basic-auth", "user", "password")
	resp, err := httpc.NewRequest[[]byte]().BasicAuth("user", "password").Get(context.Background(), u)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestHttpbin_BasicAuth_Unauthorized(t *testing.T) {
	u, _ := url.JoinPath(httpbinEndpoint, "basic-auth", "user", "password")
	resp, err := httpc.NewRequest[[]byte]().Get(context.Background(), u)
	assert.Error(t, err)
	var e *httpc.Error
	assert.ErrorAs(t, err, &e)
	assert.Equal(t, http.StatusUnauthorized, e.StatusCode())
	assert.Nil(t, resp)
}
