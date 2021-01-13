package main

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Curricane/crpc"
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
	client, _ := crpc.Dial("tcp", <-addr)
	defer func() { _ = client.Close() }()

	time.Sleep(time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("crpc req %d", i)
			var reply string
			if err := client.Call("Foo.Sum", args, &reply); err != nil {
				log.Errorf("call Foo.Sum error:", err)
				return
			}
			log.Info("reply:", reply)
		}(i)
	}
	wg.Wait()
}
