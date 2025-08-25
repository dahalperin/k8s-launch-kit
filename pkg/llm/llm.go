package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nvidia/k8s-launch-kit/pkg/config"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func SelectPrompt(promptPath string, config config.ClusterConfig) (map[string]string, error) {
	// Initialize LLM
	llm, err := openai.New(
		openai.WithAPIType(openai.APITypeAzure),
		openai.WithToken("eyJhbGciOiJIUzI1NiJ9.eyJpZCI6IjMxMGZlNjA0LWY2YmUtNDEyYy05ZWE4LWZlZjI3ZmQ0NzRlMCIsInNlY3JldCI6IlUwWkZyZ3k0dis1bGlJQWx2VWZweXBxM1NmYmZPb3lmSzVlNGY4b2pMUEU9In0.n4H3Wbl8H15TGlTEd9jil5J1mFxjRRCMXM3JnXg3rc8"),
		openai.WithBaseURL("https://llm-proxy.perflab.nvidia.com"),
		openai.WithModel("model-router"),
		openai.WithEmbeddingModel("text-embedding-3-small"),
		openai.WithAPIVersion("2025-02-01-preview"))
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile("system-prompt")
	if err != nil {
		return nil, err
	}

	prompt := string(data)

	configJson, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	prompt = fmt.Sprintf("%s\n%s\nUSER:", prompt, string(configJson))

	data, err = os.ReadFile(promptPath)
	if err != nil {
		return nil, err
	}
	prompt = fmt.Sprintf("%s\n%s", prompt, string(data))

	log.Log.V(1).Info("User prompt", "prompt", string(data))

	response, err := llms.GenerateFromSinglePrompt(context.Background(), llm, prompt, llms.WithTemperature(0.5))
	if err != nil {
		return nil, err
	}

	log.Log.V(1).Info("LLM Response", "response", response)

	jsonResponse := make(map[string]string)
	err = json.Unmarshal([]byte(response), &jsonResponse)
	if err != nil {
		return nil, err
	}

	return jsonResponse, nil
}
