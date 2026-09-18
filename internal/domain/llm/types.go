package llm

import "encoding/json"

type Role string

const (
	RoleSystem    Role = "system"
	RoleDeveloper Role = "developer"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type ContentBlock interface {
	isContentBlock()
}

type URLCitation struct {
	StartIndex int
	EndIndex   int
	URL        string
	Title      string
}

type TextBlock struct {
	Text      string
	Citations []URLCitation
}

func (TextBlock) isContentBlock() {}

type WebSearchCallBlock struct {
	Queries []string
}

func (WebSearchCallBlock) isContentBlock() {}

type AudioBlock struct {
	MediaType string
}

func (AudioBlock) isContentBlock() {}

type MediaSourceType string

const (
	MediaSourceURL    MediaSourceType = "url"
	MediaSourceBase64 MediaSourceType = "base64"
	MediaSourceFile   MediaSourceType = "file"
)

type MediaSource struct {
	Type      MediaSourceType
	URL       string
	MediaType string
	Data      string
	FileID    string
}

type ImageBlock struct {
	Source  MediaSource
	AltText string
}

func (ImageBlock) isContentBlock() {}

type DocumentBlock struct {
	Source  MediaSource
	Name    string
	Context string
}

func (DocumentBlock) isContentBlock() {}

type ToolCallBlock struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

func (ToolCallBlock) isContentBlock() {}

type ToolResultBlock struct {
	ToolCallID string
	Name       string
	Content    []ContentBlock
	IsError    bool
}

func (ToolResultBlock) isContentBlock() {}

type ReasoningBlock struct {
	Text         string
	Signature    string
	RedactedData string
	Metadata     map[string]json.RawMessage
}

func (ReasoningBlock) isContentBlock() {}

type Message struct {
	Role     Role
	Name     string
	Content  []ContentBlock
	Metadata map[string]json.RawMessage
}

type ToolKind string

const (
	ToolKindFunction  ToolKind = "function"
	ToolKindWebSearch ToolKind = "web_search"
)

type Tool struct {
	Kind        ToolKind
	Name        string
	Description string
	InputSchema json.RawMessage
	Metadata    map[string]json.RawMessage
}

type ToolChoiceMode string

const (
	ToolChoiceAuto     ToolChoiceMode = "auto"
	ToolChoiceRequired ToolChoiceMode = "required"
	ToolChoiceNone     ToolChoiceMode = "none"
	ToolChoiceNamed    ToolChoiceMode = "named"
)

type ToolChoice struct {
	Mode            ToolChoiceMode
	Name            string
	DisableParallel bool
}

type ReasoningConfig struct {
	Enabled      bool
	Mode         string
	BudgetTokens int
	Effort       string
	Summary      string
	Metadata     map[string]json.RawMessage
}

type ResponseFormat struct {
	Name        string
	Description string
	JSONSchema  json.RawMessage
	Strict      bool
}

type ResponseState struct {
	PreviousResponseID string
	ConversationID     string
	Store              bool
	InstructionMessages int
}

type Request struct {
	Model           string
	Messages        []Message
	Tools           []Tool
	ToolChoice      *ToolChoice
	Reasoning       *ReasoningConfig
	ResponseFormat  *ResponseFormat
	ResponseState   *ResponseState
	MaxOutputTokens *int
	Temperature     *float64
	TopP            *float64
	Stop            []string
	Metadata        map[string]json.RawMessage
}

type StopReason string

const (
	StopReasonEndTurn      StopReason = "end_turn"
	StopReasonMaxTokens    StopReason = "max_tokens"
	StopReasonStopSequence StopReason = "stop_sequence"
	StopReasonToolUse      StopReason = "tool_use"
	StopReasonContentBlock StopReason = "content_filter"
	StopReasonUnknown      StopReason = "unknown"
)

type Usage struct {
	InputTokens      int64
	OutputTokens     int64
	ReasoningTokens  int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	Metadata         map[string]json.RawMessage
}

type Response struct {
	ID                 string
	PreviousResponseID string
	ConversationID     string
	Model        string
	Content      []ContentBlock
	StopReason   StopReason
	StopSequence string
	Usage        Usage
	Metadata     map[string]json.RawMessage
}
