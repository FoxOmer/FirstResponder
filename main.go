package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/websocket"
	"html/template"

	"log"
	"net/http"
	"time"
)

const (

	// Time allowed to write the file to the client.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Poll endpoint for status with this period.
	checkerPeriod = 10 * time.Second
)

var (
	addr               string
	templatePath       string
	pollInterval       time.Duration
	withFollowRedirect bool

	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	endpointsConfigPath string
	endpoints           []string
)

func init() {
	flag.StringVar(&addr, "a", ":8080", "http service address")
	flag.StringVar(&templatePath, "t", "templates/endpoints.tmpl", "path to endpoints template")
	flag.DurationVar(&pollInterval, "i", checkerPeriod, "poll interval")
	flag.BoolVar(&withFollowRedirect, "f", false, "follow redirection")
	flag.StringVar(&endpointsConfigPath, "e", "endpoints.json", "path to endpoints configuration json")
}

type KeyValue struct {
	Key   string
	Value int
}

func testRequest(c *http.Client) (map[string]int, error) {
	resChan := make(chan *KeyValue)
	defer close(resChan)
	m := make(map[string]int)
	for _, endpoint := range endpoints {
		m[endpoint] = 0
	}

	for k := range m {
		go func(cl *http.Client, endpoint string) {
			response, err := cl.Get(endpoint)
			if err != nil {
				resChan <- &KeyValue{endpoint, -1}
			} else {
				resChan <- &KeyValue{endpoint, response.StatusCode}
			}
		}(c, k)
	}
	for i := 0; i < len(m); i++ {
		select {
		case result := <-resChan:
			m[result.Key] = result.Value
		}
	}

	return m, nil
}

func reader(ws *websocket.Conn) {
	defer ws.Close()
	ws.SetReadLimit(512)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			break
		}
	}
}

func writer(ws *websocket.Conn, client *http.Client) {
	pingTicker := time.NewTicker(pingPeriod)
	endpointTicker := time.NewTicker(pollInterval)
	defer func() {
		pingTicker.Stop()
		endpointTicker.Stop()
		ws.Close()
	}()
	for {
		select {
		case <-endpointTicker.C:
			var p map[string]int
			p, _ = testRequest(client)
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			ws.WriteJSON(struct {
				Data       map[string]int
				LastUpdate string
			}{p, time.Now().Format("2006-01-02T15:04:05.999999-07:00")})

		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func serveWs(w http.ResponseWriter, r *http.Request, client *http.Client) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}

	go writer(ws, client)
	reader(ws)
}

func serveChecker(w http.ResponseWriter, r *http.Request, tmpl *template.Template, client *http.Client) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", 404)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	p, _ := testRequest(client)

	var v = struct {
		Data       map[string]int
		Host       string
		LastUpdate string
	}{
		p,
		r.Host,
		time.Now().Format("2006-01-02T15:04:05.999999-07:00"),
	}
	tmpl.Execute(w, &v)
}

func newHttpClient(withFollowRedirect bool) *http.Client {
	if withFollowRedirect {
		return http.DefaultClient
	}
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}
}

func main() {
	var (
		tmpl *template.Template
		err  error
	)

	flag.Parse()

	endpoints = loadEndpoints(endpointsConfigPath)

	if tmpl, err = template.ParseFiles(templatePath); err != nil {
		log.Fatal(fmt.Sprintf("FATAL - Could not load template: %s", templatePath))
		return
	}

	client := newHttpClient(withFollowRedirect)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveChecker(w, r, tmpl, client)
	})
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(w, r, client)
	})
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
