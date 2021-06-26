package main

import (
	"io"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type Dashboard struct {
	Listen        string
	SiteFilePath  string
	SiteRoutePath string
	svc           *http.Server

	closer io.Closer
}

func (s *Dashboard) Run() {
	sitePath, err := filepath.Abs(s.SiteFilePath)
	if err != nil {
		return
	}

	l, err := listen(s.Listen)
	if err != nil {
		return
	}

	r := mux.NewRouter()

	r.Methods("GET").PathPrefix("/say-hello").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("hello world ~~"))
		})

	//
	// Static file server
	//
	r.PathPrefix("/site/").
		Handler(http.StripPrefix("/site/", http.FileServer(http.Dir(sitePath))))

	//
	// Websocket handlers
	//
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     func(r *http.Request) bool { return true },
	}
	r.Methods("GET").PathPrefix("/ws-conn").
		HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				w.Write([]byte("error"))
				return
			}
			// if err != nil {
			// 	// log.Println("websocket upgrade failed")
			// 	// utils.WriteJSON(w, err, nil)
			// 	return
			// }
			// // type, data, err :=  conn.ReadMessage()
			// // log.WithField("src", conn.RemoteAddr()).Debug("new websocket connected")
			// // wsconn := &msgclient.WSConn{Conn: conn}
			// // svc.RegisterWSClient(wsconn)
			// // conn.SetCloseHandler(func(code int, text string) error {
			// // 	log.WithField("code", code).WithField("text", text).Debug("websocket closed")
			// // 	svc.UnregisterWSClient(wsconn)
			// // 	return nil
			// // })
		})

	//
	// API handlers
	//
	api := r.PathPrefix("/agent-api").Subrouter()
	{
		// GET /ctx-info
		api.Methods("GET").PathPrefix("/ctx-info").
			HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// info, err := cls.GetCtxInfo()
				// utils.WriteJSON(w, err, &struct {
				// 	VHost     string `json:"vhost"`
				// 	WsHost    string `json:"wsHost"`
				// 	ServerAPI string `json:"serverAPI"`
				// }{info.VHost, s.Listen, param["$$serverAPI"]})
			})

		// POST /new-message
		api.Methods("POST").PathPrefix("/new-message").
			HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// var msg def.Message
				// utils.ReadJSON(r, &msg)
				// msg.Track("agent-api:new-message")

				// id, err := cls.SendGroupMessage(msg.GroupID, &msg)
				// utils.WriteJSON(w, err, id)
			})

		// POST /recent-message
		api.Methods("POST").PathPrefix("/recent-message").
			HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// msgs, err := cls.GetGroupMessages(
				// 	[]uint32{0},
				// 	time.Now().Add(-7*24*time.Hour),
				// 	1000,
				// )
				// utils.WriteJSON(w, err, msgs)
			})

		// POST /online-members
		api.Methods("GET").Path("/online-members/{groupid}").
			HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// vars := mux.Vars(r)
				// groupID, err := strconv.Atoi(vars["groupid"])
				// if err != nil {
				// 	utils.WriteJSON(w, err, nil)
				// 	return
				// }

				// members, err := cls.GetGroupMembers(uint32(groupID))
				// if err != nil {
				// 	utils.WriteJSON(w, err, nil)
				// 	return
				// }
				// utils.WriteJSON(w, nil, members)
			})

		// GET /streams
		api.Methods("GET").Path("/streams").
			HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// utils.WriteJSON(w, nil, t.GetStreamStates())
			})
	}

	s.svc = &http.Server{Handler: r}
	s.closer = s.svc
	s.svc.Serve(l)
	l.Close()
}
