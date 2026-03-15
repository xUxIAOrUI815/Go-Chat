package chat

import (
	"context"
	"encoding/json"
	"go-chat/internal/config"
	"go-chat/internal/dao"
	"go-chat/internal/dto/request"
	"go-chat/internal/model"
	myKafka "go-chat/internal/service/kafka"
	"go-chat/pkg/constants"
	"go-chat/pkg/enum/message/message_status_enum"
	"go-chat/pkg/zlog"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/segmentio/kafka-go"
)

// 消息回传结构体 服务端向客户端推送消息
type MessageBack struct {
	Message []byte
	Uuid    string
}

// 客户端连接结构体 每个 WebSocket 连接对应一个 Client 实例
type Client struct {
	Conn     *websocket.Conn
	Uuid     string
	SendTo   chan []byte
	SendBack chan *MessageBack
}

// WebSocket 升级器 将HTTP连接升级为WebSocket连接
var upgrader = websocket.Upgrader{
	ReadBufferSize:  2048, // 读缓冲区大小
	WriteBufferSize: 2048, // 写缓冲区大小
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var ctx = context.Background()

var messageMode = config.GetConfig().KafkaConfig.MessageMode // 消息模式 channnel/kafka

// 读取客户端发送的消息
func (c *Client) Read() {
	zlog.Info("ws read goroutine start")
	for {
		// Step1: 阻塞读取客户端通过 WebSocket 发过来的消息 字节流
		_, jsonMessage, err := c.Conn.ReadMessage() // 阻塞状态
		if err != nil {
			zlog.Error(err.Error())
			return // 连接异常 退出协程 断开 WebSocket
		} else {
			// Step2: 解析JSON消息为结构化对象
			var message = request.ChatMessageRequest{}
			if err := json.Unmarshal(jsonMessage, &message); err != nil {
				zlog.Error(err.Error())
			}
			log.Println("接收到的消息为: ", jsonMessage)
			// Step3: 根据配置选择消息转发模式
			if messageMode == "channel" { // 内存通道模式
				// 条件: ChatServer的全局通道没满 + 客户端本地通道有积压消息
				for len(ChatServer.Transmit) < constants.CHANNEL_SIZE && len(c.SendTo) > 0 {
					sendToMessage := <-c.SendTo                     // 从客户端本地通道读取一条积压消息
					ChatServer.SendMessageToTransmit(sendToMessage) // 送到ChatServer 全局通道
				}
				// 处理刚收到的新消息
				if len(ChatServer.Transmit) < constants.CHANNEL_SIZE { // 满足条件
					ChatServer.SendMessageToTransmit(jsonMessage) // 送到ChatServer 全局通道
				} else if len(c.SendTo) < constants.CHANNEL_SIZE { // 缓冲区已满
					c.SendTo <- jsonMessage // 缓存到客户端本地通道
				} else {
					if err := c.Conn.WriteMessage(websocket.TextMessage, []byte("消息发送失败，请稍后再试")); err != nil {
						zlog.Error(err.Error())
					}
				}
			} else {
				if err := myKafka.KafkaService.ChatWriter.WriteMessages(ctx, kafka.Message{
					Key:   []byte(strconv.Itoa(config.GetConfig().KafkaConfig.Partition)),
					Value: jsonMessage,
				}); err != nil {
					zlog.Error(err.Error())
				}
				zlog.Info("已发送消息：" + string(jsonMessage))
			}
		}
	}
}

// 从 send 通道读取消息发送给 websocket
func (c *Client) Write() {
	zlog.Info("ws write goroutine start")
	for messageBack := range c.SendBack {
		err := c.Conn.WriteMessage(websocket.TextMessage, messageBack.Message)
		if err != nil {
			zlog.Error(err.Error())
			return
		}
		// 修改状态为 已发送
		if res := dao.GormDB.Model(&model.Message{}).Where("uuid = ?", messageBack.Uuid).Update("status", message_status_enum.SENT); res.Error != nil {
			zlog.Error(res.Error.Error())
		}
	}
}

// NewClientInit 当接受到前端有登录消息时，会调用该函数
func NewClientInit(c *gin.Context, clientId string) {
	kafkaConfig := config.GetConfig().KafkaConfig
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zlog.Error(err.Error())
		return
	}
	client := &Client{
		Conn:     conn,                                            // websocket连接
		Uuid:     clientId,                                        // 客户端标识
		SendTo:   make(chan []byte, constants.CHANNEL_SIZE),       // 发往服务端的消息缓冲通道
		SendBack: make(chan *MessageBack, constants.CHANNEL_SIZE), // 服务端推送给前端的消息缓冲通道
	}
	if kafkaConfig.MessageMode == "channel" {
		ChatServer.SendClientToLogin(client)
	} else {
		KafkaChatServer.SendClientToLogin(client)
	}
	go client.Read()  // 启动读协程，持续读取前端发送的消息
	go client.Write() // 启动写协程，持续将服务端发送的消息发送给前端
	zlog.Info("新用户加入：" + clientId)
}

// ClientLogout 当接受到前端有登出消息时，会调用该函数
func ClientLogout(clientId string) (string, int) {
	//
	kafkaConfig := config.GetConfig().KafkaConfig
	client := ChatServer.Clients[clientId]
	if client != nil {
		if kafkaConfig.MessageMode == "channel" {
			ChatServer.SendClientToLogout(client)
		} else {
			KafkaChatServer.SendClientToLogout(client)
		}

		if err := client.Conn.Close(); err != nil {
			zlog.Error(err.Error())
			return constants.SYSTEM_ERROR, -1
		}

		close(client.SendTo)
		close(client.SendBack)
	}
	return "登出成功", 0
}
