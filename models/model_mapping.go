package models

import (
	"time"
)

type UpstreamType string

const (
	UpstreamTypeOpenAIChat        UpstreamType = "openai_chat"
	UpstreamTypeOpenAIResponses   UpstreamType = "openai_responses"
	UpstreamTypeAnthropicMessages UpstreamType = "anthropic_messages"
	DefaultUpstreamType                        = UpstreamTypeOpenAIChat
)

type ModelConfig struct {
	ID                        uint         `gorm:"primarykey" json:"id"`
	Name                      string       `gorm:"not null;type:varchar(255);index" json:"name"`
	ModelName                 string       `gorm:"not null;type:varchar(255)" json:"model_name"`
	APIBaseURL                string       `gorm:"not null;type:varchar(500)" json:"api_base_url"`
	APIKey                    string       `gorm:"not null;type:varchar(500)" json:"api_key"`
	UpstreamType              UpstreamType `gorm:"not null;type:varchar(64);default:'openai_chat';index" json:"upstream_type"`
	MaxTokens                 int          `gorm:"default:8192" json:"max_tokens"`
	Priority                  int          `gorm:"default:0;index" json:"priority"`
	MaxConcurrency            int          `gorm:"default:0" json:"max_concurrency"`
	Temperature               float64      `gorm:"default:0.7" json:"temperature"`
	Description               string       `gorm:"type:varchar(500)" json:"description"`
	SupportsTools             bool         `gorm:"default:false" json:"supports_tools"`
	SupportsStream            bool         `gorm:"default:true" json:"supports_stream"`
	SupportsReasoning         bool         `gorm:"default:false" json:"supports_reasoning"`
	SupportsJSONSchema        bool         `gorm:"default:false" json:"supports_json_schema"`
	SupportsVision            bool         `gorm:"default:false" json:"supports_vision"`
	SupportsParallelToolCalls bool         `gorm:"default:false" json:"supports_parallel_tool_calls"`
	SupportsRefusal           bool         `gorm:"default:false" json:"supports_refusal"`
	SupportsAnnotations       bool         `gorm:"default:false" json:"supports_annotations"`
	SupportsAudioOutput       bool         `gorm:"default:false" json:"supports_audio_output"`
	SupportsWebSearch         bool         `gorm:"default:false" json:"supports_web_search"`
	SupportsMCP               bool         `gorm:"default:false" json:"supports_mcp"`
	SupportsCodeInterpreter   bool         `gorm:"default:false" json:"supports_code_interpreter"`
	SupportsImageGeneration   bool         `gorm:"default:false" json:"supports_image_generation"`
	SupportsPromptCache       bool         `gorm:"default:false" json:"supports_prompt_cache"`
	CreatedAt                 time.Time    `json:"created_at"`
	UpdatedAt                 time.Time    `json:"updated_at"`
	Enabled                   bool         `gorm:"default:true" json:"enabled"`
}

func (ModelConfig) TableName() string {
	return "model_configs"
}
