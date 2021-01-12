package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/Curricane/crpc"
	"github.com/Curricane/crpc/codec"
	"github.com/Curricane/crpc/log"
)

func startServer(addr chan string) {
	// pick a free port
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Error("network error: ", err)
		return
	}
	log.Info("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	crpc.Accept(l)
}

func main() {
	addr := make(chan string)
	go startServer(addr)

	// in fact, following code is like a simple crpc client
	conn, _ := net.Dial("tcp", <-addr)
	defer func() { _ = conn.Close() }()

	time.Sleep(time.Second)

	// send options
	_ = json.NewEncoder(conn).Encode(crpc.DefaultOption)
	cc := codec.NewGobCodec(conn)

	// send request & receive response
	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.Sum",
			Seq:           uint64(i),
		}
		_ = cc.Write(h, fmt.Sprintf("crpc req %d", h.Seq))
		_ = cc.ReadHeader(h)
		var reply string
		_ = cc.ReadBody(&reply)
		log.Info("reply:", reply)
	}
}
