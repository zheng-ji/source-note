
gob : Golang object binary
TCPConn  ==>  转化为 io.ReadWriteCloser
bufio.NewWriter(conn)

conn 是由构建函数传入，通常是通过 TCP 或者 Unix 建立 socket 时得到的链接实例，dec 和 enc 对应 gob 的 Decoder 和 Encoder，buf 是为了防止阻塞而创建的带缓冲的 Writer，一般这么做能提升性能。

```
type Codec interface {
	io.Closer // 为了能关闭 close
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}

// 读取Header
func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}

// 读取 Body
func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

type Header struct {
	ServiceMethod string // format "Service.Method"
	Seq           uint64 // sequence number chosen by client
	Error         string
}


// request stores all information of a call
type request struct {
	h            *codec.Header // header of request
	argv        reflect.Value // request body
	replyv      reflect.Value // reply 
}

// 主要处理逻辑
func (server *Server) serveCodec(cc codec.Codec) {
	sending := new(sync.Mutex) // make sure to send a complete response
	wg := new(sync.WaitGroup)  // wait until all request are handled
	for {
		req, err := server.readRequest(cc)
		if err != nil {
			if req == nil {
				break // it's not possible to recover, so close the connection
			}
			req.h.Error = err.Error()
			server.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		go server.handleRequest(cc, req, sending, wg)
	}
	wg.Wait()
	_ = cc.Close()
}
```
