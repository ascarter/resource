package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type testCase struct {
	Path     string
	Method   string
	Status   int
	Matches  map[string]string
	Body     url.Values
	Expected string
}

func (tc *testCase) String() string {
	return fmt.Sprintf("%s %s", tc.Method, tc.Path)
}

func (tc *testCase) Run(t *testing.T, h http.Handler) {
	// POST body
	body, err := json.Marshal(tc.Body)
	if err != nil {
		t.Fatal(err)
	}

	req, err := http.NewRequest(tc.Method, tc.Path, bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	tc.Verify(t, w)
}

func (tc *testCase) Verify(t *testing.T, w *httptest.ResponseRecorder) {
	if w.Code != tc.Status {
		t.Errorf("handler status code %v, expected %v", w.Code, tc.Status)
	}

	if w.Code == http.StatusOK {
		var data testResponse
		err := json.Unmarshal(w.Body.Bytes(), &data)
		if err != nil {
			t.Fatalf("%v: %s", err, w.Body.String())
		}

		if data.Method != tc.Method {
			t.Errorf("handler path %s method %s, expected %s", tc.Path, data.Method, tc.Method)
		}

		if data.Path != tc.Path {
			t.Errorf("handler path %s, expected %s", data.Path, tc.Path)
		}

		if tc.Matches != nil {
			if len(data.Matches) != len(tc.Matches) {
				t.Errorf("handler regexp matches %+v, expected %+v", data.Matches, tc.Matches)
			} else {
				for k, v := range data.Matches {
					mv, ok := tc.Matches[k]
					if !ok {
						t.Errorf("handler match %s param not found", k)
					}
					if mv != v {
						t.Errorf("handler match %s %v, expected %v", k, v, mv)
					}
				}
			}
		}
	}
}

type testResponse struct {
	Method  string
	Path    string
	Matches map[string]string
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	response := testResponse{Method: r.Method, Path: r.URL.Path}

	matches, ok := FromContext(r.Context())
	if ok {
		response.Matches = matches
	}

	output, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}

type testResource struct{}

// GET /resource
func (tr *testResource) Index(w http.ResponseWriter, r *http.Request) {
	testHandler(w, r)
}

// POST /resource
func (tr *testResource) Create(w http.ResponseWriter, r *http.Request) {
	testHandler(w, r)
}

// GET /resource/:id
func (tr *testResource) Show(w http.ResponseWriter, r *http.Request) {
	testHandler(w, r)
}

// PUT /resource/:id
func (tr *testResource) Update(w http.ResponseWriter, r *http.Request) {
	testHandler(w, r)
}

// DELETE /resource/:id
func (tr *testResource) Destroy(w http.ResponseWriter, r *http.Request) {
	testHandler(w, r)
}

func TestRouter(t *testing.T) {
	t.Parallel()

	expected := "testrequest"

	h := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "%s: %s", name, expected)
		})
	}

	router := NewRouter()
	router.Handle("/fn1", h("fn1"))
	router.Handle("/fn2", h("fn2"))

	server := httptest.NewServer(router)
	defer server.Close()

	testcases := []testCase{
		{
			Path:     "/fn1",
			Method:   http.MethodGet,
			Status:   http.StatusOK,
			Expected: "fn1: testrequest",
		},
		{
			Path:     "/fn2",
			Method:   http.MethodGet,
			Status:   http.StatusOK,
			Expected: "fn2: testrequest",
		},
		{
			Path:     "/fn1/foo/..",
			Method:   http.MethodGet,
			Status:   http.StatusOK,
			Expected: "fn1: testrequest",
		},
		{
			Path:     "/./fn1/.",
			Method:   http.MethodGet,
			Status:   http.StatusOK,
			Expected: "fn1: testrequest",
		},
		{
			Path:   "/foo",
			Method: http.MethodGet,
			Status: http.StatusNotFound,
		},
	}

	for _, tc := range testcases {
		res, err := http.Get(server.URL + tc.Path)
		if err != nil {
			t.Fatal(err)
		}

		if res.StatusCode != tc.Status {
			t.Errorf("%s: handler status code %v, expected %v", tc.String(), res.StatusCode, tc.Status)
		}

		if res.StatusCode == http.StatusOK {
			body, err := ioutil.ReadAll(res.Body)
			defer res.Body.Close()

			if err != nil {
				t.Fatal(err)
			}

			bodyText := string(body)
			if bodyText != tc.Expected {
				t.Errorf("%s: %q != %q", tc.String(), bodyText, expected)
			}
		}
	}
}

func TestResourceRouter(t *testing.T) {
	testcases := []testCase{
		{
			Path:    "/posts",
			Method:  http.MethodGet,
			Status:  http.StatusOK,
			Matches: map[string]string{},
		},
		{
			Path:    "/posts/1",
			Method:  http.MethodGet,
			Status:  http.StatusOK,
			Matches: map[string]string{"id": "1"},
		},
		{
			Path:    "/posts",
			Method:  http.MethodPost,
			Status:  http.StatusOK,
			Matches: map[string]string{},
			Body: url.Values{
				"title": {"sample post"},
				"body":  {"post body"},
			},
		},
		{
			Path:    "/posts/1",
			Method:  http.MethodPut,
			Status:  http.StatusOK,
			Matches: map[string]string{"id": "1"},
			Body: url.Values{
				"body": {"updated post body"},
			},
		},
		{
			Path:    "/posts/1",
			Method:  http.MethodDelete,
			Status:  http.StatusOK,
			Matches: map[string]string{"id": "1"},
		},
		{
			Path:    "/posts/23/comments",
			Method:  http.MethodGet,
			Status:  http.StatusOK,
			Matches: map[string]string{"id": "23"},
		},
		{
			Path:    "/posts/23/foo",
			Method:  http.MethodGet,
			Status:  http.StatusOK,
			Matches: map[string]string{"id": "23"},
		},
		{
			Path:   "/foo",
			Method: http.MethodGet,
			Status: http.StatusNotFound,
		},
	}

	router := NewRouter()
	router.HandleResource(`/posts/`, &testResource{})

	for _, tc := range testcases {
		t.Run(tc.String(), func(t *testing.T) {
			tc := tc
			tc.Run(t, router)
		})
	}
}

func TestStaticRoutes(t *testing.T) {
	testcases := []testCase{
		{
			Path:   "/posts",
			Method: http.MethodGet,
			Status: http.StatusOK,
		},
		{
			Path:   "/posts/comments",
			Method: http.MethodGet,
			Status: http.StatusOK,
		},
		{
			Path:   "/posts/foo",
			Method: http.MethodGet,
			Status: http.StatusNotFound,
		},
		{
			Path:   "/foo/bar",
			Method: http.MethodGet,
			Status: http.StatusNotFound,
		},
	}

	h := NewRouter()
	h.HandleFunc(`/posts`, testHandler)
	h.HandleFunc(`/posts/comments`, testHandler)

	for _, tc := range testcases {
		t.Run(tc.String(), func(t *testing.T) {
			tc := tc
			tc.Run(t, h)
		})
	}
}

func TestResourceHandler(t *testing.T) {
	testcases := []testCase{
		{
			Path:    "/posts",
			Method:  http.MethodGet,
			Status:  http.StatusOK,
			Matches: map[string]string{},
		},
		{
			Path:    "/posts/1",
			Method:  http.MethodGet,
			Status:  http.StatusOK,
			Matches: map[string]string{"id": "1"},
		},
		{
			Path:    "/posts",
			Method:  http.MethodPost,
			Status:  http.StatusOK,
			Matches: map[string]string{},
			Body: url.Values{
				"title": {"sample post"},
				"body":  {"post body"},
			},
		},
		{
			Path:    "/posts/1",
			Method:  http.MethodPut,
			Status:  http.StatusOK,
			Matches: map[string]string{"id": "1"},
			Body: url.Values{
				"body": {"updated post body"},
			},
		},
		{
			Path:    "/posts/1",
			Method:  http.MethodDelete,
			Status:  http.StatusOK,
			Matches: map[string]string{"id": "1"},
		},
		{
			Path:    "/posts/23/comments",
			Method:  http.MethodGet,
			Status:  http.StatusOK,
			Matches: map[string]string{"id": "23"},
		},
		{
			Path:    "/posts/23/foo",
			Method:  http.MethodGet,
			Status:  http.StatusOK,
			Matches: map[string]string{"id": "23"},
		},
		{
			Path:   "/foo",
			Method: http.MethodGet,
			Status: http.StatusNotFound,
		},
	}

	h := NewResourceHandler(`/posts`, &testResource{})

	for _, tc := range testcases {
		t.Run(tc.String(), func(t *testing.T) {
			tc := tc
			tc.Run(t, h)
		})
	}
}
