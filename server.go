package crpc

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"

	"github.com/Curricane/crpc/codec"
	"github.com/Curricane/crpc/log"
)

/*
实现crpc服务端
1，声明报文格式 | Option | Header1 | Body1 | Header2 | Body2 |
2, server连接 Accept
3，server处理请求 Accept->获取请求->处理请求->响应请求
*/

const MagicNumber = 0x3bef5c

// Option 固定在报文的最开始，Header 和 Body 可以有多个
// 报文 | Option | Header1 | Body1 | Header2 | Body2 | ...
type Option struct {
	MagicNumber int        // MagicNumber marks this's a crpc request
	CodecType   codec.Type // client may choose different Codec to encode body
}

// DefaultOption 默认的Option
var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

// Server represents an RPC Server.
type Server struct {
	serviceMap sync.Map
}

// NewServer returns a new Server.
func NewServer() *Server {
	return &Server{}
}

// DefaultServer is the default instance of *Server.
var DefaultServer = NewServer()

func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

// Register publishes in the server the set of methods
func (server *Server) Register(rcvr interface{}) error {
	s := newService(rcvr)
	if _, dup := server.serviceMap.LoadOrStore(s.name, s); dup {
		return errors.New("rpc: sercie already defined: " + s.name)
	}
	return nil
}

// ServeConn runs the server on a single connection.
// ServeConn blocks, serving the connection until the client hangs up.
func (server *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()

	// step1 读取Option
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Infof("rpc server: options error: ", err)
		return
	}
	if opt.MagicNumber != MagicNumber {
		log.Infof("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}

	// step2 确定Codec
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Infof("rpc server: invalid codec type %s", opt.CodecType)
		return
	}

	// step3 Codec处理连接
	server.serverCodec(f(conn))
}

// invalidRequest is a placeholder for response argv when error occurs
var invalidRequest = struct{}{}

func (server *Server) serverCodec(cc codec.Codec) {
	sending := new(sync.Mutex) // make sure to send a complete response
	wg := new(sync.WaitGroup)  // wait until all request are handled
	for {
		// step1 获取request
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

		// step2 处理request
		go server.handleRequest(cc, req, sending, wg)
	}
	wg.Wait()
	_ = cc.Close()
}

// 一个典型的 RPC 调用如下
// err = client.Call("Arith.Multiply", args, &reply)
// 请求和响应中的参数和返回值抽象为 body，剩余的信息放在 header 中
type request struct {
	h            *codec.Header // header of request
	argv, replyv reflect.Value // argv and replyv of request
	mtype        *methodType
	svc          *service
}

func (server *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}
	req.svc, req.mtype, err = server.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()
	
	// make sure that argvi is a pointer, ReadBody need a pointer as parameter
	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}
	if err = cc.ReadBody(argvi); err != nil {
		log.Errorf("rpc server: read body err:", err)
		return req, err
	}
	return req, nil
}

func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Errorf("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (server *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Errorf("rpc server: write response error:", err)
	}
}

func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	// day 1, just print argv and send a hello message
	defer wg.Done()
	err := req.svc.call(req.mtype, req.argv, req.replyv)
	if err != nil {
		req.h.Error = err.Error()
		server.sendResponse(cc, req.h, invalidRequest, sending)
		return
	}
	server.sendResponse(cc, req.h, req.replyv.Interface(), sending)
	return
}

// Accept accepts connections on the listener and serves requests
// for each incoming connection.
func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Info("rpc server: accept error:", err)
			return
		}
		go server.ServeConn(conn)
	}
}

func (server *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed: " + serviceMethod)
		return
	}
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	svci, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}
	
	svc = svci.(*service)
	
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc server: can't find method " + methodName)
	}
	
	return
}

// Accept accepts connections on the listener and serves requests
// for each incoming connection.
func Accept(lis net.Listener) { DefaultServer.Accept(lis) }
