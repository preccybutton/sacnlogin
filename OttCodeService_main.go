package main

import (
	"flag"
	"fmt"
	"github.com/bonjovis/config"
	"github.com/buaazp/fasthttprouter"
	"github.com/fasthttp/websocket"
	"github.com/go-redis/redis"
	"github.com/lestrrat-go/file-rotatelogs"
	"github.com/valyala/fasthttp"
	"html/template"
	"log"
	"reflect"
	"strings"
	"time"
	"testin/OttCodeService"
)

var (
	conf     *config.Config
	client   *redis.Client
	upgrader = websocket.FastHTTPUpgrader{
		CheckOrigin: func(r *fasthttp.RequestCtx) bool {
			return true
		},
		HandshakeTimeout: 60 * time.Second,
	}
)

func closeConn(ws *websocket.Conn) {
	log.Println("websocket connect closed")
	ws.Close()
}

func ws(ctx *fasthttp.RequestCtx) {
	//log.Println(ctx)
	tvid := string(ctx.QueryArgs().Peek("tvid"))
	if len(tvid) == 0 {
		log.Println("params error")
		return
	}
	err := upgrader.Upgrade(ctx, func(ws *websocket.Conn) {
		defer closeConn(ws)
		//log.Println("tvid:" + tvid)
		ws.SetReadDeadline(time.Now().Add(time.Duration(conf.Cint("ws", "deadLine")) * time.Second))
		closed := false
		handle := OttCodeService.New(conf, client, ws, &closed)
		go func() {
			ret := handle.SubscribeHandle(tvid)
			if ret != true {
				log.Println("action error")
			}
		}()
		for {
			res := make(map[string]interface{})
			err := ws.ReadJSON(&res)
			if err != nil {
				log.Println("read:", err)
				closed = true
				break
			} else {
				log.Println("tvid:"+tvid, res)
				if res["action"] == nil || res["action"] == "" {
					log.Println("params error")
					//break
				} else {
					ret := handle.ActionHandle(res["action"].(string), tvid)
					if ret != true {
						log.Println("handle error")
						//break
					}
				}
			}
		}
	})

	if err != nil {
		if _, ok := err.(websocket.HandshakeError); ok {
			log.Println(err)
		}
		log.Println(err)
		return
	}
}

func homeView(ctx *fasthttp.RequestCtx) {
	if conf.Cstring("http", "debug") == "false" {

		fmt.Fprintf(ctx, "Your ip is %q\n\n", ctx.RemoteIP())
		ctx.SetContentType("text/plain; charset=utf8")
		ctx.Response.Header.Set("X-My-Header", "bestvapp")
	} else {
		ctx.SetContentType("text/html")
		homeTemplate.Execute(ctx, "wss://"+conf.Cstring("http", "host")+"/wxlogin/ws?tvid=D150111265100052C9")
	}
}

func queryIpAuth(ip string) bool {
	if conf.Cstring("http", "useIpWhiteList") == "false" {
		return true
	}
	ipStr := conf.Cstring("http", "ipWhiteList")
	iplist := strings.Split(ipStr, ",")
	for _, ipSet := range iplist {
		if ipSet == ip {
			return true
		}
	}
	log.Println(ip + "not in ip list")
	return false
}

func clientIP(ctx *fasthttp.RequestCtx) string {
	clientIP := string(ctx.Request.Header.Peek("X-Forwarded-For"))
	if index := strings.IndexByte(clientIP, ','); index >= 0 {
		clientIP = clientIP[0:index]
	}
	clientIP = strings.TrimSpace(clientIP)
	if len(clientIP) > 0 {
		return clientIP
	}
	clientIP = strings.TrimSpace(string(ctx.Request.Header.Peek("X-Real-Ip")))
	if len(clientIP) > 0 {
		return clientIP
	}
	return ctx.RemoteIP().String()
}

func main() {
	flag.Parse()

	conf, _ = config.ReadDefault("config.cfg")
	path := conf.Cstring("app", "log")
	logf, err := rotatelogs.New(
		path+"ott-code-service.log.%Y%m%d",
		rotatelogs.WithLinkName(path+"ott-code-service.log"),
		rotatelogs.WithMaxAge(-1),
		rotatelogs.WithRotationTime(24*time.Hour),
	)
	if err != nil {
		log.Printf("failed to create rotatelogs: %s", err)
		return
	}
	log.SetOutput(logf)
	router := fasthttprouter.New()
	router.GET("/", homeView)
	router.HEAD("/", homeView)
	router.GET("/ws", ws)
	server := fasthttp.Server{
		Name:    "QiaoHuService",
		Handler: router.Handler,
	}

	client = redis.NewClient(&redis.Options{
		Addr:        conf.Cstring("redis", "host") + ":" + conf.Cstring("redis", "port"),
		PoolSize:    int(conf.Cint("redis", "poolSize")),
		PoolTimeout: time.Duration(int(conf.Cint("redis", "poolTimeout"))),
		Password:    conf.Cstring("redis", "password"),
		DB:          int(conf.Cint("redis", "db")),
	})

	log.Println(reflect.TypeOf(client))

	pong, err := client.Ping().Result()

	log.Println("Redis info: ", pong, err)

	log.Println("OTT SCAN Service Version 1.0.2")
	addr := flag.String("addr", ":"+conf.Cstring("http", "port"), "http service address")
	log.Fatal(server.ListenAndServe(*addr))
}

var homeTemplate = template.Must(template.New("").Parse(`
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<script>
window.addEventListener("load", function(evt) {

    var output = document.getElementById("output");
    var input = document.getElementById("input");
    var ws;

    var print = function(message) {
        var d = document.createElement("div");
        d.innerHTML = message;
        output.appendChild(d);
    };

    document.getElementById("open").onclick = function(evt) {
        if (ws) {
			console.log(evt)
            return false;
        }
        ws = new WebSocket("{{.}}");
		print("connect to {{.}}");
        ws.onopen = function(evt) {
			console.log("open")
			print("OPEN");
			console.log(evt)
        }
        ws.onclose = function(evt) {
			print("CLOSE");
			console.log("close")
            ws = null;
        }
        ws.onmessage = function(evt) {
			console.log("response:",evt.data);
			//document.getElementById("preview").src = "data:image/png;base64," + Base64.encode(evt.data);
			document.getElementById("preview").src = URL.createObjectURL(evt.data);
			print("response:",evt.data);
        }
        ws.onerror = function(evt) {
			console.log("error:",evt);
        }
        return false;
    };

    document.getElementById("send").onclick = function(evt) {
        if (!ws) {
            return false;
        }
		console.log("send:",input.value);
        ws.send(input.value);
        return false;
    };

    document.getElementById("close").onclick = function(evt) {
        if (!ws) {
            return false;
        }
        ws.close();
        return false;
    };

});
</script>
</head>
<body>
<table>
<tr><td valign="top" width="50%">
<p>Click "Open" to create a connection to the server,
"Send" to send a message to the server and "Close" to close the connection.
You can change the message and send multiple times.
<p>
<form>
<button id="open">Open</button>
<button id="close">Close</button>
<p><input id="input" type="text" value='{"action":"qrcode"}'>
<button id="send">Send</button>
<img id="preview" src="" />
</form>
</td><td valign="top" width="50%">
<div id="output"></div>
</td></tr></table>
</body>
</html>
`))
