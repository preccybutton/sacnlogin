package OttCodeService

import (
	"github.com/bonjovis/config"
	"github.com/fasthttp/websocket"
	"github.com/go-redis/redis"
	"log"
	"time"
)

type ActionInstance struct {
	conf   *config.Config
	client *redis.Client
	ws     *websocket.Conn
	closed *bool
}

func New(conf *config.Config, client *redis.Client, ws *websocket.Conn, closed *bool) *ActionInstance {
	action := &ActionInstance{conf, client, ws, closed}
	return action
}

func (ac *ActionInstance) ActionHandle(action string, tvid string) bool {
	switch action {
	case "qrcode":
		ret := ac.GetQRCodeHandle(tvid)
		if ret != true {
			log.Println("tvid:"+tvid, "get qrcode error")
			return false
		}
		return true
	default:
		log.Println("tvid:"+tvid, "action error")
		return false
	}
}

func (ac *ActionInstance) getOTTLoginUrl(tvid string) (bool, string) {
	return true, "http://www.baidu.com"
}

func (ac *ActionInstance) GetQRCodeHandle(tvid string) bool {
	ret, url := ac.getOTTLoginUrl(tvid)
	if ret == false {
		return false
	}
	QRCode, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		log.Println("tvid:"+tvid, err)
		return false
	}
	err = ac.ws.WriteMessage(websocket.BinaryMessage, QRCode)
	if err != nil {
		log.Println("tvid:"+tvid, "write message:", err)
		return false
	}
	return true
}

func (ac *ActionInstance) SubscribeHandle(tvid string) bool {
	sub := ac.client.Subscribe("ott:code:" + tvid)
	defer sub.Close()
	_, err := sub.Receive()
	if err != nil {
		log.Println("tvid:"+tvid, "subscribe err:", err)
	}
	ch := sub.Channel()
	time.AfterFunc(time.Duration(ac.conf.Cint("ws", "deadLine")-1)*time.Second, func() {
		log.Println("tvid:"+tvid, "subscribe handle closed")
		sub.Close()
	})
	for msg := range ch {
		log.Println("tvid:"+tvid, "closed status:", *ac.closed)
		if !*ac.closed {
			err = ac.ws.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
			if err != nil {
				log.Println("tvid:"+tvid, "write:", err)
				return false
			}
		} else {
			return false
		}
	}
	return true
}
