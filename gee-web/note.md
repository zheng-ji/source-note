
### Day1

朴素的逻辑，只要实现了 ServerHTTP 接口的 Struct，均可作为参数调用 http.ListenAndServe(

```
type HandlerFunc func(http.ResponseWriter, *http.Request)

type Engine struct {
	router map[string]HandlerFunc
}

func (engine *Engine) GET(pattern string, handler HandlerFunc) {
	engine.addRoute("GET", pattern, handler)
}

func (engine *Engine) addRoute(method string, pattern string, handler HandlerFunc) {
	key := method + "-" + pattern
	log.Printf("Route %4s - %s", method, pattern)
	engine.router[key] = handler
}

func (engine *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	key := req.Method + "-" + req.URL.Path
	if handler, ok := engine.router[key]; ok {
		handler(w, req)
	} else {
		fmt.Fprintf(w, "404 NOT FOUND: %s\n", req.URL)
	}
}

// Run defines the method to start a http server
func (engine *Engine) Run(addr string) (err error) {
	return http.ListenAndServe(addr, engine)
}
```

Example
```
func main() {
	r := gee.New()
	r.GET("/", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "URL.Path = %q\n", req.URL.Path)
	})

	r.GET("/hello", func(w http.ResponseWriter, req *http.Request) {
		for k, v := range req.Header {
			fmt.Fprintf(w, "Header[%q] = %q\n", k, v)
		}
	})

	r.Run(":9999")
}
```

### Day2 With Context

* 把 http.ResponseWriter, http.Request 囊括进 Context

* 把 handlers map[string]HandlerFunc 囊括进 router

```
// Engine implement the interface of ServeHTTP
type Engine struct {
	router *router
}

// New is the constructor of gee.Engine
func New() *Engine {
	return &Engine{router: newRouter()}
}

func (r *router) addRoute(method string, pattern string, handler HandlerFunc) {
	key := method + "-" + pattern
	r.handlers[key] = handler
}

func (r *router) handle(c *Context) {
	key := c.Method + "-" + c.Path
	if handler, ok := r.handlers[key]; ok {
		handler(c)
	} else {
		c.String(http.StatusNotFound, "404 NOT FOUND: %s\n", c.Path)
	}
}

// Example
func main() {
	r.GET("/hello", func(c *gee.Context) {
		// expect /hello?name=geektutu
		c.String(http.StatusOK, "hello %s, you're at %s\n", c.Query("name"), c.Path)
	})
}
```

# Day3
将路径解析为节点，然后存储在router的roots节点之内, 这么做是了为解决匹配动态参数

```
key := method + "-" + pattern
_, ok := r.roots[method]
if !ok {
    r.roots[method] = &node{}
}
r.roots[method].insert(pattern, parts, 0)
r.handlers[key] = handler
```

Example

```
func TestGetRoute(t *testing.T) {
	r := newTestRouter()
	n, ps := r.getRoute("GET", "/hello/geektutu")

	if n == nil {
		t.Fatal("nil shouldn't be returned")
	}

	if n.pattern != "/hello/:name" {
		t.Fatal("should match /hello/:name")
	}

	if ps["name"] != "geektutu" {
		t.Fatal("name should be equal to 'geektutu'")
	}

	fmt.Printf("matched path: %s, params['name']: %s\n", n.pattern, ps["name"])
}
```

trie.go 代码就比较烧脑，我写了写注释在代码层面上
