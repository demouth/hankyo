package hankyo

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBasic(t *testing.T) {
	h := New()
	h.Get("/users/:id", func(c *Context) {
		if p := c.Param("id"); p != "1" {
			t.Errorf("Response code should be Bad request, was: %s", p)
		}
	})
	h.Post("/users/:id", func(c *Context) {
		if p := c.Param("id"); p != "2" {
			t.Errorf("Response code should be Bad request, was: %s", p)
		}
	})
	h.Delete("/users/:id", func(c *Context) {
		if p := c.Param("id"); p != "3" {
			t.Errorf("Response code should be Bad request, was: %s", p)
		}
	})
	h.Put("/users/:id", func(c *Context) {
		if p := c.Param("id"); p != "4" {
			t.Errorf("Response code should be Bad request, was: %s", p)
		}
	})

	r := request(h, "GET", "/users/1")
	if r.Code != 200 {
		t.Errorf("Response code should be Bad request, was: %d", r.Code)
	}
	r = request(h, "POST", "/users/2")
	if r.Code != 200 {
		t.Errorf("Response code should be Bad request, was: %d", r.Code)
	}
	r = request(h, "DELETE", "/users/3")
	if r.Code != 200 {
		t.Errorf("Response code should be Bad request, was: %d", r.Code)
	}
	r = request(h, "PUT", "/users/4")
	if r.Code != 200 {
		t.Errorf("Response code should be Bad request, was: %d", r.Code)
	}
}

func TestGet(t *testing.T) {
	b := New()
	b.Get("/users/a/b", func(c *Context) {
	})
	b.Get("/users", func(c *Context) {
		c.JSON(200, struct {
			A string
		}{
			A: "1",
		})
	})
	b.Get("/users/a/:c", func(c *Context) {
	})
	r, _ := http.NewRequest("GET", "/users", nil)
	w := httptest.NewRecorder()
	b.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Errorf("status code should be 200, found %d", w.Code)
	}
	if w.Body.String() != "{\"A\":\"1\"}\n" {
		t.Errorf("wrong body, found %#v", w.Body.String())
	}
}

func TestMiddleware(t *testing.T) {
	h := New()
	b := new(bytes.Buffer)

	h.Use(func(c *Context) {
		b.WriteString("a")
	})

	h.Use(func(c *Context) {
		b.WriteString("b")
	})

	h.Get("/hello", func(c *Context) {
		c.String(200, "world")
	})

	w := request(h, "GET", "/hello")
	if b.String() != "ab" {
		t.Errorf("buffer should be ab, found %s", b.String())
	}
	if w.Body.String() != "world" {
		t.Error("body should be world")
	}
}

func request(h *Hankyo, method, path string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}
