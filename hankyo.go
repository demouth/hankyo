package hankyo

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

type (
	Hankyo struct {
		Router                     *router
		handlers                   []HandlerFunc
		maxParam                   byte
		pool                       sync.Pool
		notFoundHandler            HandlerFunc
		methodNotAllowedHandler    HandlerFunc
		internalServerErrorHandler HandlerFunc
	}
	router struct {
		root   *node
		hankyo *Hankyo
	}
	node struct {
		label    byte
		prefix   string
		has      ntype
		handlers []HandlerFunc
		edges    edges
	}
	edges       []*node
	HandlerFunc func(*Context)
	Context     struct {
		Request  *http.Request
		Response *response
		params   Params
		handlers []HandlerFunc
		l        int // Handlers length
		i        int // Current handler index
		hankyo   *Hankyo
	}
	Status   uint16
	response struct {
		http.ResponseWriter
		committed bool
	}
	param struct {
		Name  string
		Value string
	}
	Params []param

	ntype byte
)

const (
	OK Status = iota
	NotFound
	NotAllowed

	snode ntype = iota // Static node
	pnode              // Param node
	anode              // Catch-all node

	MIMEJSON = "application/json"
	MIMEText = "text/plain"

	HeaderContentType = "Content-Type"
)

var (
	MethodMap = map[string]uint8{
		"CONNECT": 1,
		"DELETE":  2,
		"GET":     3,
		"HEAD":    4,
		"OPTIONS": 5,
		"PATCH":   6,
		"POST":    7,
		"PUT":     8,
		"TRACE":   9,
	}
)

//////////////// Hankyo ////////////////

func New() (h *Hankyo) {
	h = &Hankyo{
		maxParam: 5,
		notFoundHandler: func(c *Context) {
			http.Error(c.Response, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		},
		methodNotAllowedHandler: func(c *Context) {
			http.Error(c.Response, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		},
		internalServerErrorHandler: func(c *Context) {
			http.Error(c.Response, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		},
	}
	h.Router = NewRouter(h)
	h.pool.New = func() interface{} {
		return &Context{
			Response: &response{},
			params:   make(Params, h.maxParam),
			i:        -1,
			hankyo:   h,
		}
	}
	return h
}

func (h *Hankyo) Get(path string, hf ...HandlerFunc) {
	h.Handle("GET", path, hf)
}

func (h *Hankyo) Use(hf ...HandlerFunc) {
	h.handlers = append(h.handlers, hf...)
}

func (h *Hankyo) Handle(method, path string, hf []HandlerFunc) {
	hf = append(h.handlers, hf...)
	l := len(hf)
	h.Router.Add(method, path, func(c *Context) {
		c.handlers = hf
		c.l = l
		c.Next()
	})
}

func (h *Hankyo) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	handler, c, s := h.Router.Find(r.Method, r.URL.Path)
	c.reset(rw, r)
	if handler != nil {
		handler(c)
	} else {
		if s == NotFound {
			h.notFoundHandler(c)
		} else if s == NotAllowed {
			h.methodNotAllowedHandler(c)
		}
	}
	h.pool.Put(c)
}

func (h *Hankyo) Run(addr string) {
	log.Fatal(http.ListenAndServe(addr, h))
}

//////////////// Router ////////////////

func NewRouter(h *Hankyo) (r *router) {
	r = &router{
		root: &node{
			prefix:   "",
			handlers: make([]HandlerFunc, len(MethodMap)),
			edges:    edges{},
		},
		hankyo: h,
	}
	return r
}

func (r *router) Add(method, path string, h HandlerFunc) {
	// TODO: Need to implement
	i := 0
	l := len(path)
	for ; i < l; i++ {
		if path[i] == ':' {
			// "/authors/:name/books/:id"
			//           ^           ^
			r.insert(method, path[:i], nil, pnode)
			for ; i < l && path[i] != '/'; i++ {
			}
			if i == l {
				// "/authors/:name/books/:id"
				//                         ^
				r.insert(method, path[:i], h, snode)
				return
			}
			// "/authors/:name/books/:id"
			//                ^
			r.insert(method, path[:i], h, snode)
		} else if path[i] == '*' {
			r.insert(method, path[:i], h, anode)
		}
	}
	r.insert(method, path, h, snode)
}

// Example:
//
//	URL:
//	- GET /users/a/b
//	- GET /users
//	- GET /users/a/c
//	NodeTree:
//	- /users
//		|- /a/
//			|- b
//			|- c
func (r *router) insert(method, path string, h HandlerFunc, has ntype) {
	cn := r.root
	search := path

	for {
		sl := len(search)
		pl := len(cn.prefix)
		l := lcp(search, cn.prefix)

		if l == 0 {
			// At root node

			// URL:
			//   - "/authors/:name"
			// Input:
			//   - method: GET
			//   - path: "/authors/"
			// Root Node - before:
			//   - prefix: ""
			// Root Node - after:
			//   - prefix: "/authors/"

			cn.label = search[0]
			cn.prefix = search
			cn.has = has
			if h != nil {
				cn.handlers[MethodMap[method]] = h
			}
			return
		} else if l < pl {
			// Split the node
			n := newNode(cn.prefix[l:], cn.has, cn.handlers, cn.edges)
			cn.edges = edges{n} // Add to parent

			// Reset parent node
			cn.label = cn.prefix[0]
			cn.prefix = cn.prefix[:l]
			cn.has = snode
			cn.handlers = make([]HandlerFunc, len(MethodMap))

			if l == sl {
				// At parent node
				cn.handlers[MethodMap[method]] = h
			} else {
				// Need to fork a node
				n = newNode(search[l:], has, nil, nil)
				n.handlers[MethodMap[method]] = h
				cn.edges = append(cn.edges, n)
			}
			break
		} else if l < sl {
			search = search[l:]
			e := cn.findEdge(search[0])

			if e == nil {
				n := newNode(search, has, nil, nil)
				if h != nil {
					n.handlers[MethodMap[method]] = h
				}
				cn.edges = append(cn.edges, n)
				break
			} else {
				cn = e
			}
		} else {
			// Node already exists
			if h != nil {
				cn.handlers[MethodMap[method]] = h
			}
			break
		}
	}
}

func (n *node) findEdge(l byte) *node {
	for _, e := range n.edges {
		if e.label == l {
			return e
		}
	}
	return nil
}

func newNode(pfx string, has ntype, h []HandlerFunc, e edges) (n *node) {
	n = &node{
		label:    pfx[0],
		prefix:   pfx,
		has:      has,
		handlers: h,
		edges:    e,
	}
	if h == nil {
		n.handlers = make([]HandlerFunc, len(MethodMap))
	}
	if e == nil {
		n.edges = edges{}
	}
	return
}

func (r *router) Find(method, path string) (handler HandlerFunc, c *Context, s Status) {
	c = r.hankyo.pool.Get().(*Context)

	cn := r.root // Current node
	search := path
	n := 0 // Param count

	for {
		if search == "" || search == cn.prefix {
			// Node found
			h := cn.handlers[MethodMap[method]]
			if h != nil {
				// Handler found
				handler = h
			} else {
				s = NotAllowed
			}
			return
		}

		pl := len(cn.prefix)
		l := lcp(search, cn.prefix)

		if l == pl {
			search = search[l:]
			switch cn.has {
			case pnode:
				cn = cn.edges[0]
				i := 0
				l = len(search)

				for ; i < l && search[i] != '/'; i++ {
				}
				p := c.params[:n+1]
				p[n].Name = cn.prefix[1:]
				p[n].Value = search[:i]
				n++

				search = search[i:]

				if i == l {
					// All params read
					continue
				}
			case anode:
				p := c.params[:n+1]
				p[n].Name = "_name"
				p[n].Value = search
				search = "" // End search
				continue
			}
			e := cn.findEdge(search[0])
			if e == nil {
				// Not found
				s = NotFound
				return
			}
			cn = e
			continue
		} else {
			// Not found
			s = NotFound
			return
		}
	}
}

// Length of longest common prefix
//   - a: ABCDEF
//   - b: ABCx
//   - returns 3
func lcp(a, b string) (i int) {
	max := len(a)
	l := len(b)
	if l < max {
		max = l
	}
	for ; i < max && a[i] == b[i]; i++ {
	}
	return
}

//////////////// Context ////////////////

func (c *Context) Next() {
	c.i++
	i := c.i
	l := c.l
	for ; i < l; i++ {
		c.handlers[i](c)
	}
}

func (c *Context) reset(rw http.ResponseWriter, r *http.Request) {
	c.Response.reset(rw)
	c.Request = r
	c.i = -1
}

func (c *Context) Param(n string) string {
	return c.params.Get(n)
}

func (c *Context) JSON(n int, i interface{}) {
	enc := json.NewEncoder(c.Response)
	c.Response.Header().Set(HeaderContentType, MIMEJSON+"; charset=utf-8")
	c.Response.WriteHeader(n)
	if err := enc.Encode(i); err != nil {
		c.hankyo.internalServerErrorHandler(c)
	}
}

func (c *Context) String(n int, s string) {
	c.Response.Header().Set(HeaderContentType, MIMEText+"; charset=utf-8")
	c.Response.WriteHeader(n)
	c.Response.Write([]byte(s))
}

//////////////// response ////////////////

func (r *response) reset(rw http.ResponseWriter) {
	r.ResponseWriter = rw
	r.committed = false
}

//////////////// Params ////////////////

func (ps Params) Get(n string) (v string) {
	for _, p := range ps {
		if p.Name == n {
			v = p.Value
		}
	}
	return
}
