// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jinzhu/gorm"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var (
	db *gorm.DB
)

func dbsetting(database *gorm.DB) {
	db = database
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,

	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub // 親となるHub

	// The websocket connection.
	conn *websocket.Conn // 自分のwebsocket

	// Buffered channel of outbound messages.
	send chan []byte // broadcastのメッセージを受け取るチャネル
}

type Message struct {
	MessageType string `json:"messageType"`
	Message     string `json:"message"`
}

// type QuestionRequest struct {
// 	MessageType  string `json:"messageType"`
// 	UserId       string `json:"userId"`
// 	MeetingId    int    `json:"meetingId"`
// 	QuestionBody string `json:"questionBody"`
// 	DocumentId   int    `json:"documentId`
// 	DocumentPage int    `json:"documentPage`
// 	QuestionTime string `json:questionTime`
// }

type QuestionResult struct {
	MessageType  string `json:"messageType"`
	MeetingId    int    `json:"meetingId"`
	QuestionBody string `json:"questionBody"`
	DocumentId   int    `json:"documentId"`
	DocumentPage int    `json:"documentPage"`
	QuestionTime string `json:"questionTime"`
}

type ModeratorMsg struct {
	MessageType      string `json:"messageType"`
	MeetingId        int    `json:"meetingId"`
	ModeratorMsgBody string `json:"moderatorMsgBody"`
}

var questionCount = make(map[int]int)

const maxQuestionNum = 5

func loadJson(byteArray []byte) (interface{}, error) {
	var jsonObj interface{}
	err := json.Unmarshal(byteArray, &jsonObj)
	return jsonObj, err
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()

		// エラー処理
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// websocketで受け取ったデータの処理
		jsonObj, jsonerr := loadJson(message)
		if jsonerr != nil {
			fmt.Println("loadJsonでエラーが発生しました")
			continue
		}
		fmt.Printf(string(message) + "\n")
		message_type := jsonObj.(map[string]interface{})["messageType"].(string)

		var messagestruct interface{}

		switch message_type {
		case "message":
			message_jsonobj := jsonObj.(map[string]interface{})["message"].(string)
			messagestruct = Message{MessageType: "message", Message: message_jsonobj}
		case "question":
			var (
				layout      = "2006/01/02 15:04:05"
				location, _ = time.LoadLocation("Asia/Tokyo")
			)

			userId := jsonObj.(map[string]interface{})["userId"].(string)
			meetingId := int(jsonObj.(map[string]interface{})["meetingId"].(float64))
			questionBody := jsonObj.(map[string]interface{})["questionBody"].(string)
			documentId := int(jsonObj.(map[string]interface{})["documentId"].(float64))
			documentPage := int(jsonObj.(map[string]interface{})["documentPage"].(float64))
			questionTimeStr := jsonObj.(map[string]interface{})["questionTime"].(string)

			questionTime, _ := time.ParseInLocation(layout, questionTimeStr, location)
			question := Question{
				UserId:       userId,
				QuestionBody: questionBody,
				DocumentId:   documentId,
				DocumentPage: documentPage,
				VoteNum:      0,
				QuestionTime: questionTime,
			}

			if !createQuestion(db, question) {
				return
			}

			messagestruct = QuestionResult{
				MessageType:  message_type,
				MeetingId:    meetingId,
				QuestionBody: questionBody,
				DocumentId:   documentId,
				DocumentPage: documentPage,
				QuestionTime: questionTimeStr,
			}
		case "finishword":
			meetingId := int(jsonObj.(map[string]interface{})["meetingId"].(float64))
			presenterId := jsonObj.(map[string]interface{})["presenterId"].(string)
			documentId := int(jsonObj.(map[string]interface{})["documentId"].(float64))
			finishType := jsonObj.(map[string]interface{})["finishType"].(string)

			var moderatorMsg string
			// 規定の質問数に達した場合
			if questionCount[meetingId] >= maxQuestionNum {
				endPresenter, nextUserId := getNextPresenterId(db, meetingId, presenterId)
				if !endPresenter {
					moderatorMsg = personEnd(presenterId, nextUserId)
				} else {
					moderatorMsg = meetingEnd()
				}
				questionCount[meetingId] = 0
			} else {
				if finishType == "present" {
					moderatorMsg = presenOrQuestionEnd(db, meetingId, presenterId, documentId, true)
				} else if finishType == "question" {
					moderatorMsg = presenOrQuestionEnd(db, meetingId, presenterId, documentId, false)
				} else {
					fmt.Printf("予期せぬfinishType:%s\n", finishType)
					continue
				}
				if questionCount[meetingId] == 0 {
					questionCount[meetingId] = 1
				} else {
					questionCount[meetingId] += 1
				}
				fmt.Printf("現在の質問数：%d\n", questionCount[meetingId])
			}
			fmt.Println(moderatorMsg)

			messagestruct = ModeratorMsg{
				MessageType:      "moderator_msg",
				MeetingId:        meetingId,
				ModeratorMsgBody: moderatorMsg,
			}
		default:
			continue
		}
		messagejson, _ := json.Marshal(messagestruct)

		// 自分のメッセージをhubのbroadcastチャネルに送り込む
		fmt.Printf("%+v\n", messagestruct)
		c.hub.broadcast <- messagejson
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			// タイムアウト時間の設定
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			// エラー処理
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("unsuccessed upgrade.")
		log.Println(err)
		return
	} else {
		fmt.Println("successed upgrade!")
	}
	// sendは他の人からのメッセージが投入される
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client // hubのregisterチャネルに自分のClientを登録

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()
}
