package codec

import (
	"bufio"
	"encoding/gob"
	"io"

	"github.com/Curricane/crpc/log"
)

/*
Codec之Gob实现
*/

// GobCodec 结构
type GobCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer // buf 是为了防止阻塞而创建的带缓冲的 Writer，一般这么做能提升性能。
	dec  *gob.Decoder
	enc  *gob.Encoder
}

var _ Codec = (*GobCodec)(nil)

// NewGobCodec 返回一个Codec
func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf, // 缓存编码的数据
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}

// ReadHeader 从c.conn中读取数据并解码为Header
func (c *GobCodec) ReadHeader(h *Header) error {
	return c.dec.Decode(h)
}

// ReadBody 从c.conn中读取数据并解码为body
func (c *GobCodec) ReadBody(body interface{}) error {
	return c.dec.Decode(body)
}

// Write 向c.buf中写入编码后的Header和body
func (c *GobCodec) Write(h *Header, body interface{}) (err error) {

	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()

	if err = c.enc.Encode(h); err != nil {
		log.Infof("rpc: gob error encoding header:", err)
		return
	}

	if err = c.enc.Encode(body); err != nil {
		log.Infof("rpc: gob error encoding body:", err)
		return
	}
	return
}

// Close 关闭c.conn连接
func (c *GobCodec) Close() error {
	return c.conn.Close()
}
