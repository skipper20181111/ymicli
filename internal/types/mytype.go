package types

const (
	ConfigKey    = "config"
	AbsolutePath = "AbsolutePath"
)

type LLMConfig struct {
	LLMContextStringChan  *chan string         `json:"llm_context_string_chan"`
	UserContextStringChan *chan string         `json:"user_context_string_chan"`
	DingTalkChan          *chan DocExportEvent `json:"DingTalkChan"`
}
type DocExportEvent struct {
	EventID    string `json:"eventId"`    // 事件的唯一标识符
	Extension  string `json:"extension"`  // 文件扩展名，例如 "adoc"
	UnionID    string `json:"unionId"`    // 联合ID，通常用于标识用户或组织
	Success    bool   `json:"success"`    // 操作是否成功
	DentryUUID string `json:"dentryUuid"` // 文档条目的唯一通用标识符
	Format     string `json:"format"`     // 导出的文件格式，例如 "markdown"
	Name       string `json:"name"`       // 文件名，例如 "ocr提额qa1测试0630.adoc"
	Type       string `json:"type"`       // 事件类型，例如 "DOC_EXPORT_FOR_OPEN_PLATFORM"
	Version    int    `json:"version"`    // 文档版本号
	Operation  string `json:"operation"`  // 执行的操作，例如 "file_export"
	URL        string `json:"url"`        // 导出文件可访问的URL
	TaskID     string `json:"taskId"`     // 任务的唯一标识符
}
