package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/net-agent/flex"
	"github.com/net-agent/mixlisten"
	"github.com/net-agent/remotework/rpc/notifyclient"
)

func main() {
	var flags ServerFlags
	flags.Parse()

	// 读取配置
	log.Printf("read config from '%v'\n", flags.ConfigFileName)
	config, err := NewConfig(flags.ConfigFileName)
	if err != nil {
		log.Fatal("load config failed: ", err)
	}

	// 初始化
	sw := flex.NewSwitcher(nil)

	log.Printf("try to listen on '%v'\n", config.Server.Listen)

	if !config.Server.WsEnable {
		sw.Run(config.Server.Listen, config.Server.Password)
		return
	}

	mxl := mixlisten.Listen("tcp", config.Server.Listen)
	mxl.Register(mixlisten.Flex())
	mxl.Register(mixlisten.HTTP())

	flexListener, err := mxl.GetListener(mixlisten.Flex().Name())
	if err != nil {
		log.Fatal("get flex listener failed: ", err)
	}

	httpListener, err := mxl.GetListener(mixlisten.HTTP().Name())
	if err != nil {
		log.Fatal("get http listener failed: ", err)
	}
	go serveFlex(sw, flexListener, config.Server.Password)
	go serveHTTP(sw, httpListener, config.Server.WsPath)

	mxl.Run()
	log.Println("server stopped")
}

func serveFlex(sw *flex.Switcher, listener net.Listener, password string) {
	sw.Serve(listener, password)
	log.Println("flex server stopped.")
}

func serveHTTP(sw *flex.Switcher, listener net.Listener, wsPath string) {
	r := mux.NewRouter()

	r.Methods("GET").Path(wsPath).HandlerFunc(GetWsconnHandler(sw))

	api := r.PathPrefix("/api").Subrouter()
	api.Methods("GET").Path("/test").HandlerFunc(GetTestHandler(sw))

	log.Println("http server is running")
	http.Serve(listener, r)
	log.Println("http server stopped.")
}

func GetWsconnHandler(sw *flex.Switcher) http.HandlerFunc {

	upgrader := websocket.Upgrader{}

	return func(w http.ResponseWriter, r *http.Request) {
		wsconn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			w.Write([]byte(fmt.Sprintf("upgrade failed: %v", err)))
			return
		}
		go sw.ServePacketConn(flex.NewWsPacketConn(wsconn))
	}
}

func GetTestHandler(sw *flex.Switcher) http.HandlerFunc {
	domain := "pushserverhost"
	host, err := RegistHost(sw, domain)
	if err != nil {
		log.Printf("regist '%v' failed: %v\n", domain, err)
		return nil
	}

	return func(w http.ResponseWriter, r *http.Request) {
		stream, err := host.Dial("test:15")
		if err != nil {
			log.Printf("dial test:15 failed: %v\n", err)
			return
		}
		client := rpc.NewClient(stream)

		var args notifyclient.PushNotifyArgs
		var reply notifyclient.PushNotifyReply

		q := r.URL.Query()
		args.Sender = q.Get("sender")
		args.Message = q.Get("msg")

		err = client.Call("NotifyClient.PushNotify", &args, &reply)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		w.Write([]byte("okok"))
	}
}
