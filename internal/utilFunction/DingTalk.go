package utilFunction

import (
	"context"
	"encoding/json"
	"github.com/charmbracelet/crush/internal/types"
	"github.com/charmbracelet/crush/internal/utilFunction/myclient"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/event"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
)

func (d *DingTalkClient) TestDingDingFunction(ctx context.Context, ctxWithTimeout context.Context) {
	ClientId := "dingew4tmkpzy7vyjdsy"
	ClientSecret := "AP1dzVGdlTa5fq5xu5G4hH7Gfk6tFMeE7tSTbZYqwfW4enSzjFS-wuwE0JmYLbT-"
	cli := myclient.NewStreamClient(myclient.WithAppCredential(client.NewAppCredentialConfig(ClientId, ClientSecret)))
	//注册事件类型的处理函数
	cli.RegisterAllEventRouter(OnEventReceived)
	err := cli.Start(ctx, ctxWithTimeout)
	if err != nil {
		panic(err)
	}

	defer cli.Close()

	select {
	case <-ctxWithTimeout.Done(): // 监听上下文的取消信号
		return // 收到信号后退出协程
	}
}

func OnEventReceived(ctx context.Context, df *payload.DataFrame) (frameResp *payload.DataFrameResponse, err error) {
	eventHeader := event.NewEventHeaderFromDataFrame(df)
	value := ctx.Value(types.ConfigKey).(*types.LLMConfig)

	talkChan := value.DingTalkChan
	//data := df.Data
	logger.GetLogger().Infof("received event, eventId=[%s] eventBornTime=[%d] eventCorpId=[%s] eventType=[%s] eventUnifiedAppId=[%s] data=[%s]",
		eventHeader.EventId,
		eventHeader.EventBornTime,
		eventHeader.EventCorpId,
		eventHeader.EventType,
		eventHeader.EventUnifiedAppId,
		df.Data)
	DocExportEvent := types.DocExportEvent{}
	json.Unmarshal([]byte(df.Data), &DocExportEvent)
	if DocExportEvent.URL != "" {
		*talkChan <- DocExportEvent
	}
	frameResp = payload.NewSuccessDataFrameResponse()
	if err := frameResp.SetJson(event.NewEventProcessResultSuccess()); err != nil {
		return nil, err
	}

	return
}
