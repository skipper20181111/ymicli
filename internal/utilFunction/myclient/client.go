package myclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/card"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/handler"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/plugin"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/utils"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 14:23
 */

type StreamClient struct {
	AppCredential *client.AppCredentialConfig
	UserAgent     *client.UserAgentConfig
	AutoReconnect bool

	subscriptions map[string]map[string]handler.IFrameHandler

	conn          *websocket.Conn
	sessionId     string
	mutex         sync.Mutex
	extras        map[string]string
	openApiHost   string
	proxy         string
	keepAliveIdle time.Duration
}

func NewStreamClient(options ...ClientOption) *StreamClient {
	cli := &StreamClient{}

	defaultOptions := []ClientOption{
		WithSubscription(utils.SubscriptionTypeKSystem, "disconnect", cli.OnDisconnect),
		WithSubscription(utils.SubscriptionTypeKSystem, "ping", cli.OnPing),
		WithUserAgent(client.NewDingtalkGoSDKUserAgent()),
		WithAutoReconnect(true),
	}

	for _, option := range defaultOptions {
		option(cli)
	}

	for _, option := range options {
		if option == nil {
			continue
		}

		option(cli)
	}

	return cli
}

func (cli *StreamClient) Start(ctx context.Context, ctxWithTimeout context.Context) error {
	if cli.conn != nil {
		return nil
	}

	cli.mutex.Lock()
	defer cli.mutex.Unlock()

	if cli.conn != nil {
		return nil
	}

	endpoint, err := cli.GetConnectionEndpoint(ctx)
	if err != nil {
		return err
	}

	wssUrl := fmt.Sprintf("%s?ticket=%s", endpoint.Endpoint, endpoint.Ticket)

	header := make(http.Header)

	var dialer *websocket.Dialer

	if len(cli.proxy) == 0 {
		dialer = websocket.DefaultDialer
	} else {
		proxyURL, err := url.Parse(cli.proxy)
		if err != nil {
			return err
		}
		dialer = &websocket.Dialer{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	conn, resp, err := dialer.Dial(wssUrl, header)
	if err != nil {
		return err
	}

	// 建连失败
	if resp.StatusCode >= http.StatusBadRequest {
		return utils.ErrorFromHttpResponseBody(resp)
	}

	cli.conn = conn
	cli.sessionId = endpoint.Ticket

	go cli.processLoop(ctx, ctxWithTimeout)

	return nil
}

func (cli *StreamClient) processLoop(ctx context.Context, ctxWithTimeout context.Context) {
	defer func() {
		if err := recover(); err != nil {
		}
		//if cli.AutoReconnect {
		//	go cli.reconnect()
		//}
	}()

	if cli.conn == nil {
		return
	}

	readChan := make(chan []byte)
	pongChan := make(chan struct{})
	closeChan := make(chan struct{})
	defer func() { close(closeChan) }()
	defer func() { close(pongChan) }()
	defer func() { close(readChan) }()

	cli.conn.SetPongHandler(func(appData string) error {
		pongChan <- struct{}{}
		return nil
	})
	go func() {
		select {
		case <-ctx.Done():
			closeChan <- struct{}{}
		}
	}()
	//开始启动协程读数据
	go func() {
		defer func() {
			if err := recover(); err != nil {
			}
		}()
		for {
			messageType, message, err := cli.conn.ReadMessage()
			if err != nil {
				closeChan <- struct{}{}
				return
			}
			if messageType == websocket.TextMessage {
				readChan <- message
			}
		}
	}()

	//循环处理事件
	for {
		timer := time.NewTimer(cli.keepAliveIdle)
		time.Sleep(500 * time.Millisecond)
		select {
		case msg, ok := <-readChan:
			timer.Stop()
			if ok {
				go cli.processDataFrame(ctx, msg)
			} else {
				return
			}
		case <-timer.C:
			e := cli.conn.WriteMessage(websocket.PingMessage, nil)
			if e != nil {
				return
			}
			go func() {
				select {
				case <-pongChan:
					return
				case <-time.After(5 * time.Second):
					closeChan <- struct{}{}
					return
				case <-ctx.Done():
					return
				}
			}()
		case <-closeChan:
			return
		}
	}
}

func (cli *StreamClient) processDataFrame(ctx context.Context, rawData []byte) {
	defer func() {
		if err := recover(); err != nil {
		}
	}()

	dataFrame, err := payload.DecodeDataFrame(rawData)
	if err != nil {
		return
	}

	if dataFrame == nil || dataFrame.Headers == nil {
		return
	}

	var dataAck *payload.DataFrameResponse
	frameHandler, err := cli.GetHandler(dataFrame.Type, dataFrame.GetTopic())
	if err != nil || frameHandler == nil {
		// 没有注册handler，返回404
		dataAck = payload.NewDataFrameResponse(payload.DataFrameResponseStatusCodeKHandlerNotFound)
	} else {
		dataAck, err = frameHandler(ctx, dataFrame)

		if err != nil && dataAck == nil {
			dataAck = payload.NewErrorDataFrameResponse(err)
		}
	}

	if dataAck == nil {
		dataAck = payload.NewSuccessDataFrameResponse()
	}

	if dataAck.GetHeader(payload.DataFrameHeaderKMessageId) == "" {
		dataAck.SetHeader(payload.DataFrameHeaderKMessageId, dataFrame.GetMessageId())
	}

	if dataAck.GetHeader(payload.DataFrameHeaderKContentType) == "" {
		dataAck.SetHeader(payload.DataFrameHeaderKContentType, payload.DataFrameContentTypeKJson)
	}

	errSend := cli.SendDataFrameResponse(context.Background(), dataAck)
	json.Marshal(dataAck)

	if errSend != nil {
	}
}

func (cli *StreamClient) Close() {
	if cli.conn == nil {
		return
	}

	cli.mutex.Lock()
	defer cli.mutex.Unlock()

	if cli.conn == nil {
		return
	}

	if err := cli.conn.Close(); err != nil {
	} else {
	}
	cli.conn = nil
	cli.sessionId = ""
}

func (cli *StreamClient) reconnect() {
	defer func() {
		if err := recover(); err != nil {
		}
	}()

	cli.Close()

	for {
		err := cli.Start(context.Background(), context.Background())
		if err != nil {
			time.Sleep(time.Second * 3)
		} else {
			return
		}
	}

}

func (cli *StreamClient) GetHandler(stype, stopic string) (handler.IFrameHandler, error) {
	subs := cli.subscriptions[stype]
	if subs == nil || subs[stopic] == nil {
		return nil, errors.New("HandlerNotRegistedForTypeTopic_" + stype + "_" + stopic)
	}

	return subs[stopic], nil
}

func (cli *StreamClient) CheckConfigValid() error {
	if err := cli.AppCredential.Valid(); err != nil {
		return err
	}

	if err := cli.UserAgent.Valid(); err != nil {
		return err
	}

	if cli.subscriptions == nil {
		return errors.New("subscriptionsNil")
	}

	for ttype, subs := range cli.subscriptions {
		if _, ok := utils.SubscriptionTypeSet[ttype]; !ok {
			return errors.New("UnKnownSubscriptionType_" + ttype)
		}

		if len(subs) <= 0 {
			return errors.New("NoHandlersRegistedForType_" + ttype)
		}

		for ttopic, h := range subs {
			if h == nil {
				return errors.New("HandlerNilForTypeTopic_" + ttype + "_" + ttopic)
			}
		}
	}

	return nil
}

func (cli *StreamClient) GetConnectionEndpoint(ctx context.Context) (*payload.ConnectionEndpointResponse, error) {
	if err := cli.CheckConfigValid(); err != nil {
		return nil, err
	}

	requestModel := payload.ConnectionEndpointRequest{
		ClientId:      cli.AppCredential.ClientId,
		ClientSecret:  cli.AppCredential.ClientSecret,
		UserAgent:     cli.UserAgent.UserAgent,
		Subscriptions: make([]*payload.SubscriptionModel, 0),
		Extras:        cli.extras,
	}
	if localIp, err := utils.GetFirstLanIP(); err == nil {
		requestModel.LocalIP = localIp
	}

	for ttype, subs := range cli.subscriptions {
		for ttopic := range subs {
			requestModel.Subscriptions = append(requestModel.Subscriptions, &payload.SubscriptionModel{
				Type:  ttype,
				Topic: ttopic,
			})
		}
	}

	requestJsonBody, _ := json.Marshal(requestModel)

	var targetHost string
	if len(cli.openApiHost) == 0 {
		targetHost = utils.DefaultOpenApiHost
	} else {
		targetHost = cli.openApiHost
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetHost+utils.GetConnectionEndpointAPIUrl, bytes.NewReader(requestJsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var transport http.RoundTripper
	if len(cli.proxy) == 0 {
		transport = http.DefaultTransport
	} else {
		proxyURL, err := url.Parse(cli.proxy)

		if err != nil {
			return nil, err
		}
		transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second, //设置超时，包含connection时间、任意重定向时间、读取response body时间
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, utils.ErrorFromHttpResponseBody(resp)
	}

	defer resp.Body.Close()

	responseJsonBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	endpoint := &payload.ConnectionEndpointResponse{}

	if err := json.Unmarshal(responseJsonBody, endpoint); err != nil {
		return nil, err
	}

	if err := endpoint.Valid(); err != nil {
		return nil, err
	}

	return endpoint, nil
}

func (cli *StreamClient) OnDisconnect(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {

	cli.Close()
	return nil, nil
}

func (cli *StreamClient) OnPing(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	dfPong := payload.NewDataFrameAckPong(df.GetMessageId())
	dfPong.Data = df.Data

	return dfPong, nil
}

// 返回正常数据包
func (cli *StreamClient) SendDataFrameResponse(ctx context.Context, resp *payload.DataFrameResponse) error {
	if resp == nil {
		return errors.New("SendDataFrameResponseError_ResponseNil")
	}

	if cli.conn == nil {
		return errors.New("disconnected")
	}
	return cli.conn.WriteJSON(resp)
}

// 通用注册函数
func (cli *StreamClient) RegisterRouter(stype, stopic string, frameHandler handler.IFrameHandler) {
	if cli.subscriptions == nil {
		cli.subscriptions = make(map[string]map[string]handler.IFrameHandler)
	}

	if _, ok := cli.subscriptions[stype]; !ok {
		cli.subscriptions[stype] = make(map[string]handler.IFrameHandler)
	}

	cli.subscriptions[stype][stopic] = frameHandler
}

// callback类型注册函数
func (cli *StreamClient) RegisterCallbackRouter(topic string, frameHandler handler.IFrameHandler) {
	cli.RegisterRouter(utils.SubscriptionTypeKCallback, topic, frameHandler)
}

// 聊天机器人的注册函数
func (cli *StreamClient) RegisterChatBotCallbackRouter(messageHandler chatbot.IChatBotMessageHandler) {
	cli.RegisterRouter(utils.SubscriptionTypeKCallback, payload.BotMessageCallbackTopic, chatbot.NewDefaultChatBotFrameHandler(messageHandler).OnEventReceived)
}

// AI插件的注册函数
func (cli *StreamClient) RegisterPluginCallbackRouter(messageHandler plugin.IPluginMessageHandler) {
	cli.RegisterRouter(utils.SubscriptionTypeKCallback, payload.PluginMessageCallbackTopic, plugin.NewDefaultPluginFrameHandler(messageHandler).OnEventReceived)
}

// 互动卡片的注册函数
func (cli *StreamClient) RegisterCardCallbackRouter(messageHandler card.ICardCallbackHandler) {
	cli.RegisterRouter(utils.SubscriptionTypeKCallback, payload.CardInstanceCallbackTopic, card.NewDefaultPluginFrameHandler(messageHandler).OnEventReceived)
}

// 事件类型的注册函数
func (cli *StreamClient) RegisterEventRouter(topic string, frameHandler handler.IFrameHandler) {
	cli.RegisterRouter(utils.SubscriptionTypeKEvent, topic, frameHandler)
}

// 所有事件的注册函数
func (cli *StreamClient) RegisterAllEventRouter(frameHandler handler.IFrameHandler) {
	cli.RegisterRouter(utils.SubscriptionTypeKEvent, "*", frameHandler)
}
