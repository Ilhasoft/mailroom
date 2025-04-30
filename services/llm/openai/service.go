package openai

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/nyaruka/goflow/flows"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/responses"
	"github.com/openai/openai-go/shared"
)

const (
	TypeOpenAI = "openai"

	configAPIKey = "api_key"
)

func init() {
	models.RegisterLLMService(TypeOpenAI, New)
}

// an LLM service implementation for OpenAI
type service struct {
	client openai.Client
	model  string
}

func New(m *models.LLM, c *http.Client) (flows.LLMService, error) {
	apiKey := m.Config().GetString(configAPIKey, "")
	if apiKey == "" {
		return nil, fmt.Errorf("config incomplete for LLM: %s", m.UUID())
	}

	return &service{
		client: openai.NewClient(option.WithAPIKey(apiKey), option.WithHTTPClient(c)),
		model:  m.Model(),
	}, nil
}

func (s *service) Response(ctx context.Context, instructions, input string, maxTokens int) (*flows.LLMResponse, error) {
	resp, err := s.client.Responses.New(ctx, responses.ResponseNewParams{
		Model:        shared.ResponsesModel(s.model),
		Instructions: openai.String(instructions),
		Input: responses.ResponseNewParamsInputUnion{
			OfString: openai.String(input),
		},
		Temperature:     openai.Float(0.000001),
		MaxOutputTokens: openai.Int(int64(maxTokens)),
	})
	if err != nil {
		return nil, fmt.Errorf("error calling OpenAI API: %w", err)
	}

	return &flows.LLMResponse{
		Output:     strings.TrimSpace(resp.OutputText()),
		TokensUsed: resp.Usage.TotalTokens,
	}, nil
}
