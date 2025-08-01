package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/crush/internal/permission"
	"github.com/charmbracelet/crush/internal/types"
	"github.com/charmbracelet/crush/internal/utilFunction"
	"strconv"
	"sync"
	"time"
)

type DingTalkParams struct {
	NodeId string `json:"NodeId"`
}

type dingTalkTool struct {
	workingDir  string
	permissions permission.Service
}

const (
	DingTalkToolName        = "read_alidocs_dingtalk_Document"
	DingTalkToolDescription = `### Read Alidocs Dingtalk Document

**WHEN TO USE THIS TOOL:**
- Use when you need to programmatically fetch the content of a specific DingTalk document.
- Helpful for integrating DingTalk document content into other systems or workflows.
- Useful for archiving or processing a document's content in Markdown format.

**HOW TO USE:**
- Provide the **NodeId** of the DingTalk document you want to retrieve.
- The tool will fetch the document's content and return it as a Markdown string.

**QUERY SYNTAX:**
- The tool requires a single parameter: **'NodeId'**
- The 'NodeId' is a unique identifier for a DingTalk document. You can find it in the document's URL.

**Example:**
For the URL: 'https://alidocs.dingtalk.com/i/nodes/Obva6QBXJw9wGz6OUND626jOWn4qY5Pr?doc_type=wiki_doc&iframeQuery=utm_source=portal&utm_medium=portal_recent&rnd=0.6590210840452759'

The NodeID is the string directly after '/nodes/': **'Obva6QBXJw9wGz6OUND626jOWn4qY5Pr'**.

**EXAMPLES:**
- 'NodeId:Obva6QBXJw9wGz6OUND626jOWn4qY5Pr' - Retrieve the content of the document with this specific NodeID.

**LIMITATIONS:**
- Requires proper authentication and permissions to access the document. The tool may fail if the document is private or you do not have permission to view it.
- Only fetches the content of the main document. It does not include comments, revisions, or other metadata.
- The output is in Markdown format, which may not perfectly preserve all rich text formatting, styles, or embedded objects from the original document.

**TIPS:**
- To find the NodeID, always look for the string that follows '/nodes/' in the document's URL.`
)

func NewDingTalkTool(permissions permission.Service, workingDir string) BaseTool {
	return &dingTalkTool{
		workingDir:  workingDir,
		permissions: permissions,
	}
}

func (v *dingTalkTool) Name() string {
	return DingTalkToolName
}

func (v *dingTalkTool) Info() ToolInfo {
	return ToolInfo{
		Name:        DingTalkToolName,
		Description: DingTalkToolDescription,
		Parameters: map[string]any{
			"NodeId": map[string]any{
				"type":        "string",
				"description": "钉钉文档的NodeId，例如：https://alidocs.dingtalk.com/i/nodes/Obva6QBXJw9wGz6OUND626jOWn4qY5Pr?doc_type=wiki_doc&iframeQuery=utm_source=portal&utm_medium=portal_recent&rnd=0.6590210840452759,这个钉钉文档的NodeID就是'Obva6QBXJw9wGz6OUND626jOWn4qY5Pr'",
			},
		},
		Required: []string{"NodeId"},
	}
}

// Run implements Tool.
func (v *dingTalkTool) Run(ctx context.Context, call ToolCall) (ToolResponse, error) {
	var params DingTalkParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return NewTextErrorResponse(fmt.Sprintf("error parsing parameters: %s", err)), nil
	}

	if params.NodeId == "" {
		return NewTextErrorResponse("NodeId is required"), nil
	}

	myctx := context.WithValue(context.Background(), types.ConfigKey, GetNewLLMContext())
	MarkdownStr := ""
	value := myctx.Value(types.ConfigKey).(*types.LLMConfig)
	talkChan := value.DingTalkChan
	client, _ := utilFunction.NewDingTalkClient()
	go client.TestDingDingFunction(myctx, utilFunction.GetContextWithTimeOut(time.Second*20))
	//unionid := client.GetUnionid("宋睿")
	//DocNodeInfo := client.GetDocNodeInfo(unionid, params.Url)
	response, _ := utilFunction.DownLoadDocInit(client.AccessToken, params.NodeId)
	// 声明一个 WaitGroup 和一个上下文
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second) // 设置5秒超时
	defer cancel()                                                          // 确保在函数退出时调用 cancel
	nocontent := false

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case update, ok := <-*talkChan:
				if !ok {
					// 通道已关闭
					return
				}
				// 处理接收到的数据
				parseInt, _ := strconv.ParseInt(update.TaskID, 10, 64)
				if response.Result.TaskId == parseInt {
					file, err := utilFunction.DownloadMarkdownFile(update.URL)
					if err != nil {
						fmt.Println(err.Error())
					}
					MarkdownStr = file
					return // 找到并处理后退出协程
				}
			case <-ctx.Done():
				// 超时或上下文被取消，退出协程
				nocontent = true
				return
			}
		}
	}()
	wg.Wait()
	if nocontent {
		return NewTextResponse("文档不存在或无权限，请让文档所有人添加“导出文档内容AI助理”作为可查看/下载权限"), nil

	}
	return NewTextResponse(MarkdownStr), nil
}
func GetNewLLMContext() *types.LLMConfig {
	// 1. 先创建通道和切片的实例

	llmContextStringChan := make(chan string, 100)
	userContextStringChan := make(chan string, 100)
	dingTalkChan := make(chan types.DocExportEvent, 50)
	// 2. 然后将这些实例的地址赋值给 LLMConfig 的指针字段
	llmConfig := &types.LLMConfig{
		DingTalkChan:          &dingTalkChan, // 取通道的地址
		LLMContextStringChan:  &llmContextStringChan,
		UserContextStringChan: &userContextStringChan,
	}
	return llmConfig
}
