package resource

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
)

type ctxKey int

const paramsKey ctxKey = ctxKey(0)

// RouteParams is a map of param names to values that are matched with a pattern to a path.
// Param ID's are expected to be unique.
type RouteParams map[string]string

// NewContext creates a context with matched request params
func NewContext(ctx context.Context, r *http.Request, pattern string) context.Context {
	urlParts := strings.Split(r.URL.Path, "/")
	patParts := strings.Split(pattern, "/")

	params := RouteParams{}
	for i, p := range patParts {
		if len(urlParts) <= i {
			break
		}
		u := urlParts[i]
		if len(p) > 0 && p[0] == ':' {
			params[p[1:]] = u
		}
	}

	return context.WithValue(ctx, paramsKey, params)
}

// FromContext returns the matched params from context
func FromContext(ctx context.Context) (RouteParams, bool) {
	params, ok := ctx.Value(paramsKey).(RouteParams)
	return params, ok
}

// ReadJSON reads data from request body to the interface provided.
func ReadJSON(r *http.Request, data interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, data); err != nil {
		return err
	}

	return nil
}

// WriteJSON writes data as JSON to the output writer.
// Data expected to be able to be marshaled to JSON.
func WriteJSON(w http.ResponseWriter, data interface{}) error {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
	return nil
}

// A Resource implements handlers for REST routes.
//
// Resources map specific HTTP methods to route patterns. Each method in the
// interface performs a specific operation on the resource. Each action generally
// corresponds to a CRUD operation typically in a database.
//
// For a `photos` resource:
//	Method     HTTP Method     Path            Used For
//	------     -----------     -----------     --------------------------
//	Index      GET             /photos         display list of all photos
//	Create     POST            /photos         create a new photo
//	Show       GET             /photos/:id     display specific photo
//	Update     PUT             /photos/:id     update a specific photo
//	Destroy    DELETE          /photos/:id     delete a specific photo
type Resource interface {
	Index(http.ResponseWriter, *http.Request)
	Create(http.ResponseWriter, *http.Request)
	Show(http.ResponseWriter, *http.Request)
	Update(http.ResponseWriter, *http.Request)
	Destroy(http.ResponseWriter, *http.Request)
}

// trimPath drops trailing `/`
func trimPath(p string) string {
	n := len(p)
	if p[n-1] == '/' {
		p = p[:n-1]
	}
	return p
}

// NewResourceHandler returns a new resource handler instance.
func NewResourceHandler(prefix string, resource Resource) http.Handler {
	n := len(prefix)
	if prefix[n-1] == '/' {
		// Drop trailing `/`
		prefix = prefix[:n-1]
	}

	return &resourceHandler{resource: resource, prefix: trimPath(prefix)}
}

// A resourceHandler routes requests to a Resource.
// It is used for representing a REST endpoint.
type resourceHandler struct {
	resource Resource
	prefix   string
}

// ServeHTTP dispatches request to Resource
func (h *resourceHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := trimPath(r.URL.Path)

	// Verify resource prefix
	if !strings.HasPrefix(p, h.prefix) {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
	case http.MethodGet:
		s := strings.TrimPrefix(p, h.prefix)
		if len(s) > 0 {
			ctx := NewContext(r.Context(), r, path.Join(h.prefix, ":id"))
			h.resource.Show(w, r.WithContext(ctx))
		} else {
			h.resource.Index(w, r)
		}
	case http.MethodPost:
		h.resource.Create(w, r)
	case http.MethodPut:
		ctx := NewContext(r.Context(), r, path.Join(h.prefix, ":id"))
		h.resource.Update(w, r.WithContext(ctx))
	case http.MethodDelete:
		ctx := NewContext(r.Context(), r, path.Join(h.prefix, ":id"))
		h.resource.Destroy(w, r.WithContext(ctx))
	}

	return
}

// A Router dispatches resource paths to resources.
// Router is compatible with http.ServeMux and can be used as a drop-in replacement.
type Router struct {
	mux *http.ServeMux
}

// NewRouter returns a new Router instance.
func NewRouter() *Router {
	return &Router{mux: http.NewServeMux()}
}

// Handle registers a handler for a pattern.
func (router *Router) Handle(pattern string, handler http.Handler) {
	router.mux.Handle(pattern, handler)
}

// HandleFunc registers a handler function for a pattern.
func (router *Router) HandleFunc(pattern string, fn http.HandlerFunc) {
	router.mux.HandleFunc(pattern, fn)
}

// HandleResource registers a resource as a handler for a pattern.
func (router *Router) HandleResource(pattern string, resource Resource) {
	p := trimPath(pattern)
	h := NewResourceHandler(p, resource)
	router.mux.Handle(p, h)
	router.mux.Handle(p+"/", h)
}

func (router *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	router.mux.ServeHTTP(w, r)
}
