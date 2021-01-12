package codec

import "io"

/*
1，定义Codec接口，增加codec的扩展性
2，定义了消息头格式
3，声明了，多个Codec的注册/构造方式
*/

// Header 消息头
type Header struct {
	ServiceMethod string // format "Service.Method"
	Seq           uint64 // sequence number chosen by client
	Error         string
}

// Codec 类型，为了兼容更多的codec方式
type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Write(*Header, interface{}) error
}

// NewCodecFunc Codec构造函数
type NewCodecFunc func(io.ReadWriteCloser) Codec

// Type 类型
type Type string

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json" // not implemented
)

// NewCodecFuncMap 全局变量，存放注册过的Codec构造函数
var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)

	// 注册NewGobCodec
	NewCodecFuncMap[GobType] = NewGobCodec
}
