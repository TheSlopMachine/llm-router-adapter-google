package google

import (
	"fmt"
	"time"

	sdk "github.com/TheSlopMachine/llm-router-sdk"
)

func transformRequest(req *sdk.ChatCompletionRequest) *GenerateContentRequest {
	googleReq := &GenerateContentRequest{
		Contents: make([]Content, 0),
	}

	var currentContent *Content
	var currentRole string

	for _, msg := range req.Messages {
		role := msg.Role
		if role == "system" {
			role = "user"
		} else if role == "assistant" {
			role = "model"
		}

		if currentContent == nil || currentRole != role {
			if currentContent != nil {
				googleReq.Contents = append(googleReq.Contents, *currentContent)
			}
			currentContent = &Content{
				Role:  role,
				Parts: make([]Part, 0),
			}
			currentRole = role
		}

		currentContent.Parts = append(currentContent.Parts, Part{
			Text: msg.Content,
		})
	}

	if currentContent != nil {
		googleReq.Contents = append(googleReq.Contents, *currentContent)
	}

	if req.MaxTokens > 0 || req.Temperature > 0 || req.TopP > 0 {
		googleReq.GenerationConfig = &GenerationConfig{}
		if req.MaxTokens > 0 {
			maxTokens := req.MaxTokens
			googleReq.GenerationConfig.MaxOutputTokens = &maxTokens
		}
		if req.Temperature > 0 {
			temp := req.Temperature
			googleReq.GenerationConfig.Temperature = &temp
		}
		if req.TopP > 0 {
			topP := req.TopP
			googleReq.GenerationConfig.TopP = &topP
		}
	}

	return googleReq
}

func transformResponse(googleResp *GenerateContentResponse, modelID string) *sdk.ChatCompletionResponse {
	resp := &sdk.ChatCompletionResponse{
		ID:      fmt.Sprintf("google-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   modelID,
		Choices: make([]sdk.ChatCompletionChoice, 0),
	}

	if len(googleResp.Candidates) > 0 {
		candidate := googleResp.Candidates[0]
		text := ""
		if len(candidate.Content.Parts) > 0 {
			text = candidate.Content.Parts[0].Text
		}

		resp.Choices = append(resp.Choices, sdk.ChatCompletionChoice{
			Index: candidate.Index,
			Message: sdk.ChatMessage{
				Role:    "assistant",
				Content: text,
			},
			FinishReason: mapFinishReason(candidate.FinishReason),
		})
	}

	if googleResp.UsageMetadata != nil {
		resp.Usage = sdk.ChatCompletionUsage{
			PromptTokens:     googleResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: googleResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      googleResp.UsageMetadata.TotalTokenCount,
		}
	}

	return resp
}

func transformStreamChunk(googleResp *GenerateContentResponse, modelID string) *sdk.StreamChunk {
	chunk := &sdk.StreamChunk{
		ID:      fmt.Sprintf("google-%d", time.Now().Unix()),
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   modelID,
		Choices: make([]sdk.StreamChunkChoice, 0),
	}

	if len(googleResp.Candidates) > 0 {
		candidate := googleResp.Candidates[0]
		text := ""
		if len(candidate.Content.Parts) > 0 {
			text = candidate.Content.Parts[0].Text
		}

		var finishReason *string
		if candidate.FinishReason != "" {
			mapped := mapFinishReason(candidate.FinishReason)
			finishReason = &mapped
		}

		chunk.Choices = append(chunk.Choices, sdk.StreamChunkChoice{
			Index: candidate.Index,
			Delta: sdk.ChatMessage{
				Role:    "assistant",
				Content: text,
			},
			FinishReason: finishReason,
		})
	}

	return chunk
}

func mapFinishReason(googleReason string) string {
	switch googleReason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION":
		return "content_filter"
	case "OTHER":
		return "stop"
	default:
		return "stop"
	}
}
