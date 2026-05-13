package google

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	sdk "github.com/TheSlopMachine/llm-router-sdk"
)

const (
	baseURL = "https://generativelanguage.googleapis.com/v1beta"
)

func init() {
	sdk.Register(&Adapter{})
}

type Adapter struct {
	client *Client
}

func (a *Adapter) TypeKey() string {
	return "google"
}

func (a *Adapter) AuthType() sdk.AuthType {
	return sdk.AuthTypeAPIKey
}

func (a *Adapter) ValidateCredentials(data map[string]string) error {
	apiKey := data["api_key"]
	if apiKey == "" {
		return fmt.Errorf("google: api_key is required")
	}
	if len(apiKey) < 20 {
		return fmt.Errorf("google: api_key appears invalid (too short)")
	}
	return nil
}

func (a *Adapter) Complete(
	ctx context.Context,
	cred *sdk.Credential,
	req *sdk.ChatCompletionRequest,
) (*sdk.ChatCompletionResponse, error) {
	apiKey := cred.Data["api_key"]

	_, modelName, err := req.Model.Parse()
	if err != nil {
		return nil, fmt.Errorf("invalid model id: %w", err)
	}

	googleReq := transformRequest(req)

	if a.client == nil {
		a.client = newClient(baseURL)
	}

	googleResp, err := a.client.generateContent(ctx, apiKey, modelName, googleReq)
	if err != nil {
		return nil, err
	}

	return transformResponse(googleResp, string(req.Model)), nil
}

func (a *Adapter) CompleteStream(
	ctx context.Context,
	cred *sdk.Credential,
	req *sdk.ChatCompletionRequest,
	w io.Writer,
) error {
	apiKey := cred.Data["api_key"]

	_, modelName, err := req.Model.Parse()
	if err != nil {
		return fmt.Errorf("invalid model id: %w", err)
	}

	googleReq := transformRequest(req)

	if a.client == nil {
		a.client = newClient(baseURL)
	}

	resp, err := a.client.streamGenerateContent(ctx, apiKey, modelName, googleReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return parseGoogleError(resp.StatusCode, body)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var googleResp GenerateContentResponse
		if err := json.Unmarshal([]byte(data), &googleResp); err != nil {
			continue
		}

		chunk := transformStreamChunk(&googleResp, string(req.Model))

		chunkJSON, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", chunkJSON)

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")

	return scanner.Err()
}

func (a *Adapter) NeedsRefresh(cred *sdk.Credential) bool {
	return false
}

func (a *Adapter) RefreshCredential(
	ctx context.Context,
	cred *sdk.Credential,
) (*sdk.Credential, error) {
	return nil, sdk.ErrNoRefreshNeeded
}

func (a *Adapter) GetModelInfos(
	ctx context.Context,
	cred *sdk.Credential,
	providerQualifier string,
) ([]sdk.ModelInfo, error) {
	apiKey := cred.Data["api_key"]

	if a.client == nil {
		a.client = newClient(baseURL)
	}

	models, err := a.client.listModels(ctx, apiKey)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	var flashRPM, flashTPM, flashRPD int64
	var proRPM, proTPM, proRPD int64
	var flashModelFound, proModelFound bool

	for _, model := range models {
		modelName := extractModelName(model)
		if !flashModelFound && strings.Contains(strings.ToLower(modelName), "flash") {
			flashRPM, flashTPM, flashRPD, _ = a.client.extractRateLimits(ctx, apiKey, modelName)
			flashModelFound = true
		}
		if !proModelFound && strings.Contains(strings.ToLower(modelName), "pro") {
			proRPM, proTPM, proRPD, _ = a.client.extractRateLimits(ctx, apiKey, modelName)
			proModelFound = true
		}
		if flashModelFound && proModelFound {
			break
		}
	}

	modelInfos := make([]sdk.ModelInfo, 0)

	for _, model := range models {
		modelName := extractModelName(model)
		if !isValidLLMModel(modelName, model.SupportedGenerationMethods) {
			continue
		}

		var rpm, tpm, rpd int64

		if strings.Contains(strings.ToLower(modelName), "pro") {
			rpm = proRPM
			tpm = proTPM
			rpd = proRPD
		} else {
			rpm = flashRPM
			tpm = flashTPM
			rpd = flashRPD
		}

		if rpm == 0 {
			rpm = estimateRPM(modelName)
			tpm = estimateTPM(modelName)
			rpd = estimateRPD(modelName)
		}

		displayName := model.DisplayName
		if displayName == "" {
			displayName = modelName
		}

		modelInfos = append(modelInfos, sdk.ModelInfo{
			Name:          modelName,
			DisplayName:   displayName,
			RPM:           rpm,
			TPM:           tpm,
			RPD:           rpd,
			ContextWindow: int64(model.InputTokenLimit),
			MaxTokens:     int64(model.OutputTokenLimit),
		})
	}

	return modelInfos, nil
}

func extractModelName(model ModelMetadata) string {
	if model.Name != "" {
		return strings.TrimPrefix(model.Name, "models/")
	}
	if model.BaseModelId != "" {
		return model.BaseModelId
	}
	return "unknown-model"
}

func (a *Adapter) GetAuthFlow() sdk.AuthFlowHandler {
	return &GoogleAuthFlow{}
}

func (a *Adapter) GetDefaultProviders() []sdk.ProviderInfo {
	return []sdk.ProviderInfo{
		{
			Name:      "Google AI Studio",
			Qualifier: "",
			BaseURL:   baseURL,
			IconURL:   "https://www.gstatic.com/lamda/images/favicon_v1_150160cddff7f294ce30.svg",
		},
	}
}

type GoogleAuthFlow struct{}

func (f *GoogleAuthFlow) InitiateFlow(ctx sdk.AuthFlowContext) (sdk.AuthFlowState, error) {
	return sdk.AuthFlowState{
		RenderHTML: `
<div class="auth-flow-content">
	<p><strong>Google AI Studio API Key</strong></p>
	<p>Get your API key from <a href="https://aistudio.google.com/app/apikey" target="_blank">Google AI Studio</a></p>
	<div class="form-group">
		<label for="api_key">API Key</label>
		<input type="text" id="api_key" name="api_key" class="form-control" placeholder="AIza..." required />
	</div>
	<button type="submit" class="btn btn-primary">Add Credential</button>
</div>`,
	}, nil
}

func (f *GoogleAuthFlow) HandleStep(ctx sdk.AuthFlowContext, input map[string][]string) (sdk.AuthFlowState, error) {
	apiKeyValues, ok := input["api_key"]
	if !ok || len(apiKeyValues) == 0 {
		return sdk.AuthFlowState{
			RenderHTML: `
<div class="auth-flow-content">
	<div class="alert alert-danger">API key is required</div>
	<p><strong>Google AI Studio API Key</strong></p>
	<p>Get your API key from <a href="https://aistudio.google.com/app/apikey" target="_blank">Google AI Studio</a></p>
	<div class="form-group">
		<label for="api_key">API Key</label>
		<input type="text" id="api_key" name="api_key" class="form-control" placeholder="AIza..." required />
	</div>
	<button type="submit" class="btn btn-primary">Add Credential</button>
</div>`,
		}, nil
	}

	apiKey := strings.TrimSpace(apiKeyValues[0])
	if apiKey == "" {
		return sdk.AuthFlowState{
			RenderHTML: `
<div class="auth-flow-content">
	<div class="alert alert-danger">API key cannot be empty</div>
	<p><strong>Google AI Studio API Key</strong></p>
	<p>Get your API key from <a href="https://aistudio.google.com/app/apikey" target="_blank">Google AI Studio</a></p>
	<div class="form-group">
		<label for="api_key">API Key</label>
		<input type="text" id="api_key" name="api_key" class="form-control" placeholder="AIza..." required />
	</div>
	<button type="submit" class="btn btn-primary">Add Credential</button>
</div>`,
		}, nil
	}

	if len(apiKey) < 20 {
		return sdk.AuthFlowState{
			RenderHTML: `
<div class="auth-flow-content">
	<div class="alert alert-danger">API key appears invalid (too short, must be at least 20 characters)</div>
	<p><strong>Google AI Studio API Key</strong></p>
	<p>Get your API key from <a href="https://aistudio.google.com/app/apikey" target="_blank">Google AI Studio</a></p>
	<div class="form-group">
		<label for="api_key">API Key</label>
		<input type="text" id="api_key" name="api_key" class="form-control" placeholder="AIza..." required />
	</div>
	<button type="submit" class="btn btn-primary">Add Credential</button>
</div>`,
		}, nil
	}

	return sdk.AuthFlowState{
		Credentials: map[string]string{
			"api_key": apiKey,
		},
	}, nil
}

func isValidLLMModel(modelName string, methods []string) bool {
	if !supportsGenerateContent(methods) {
		return false
	}

	nameLower := strings.ToLower(modelName)
	if strings.Contains(nameLower, "embedding") {
		return false
	}
	if strings.Contains(nameLower, "aqa") {
		return false
	}

	return true
}

func supportsGenerateContent(methods []string) bool {
	for _, method := range methods {
		if method == "generateContent" {
			return true
		}
	}
	return false
}

func estimateRPM(baseModelId string) int64 {
	baseModelLower := strings.ToLower(baseModelId)
	if strings.Contains(baseModelLower, "flash") {
		if strings.Contains(baseModelLower, "lite") {
			return 2000
		}
		return 1000
	}
	if strings.Contains(baseModelLower, "pro") {
		return 500
	}
	return 1000
}

func estimateTPM(baseModelId string) int64 {
	baseModelLower := strings.ToLower(baseModelId)
	if strings.Contains(baseModelLower, "flash") {
		if strings.Contains(baseModelLower, "lite") {
			return 2000000
		}
		return 1000000
	}
	if strings.Contains(baseModelLower, "pro") {
		return 500000
	}
	return 1000000
}

func estimateRPD(baseModelId string) int64 {
	baseModelLower := strings.ToLower(baseModelId)
	if strings.Contains(baseModelLower, "flash") {
		if strings.Contains(baseModelLower, "lite") {
			return 3000
		}
		return 1500
	}
	if strings.Contains(baseModelLower, "pro") {
		return 1000
	}
	return 1500
}

var _ sdk.Adapter = (*Adapter)(nil)
