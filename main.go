package main

import (
	"flag"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (

	// Time allowed to write the file to the client.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the client.
	pongWait = 60 * time.Second

	// Send pings to client with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Poll file for changes with this period.
	filePeriod = 10 * time.Second
)

var (
	addr      = flag.String("addr", ":8080", "http service address")
	homeTempl = template.Must(template.New("").Parse(homeHTML))
	upgrader  = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	client = &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		} }
)

type KeyValue struct{
	Key string
	Value int
}

func testRequest(c *http.Client) (map[string]int, error) {
	resChan := make(chan *KeyValue)
	defer close(resChan)
	m := make(map[string]int)
	m["http://www.ynet.co.il"] = 0
	m["http://www.walla.co.il"] = 0
	m["https://jigsaw.w3.org/HTTP/300/302.html"] = 0
	m["http://httpstat.us/200"] = 0
	m["http://httpstat.us/206"] = 0
	m["http://httpstat.us/401"] = 0
	m["http://httpstat.us/503"] = 0
	m["http://httpstat.us/404"] = 0
	m["http://httpstat.us/500"] = 0
	m["http://httpstat.us/203"] = 0

	for k := range m{
		go func(cl *http.Client, endpoint string){
			response, err := cl.Get(endpoint)
			if err != nil{
				resChan <- &KeyValue {endpoint,-1}
			}else{
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

func writer(ws *websocket.Conn) {
	pingTicker := time.NewTicker(pingPeriod)
	fileTicker := time.NewTicker(filePeriod)
	defer func() {
		pingTicker.Stop()
		fileTicker.Stop()
		ws.Close()
	}()
	for {
		select {
		case <-fileTicker.C:
			var p map[string]int
			p, _ = testRequest(client)
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			ws.WriteJSON(struct {
				Data map[string]int
			}{p})

		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.Println(err)
		}
		return
	}

	go writer(ws)
	reader(ws)
}

func serveHome(w http.ResponseWriter, r *http.Request) {
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
		Data    map[string]int
		Host    string
	}{
		p,
		r.Host,
	}
	homeTempl.Execute(w, &v)
}

func main() {
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	if err := http.ListenAndServe(*addr, nil); err != nil {
		log.Fatal(err)
	}
}

const homeHTML = `<!DOCTYPE html>
<html lang="en">
    <head>
		<script src="https://code.jquery.com/jquery-3.2.1.min.js" integrity="sha256-hwg4gsxgFZhOsEEamdOYGBf13FyQuiTwlAQgxVSNgt4="crossorigin="anonymous"></script>

		<!-- Latest compiled and minified CSS -->
		<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap.min.css" integrity="sha384-BVYiiSIFeK1dGmJRAkycuHAHRg32OmUcww7on3RYdg4Va+PmSTsz/K68vbdEjh4u" crossorigin="anonymous">

		<!-- Optional theme -->
		<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/css/bootstrap-theme.min.css" integrity="sha384-rHyoN1iRsVXV4nD0JutlnGaslCJuC7uwjduW9SVrLvRYooPp2bWYgmgJQIXwl/Sp" crossorigin="anonymous">

		<!-- Latest compiled and minified JavaScript -->
		<script src="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.7/js/bootstrap.min.js" integrity="sha384-Tc5IQib027qvyjSMfHjOMaLkfuWVxZxUPnCJA7l2mCWNIpG9mGCD8wGNIcPD7Txa" crossorigin="anonymous"></script>

		 <title>WebSocket Endpoint Checker</title>

    </head>
    <body>
    	<div class="container">
    		<nav class="navbar navbar-light bg-faded">
				<h1 class="page-header">Endpoint Checker</h1>
    		</nav>
			<div class="row col-lg-6">
				<table class="table table-hover table-condensed">
					<thead>
						<tr class="info">
							<th>URL</th>
							<th>Status</th>
						</tr>
					</thead>
					<tbody id="endpoint-list">
					{{range $key,$value := .Data}}
						<tr>
							<td>{{$key}}</td>
							<td>{{$value}}</td>
						</tr>
					{{end}}
					</tbody>
				<table>
			</div>
		</div>
        <script type="text/javascript">
            (function() {
                var conn = new WebSocket("ws://{{.Host}}/ws");
                conn.onclose = function(evt) {
                    data.textContent = 'Connection closed';
                }
                conn.onmessage = function(evt) {
                	console.log(evt.data)
                    console.log('list updated');
                    updateList(evt.data)
                }
            })();
            function updateList(msg){
            	a = JSON.parse(msg)
            	var ul = "";
				Object.keys(a.Data).forEach(function(currentKey) {
					var objClass = colorRow(a.Data[currentKey])
					ul += "<tr class='" + objClass + "'><td>" + currentKey + "</td><td>" + a.Data[currentKey] + "</td></tr>";
				});
				document.getElementById("endpoint-list").innerHTML = ul;
            }
            function colorRow(status){
            	if(status == 200){
            		return "success"
            	}
            	if(status > 200 && status < 400){
            		return "warning"
            	}
            	if(status >= 400){
            		return "danger"
            	}
            	return "active"
            }
        </script>
    </body>
</html>
`