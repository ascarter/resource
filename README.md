# resource
[![GoDoc](https://godoc.org/github.com/ascarter/resource?status.svg)](http://godoc.org/github.com/ascarter/resource) [![Go Report Card](https://goreportcard.com/badge/github.com/ascarter/resource)](https://goreportcard.com/report/github.com/ascarter/resource)

Resource is a REST handler library for Go. It provides `net/http` compatible extensions for routing to a resource handler.

# Usage

This is an example of a simple web app using conductor. For more detailed example,
see `example_test.go` file.

```go

package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/ascarter/resource"
)

type Employee struct {
	Name string `json:"name"`
}

type EmployeeResource struct {
	mu        sync.RWMutex
	lastID    int
	Employees map[string]Employee `json:"employees"`
}

// GET /employees
func (er *EmployeeResource) Index(w http.ResponseWriter, r *http.Request) {
	// List of employees
	if err := resource.WriteJSON(w, er.Employees); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	return
}

// POST /employees
func (er *EmployeeResource) Create(w http.ResponseWriter, r *http.Request) {
	// Read employee object
	var v Employee
	if err := resource.ReadJSON(r, &v); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	er.mu.Lock()
	defer er.mu.Unlock()

	// Add employee
	id := er.lastID + 1
	er.Employees[strconv.Itoa(id)] = v
	er.lastID++

	// Return ID of new object
	fmt.Fprintf(w, "%d", id)
}

// GET /employees/:id
func (er *EmployeeResource) Show(w http.ResponseWriter, r *http.Request) {
	params, ok := resource.FromContext(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	id := params["id"]

	er.mu.RLock()
	defer er.mu.RUnlock()

	// Find existing employee
	e, ok := er.Employees[id]
	if !ok {
		http.NotFound(w, r)
	}

	// Return employee
	if err := resource.WriteJSON(w, e); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	return
}

// PUT /employees/:id
func (er *EmployeeResource) Update(w http.ResponseWriter, r *http.Request) {
	params, ok := resource.FromContext(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	id := params["id"]

	er.mu.Lock()
	defer er.mu.Unlock()

	// Find existing employee
	if _, ok := er.Employees[id]; !ok {
		http.NotFound(w, r)
	}

	// Read updated employee object
	var v Employee
	if err := resource.ReadJSON(r, &v); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	er.Employees[id] = v

	w.WriteHeader(http.StatusOK)
	return
}

// DELETE /employees/:id
func (er *EmployeeResource) Destroy(w http.ResponseWriter, r *http.Request) {
	params, ok := resource.FromContext(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	id := params["id"]

	er.mu.Lock()
	defer er.mu.Unlock()

	// Find existing employee
	if _, ok := er.Employees[id]; !ok {
		http.NotFound(w, r)
	}

	delete(er.Employees, id)
	w.WriteHeader(http.StatusOK)
	return
}

func Example() {
	mux := resource.NewRouter()
	mux.HandleResource(`/posts`, &EmployeeResource{lastID: 0})
	log.Fatal(http.ListenAndServe(":8080", mux))
}

```

# Routing

Resource provides a routing component `Router` which can help setup routes for resource endpoints. It is a very simple router that uses `http.ServeMux`. It can be used as a drop-in replacement since it uses the same `http.ServeMux` internally. The primary feature is adding a `HandleResource` func that will conveniently setup a `ResourceHandler` for a path prefix.

The pattern supplied to `HandleResource` should be the collection path for the resource:

```go

mux := resource.NewRouter()
mux.HandleResource(`/api/posts`, &PostsResource{})
```

Router is `http.Handler` compatible so it can be used in conjunction with other middleware or routing libraries.

# Resource Interface

A Resource implements handlers for REST routes. Resources map specific HTTP methods to route patterns. Each method in the interface performs a specific operation on the resource. Each action generally corresponds to a CRUD operation typically in a database.

For a `photos` resource:

Method | HTTP Method | Path | Description
------ | ----------- | ---- | ------------
Index | GET | /photos | display list of all photos
Create | POST | /photos | create a new photo
Show  | GET | /photos/:id | display specific photo
Update | PUT | /photos/:id | update a specific photo
Destroy | DELETE | /photos/:id | delete a specific photo
