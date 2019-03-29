package server

import (
	"golang.org/x/net/websocket"
	"log"
	"net/http"
	"regexp"
	"strings"
)

func (srv *Server) handleWs(con *websocket.Conn) {

	var buf = make([]byte, 1)
	for {
		if _, err := con.Read(buf); err == nil {
			_, err := con.Write(buf)
			if err != nil {
				log.Println(err)
			}
		} else {
			log.Println(err)
			return
		}
	}
}

var connectionUpgradeRegex = regexp.MustCompile("(^|.*,\\s*)upgrade($|\\s*,)")

func isWebsocketRequest(req *http.Request) bool {
	return connectionUpgradeRegex.MatchString(strings.ToLower(req.Header.Get("Connection"))) && strings.ToLower(req.Header.Get("Upgrade")) == "websocket"
}
