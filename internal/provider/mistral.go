package provider

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"teo/internal/tools"
	"time"

	"github.com/go-resty/resty/v2"
)

type MistralChoice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	Delta        Message `json:"delta,omitempty"`
	FinishReason string  `json:"finish_reason"`
}

type MistralUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type MistralChatCompletion struct {
	ID                string          `json:"id"`
	Object            string          `json:"object"`
	Created           int64           `json:"created"`
	Model             string          `json:"model"`
	SystemFingerprint string          `json:"system_fingerprint"`
	Choices           []MistralChoice `json:"choices"`
	Usage             MistralUsage    `json:"usage"`
}

type MistralRequest struct {
	Model      string                   `json:"model"`
	Messages   []Message                `json:"messages"`
	Stream     bool                     `json:"stream"`
	Tools      []map[string]interface{} `json:"tools,omitempty"`
	ToolChoice string                   `json:"tool_choice,omitempty"`
}

type MistralModels struct {
	Object string         `json:"object"`
	Data   []MistralModel `json:"data"`
}

type MistralModel struct {
	ID               string              `json:"id"`
	Object           string              `json:"object"`
	Created          int64               `json:"created"`
	OwnedBy          string              `json:"owned_by"`
	Name             string              `json:"name"`
	Description      string              `json:"description"`
	MaxContextLength int                 `json:"max_context_length"`
	Aliases          []string            `json:"aliases"`
	Deprecation      *string             `json:"deprecation"`
	Capabilities     MistralCapabilities `json:"capabilities"`
	Type             string              `json:"type"`
}

type MistralCapabilities struct {
	CompletionChat  bool `json:"completion_chat"`
	CompletionFim   bool `json:"completion_fim"`
	FunctionCalling bool `json:"function_calling"`
	FineTuning      bool `json:"fine_tuning"`
	Vision          bool `json:"vision"`
}

type MistralProvider struct {
	baseURL      string
	apiKey       string
	defaultModel string
}

func NewMistralProvider(baseURL string, apiKey string, defaultModel string) LLMProvider {
	return &MistralProvider{
		baseURL:      baseURL,
		apiKey:       apiKey,
		defaultModel: defaultModel,
	}
}

func (m *MistralProvider) ProviderName() string {
	return "mistral"
}

func (m *MistralProvider) DefaultModel(modelName string) string {
	if modelName == "" {
		return m.defaultModel
	}
	return modelName
}

func (m *MistralProvider) Chat(modelName string, messages []Message) (Message, error) {
	client := resty.New()
	client.SetTimeout(120 * time.Second)

	request := MistralRequest{
		Model:      m.DefaultModel(modelName),
		Stream:     false,
		Messages:   messages,
		Tools:      tools.GetTools(),
		ToolChoice: "auto",
	}

	var response MistralChatCompletion
	res, _ := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", m.apiKey)).
		SetBody(request).
		SetResult(&response).
		Post(m.baseURL + "/v1/chat/completions")

	if res.StatusCode() != 200 {
		return Message{}, fmt.Errorf("error fetching response: %v", res.String())
	}

	if response.Choices[0].FinishReason == "tool_calls" {
		resp_tool := toolCalls(messages, response.Choices[0].Message)
		return m.Chat(modelName, resp_tool)
	}

	return response.Choices[0].Message, nil
}

func (m *MistralProvider) ChatStream(modelName string, messages []Message, callback func(Message) error) error {
	client := resty.New()
	client.SetTimeout(120 * time.Second)

	request := MistralRequest{
		Model:      m.DefaultModel(modelName),
		Stream:     true,
		Messages:   messages,
		Tools:      tools.GetTools(),
		ToolChoice: "auto",
	}

	res, _ := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", m.apiKey)).
		SetBody(request).
		SetDoNotParseResponse(true).
		Post(m.baseURL + "/v1/chat/completions")

	defer res.RawBody().Close()

	reader := bufio.NewReader(res.RawBody())
	var response MistralChatCompletion
	for {
		line, err := reader.ReadString('\n')

		if res.StatusCode() != 200 {
			return fmt.Errorf("error fetching stream response: %v", line)
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading stream: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "[DONE]" {
			break
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		jsonData := strings.TrimPrefix(line, "data: ")

		err = json.Unmarshal([]byte(jsonData), &response)
		if err != nil {
			return fmt.Errorf("error unmarshalling stream data: %w", err)
		}

		partialMessage := response.Choices[0].Delta
		err = callback(partialMessage)
		if err != nil {
			return fmt.Errorf("error in callback: %w", err)
		}

		if response.Choices[0].FinishReason == "tool_calls" {
			partialMessage.Role = "assistant"
			resp_tool := toolCalls(messages, partialMessage)
			return m.ChatStream(modelName, resp_tool, callback)
		}

		if response.Choices[0].FinishReason == "stop" {
			break
		}
	}

	return nil
}

func (m *MistralProvider) Models() ([]string, error) {
	response, err := m.mistralModels()
	if err != nil {
		return nil, err
	}

	var models []string
	for _, model := range response.Data {
		models = append(models, model.ID)
	}

	return models, nil
}

func (m *MistralProvider) mistralModels() (*MistralModels, error) {
	client := resty.New()

	var response MistralModels
	res, _ := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", fmt.Sprintf("Bearer %s", m.apiKey)).
		SetResult(&response).
		Get(m.baseURL + "/v1/models")

	if res.StatusCode() != 200 {
		return nil, fmt.Errorf("error fetching mistral models: %s", res.String())
	}

	return &response, nil
}
