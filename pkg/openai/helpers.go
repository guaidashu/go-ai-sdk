package openai

import (
	"encoding/json"
	"fmt"

	"github.com/guaidashu/go-ai-sdk/pkg/openai/pythontool"
)

// GetMaxRemainingTokens uses openai tiktoken to compute the number of tokens and thus you need to have python3 installed
// Watchout for functions definitions which count toward the model context length. I did not find information as to the function syntax
// used by openai to compute its number of tokens. You can probably approximate it.
func GetMaxRemainingTokens(prompt string, m Model) (int, error) {
	tokencount, err := CountTokens(prompt, m)
	if err != nil {
		return 0, err
	}

	switch m {
	default:
		return 0, fmt.Errorf("model %s not yet supported", m)
	case GPT3_5_turbo_4k, GPT3_5_turbo_4k_0301, GPT3_5_turbo_4k_0613:
		return int(Context4K) - tokencount, nil
	case GPT3_5_turbo_16k_0613, GPT3_5_turbo_16k:
		return int(Context16K) - tokencount, nil
	case GPT4_8k, GPT4_8k_0613:
		return int(Context8K) - tokencount, nil
	case GPT4_32k, GPT4_32k_0613:
		return int(Context32K) - tokencount, nil
	}
}

func CountTokens(prompt string, m Model) (int, error) {
	var encoding string
	switch m {
	default:
		return 0, fmt.Errorf("model %s not implemented yet for GetMaxRemainingTokens", m)
	case GPT3_5_turbo_4k, GPT4_8k, GPT4_32k, GPT3_5_turbo_4k_0301, GPT3_5_turbo_4k_0613, GPT3_5_turbo_16k_0613, GPT3_5_turbo_16k, GPT4_8k_0613, GPT4_32k_0613:
		encoding = "cl100k_base"
	}

	tokencount, err := pythontool.CountTokens(encoding, prompt)
	if err != nil {
		return 0, err
	}

	// TODO: Consider the following: `every reply is primed with <|start|>assistant<|message|>`
	tokencount += 50

	return tokencount, nil
}

func GetMaxRemainingTokensChatCompletion(req *ChatCompletionRequest) (int, error) {
	numTokens, err := CountTokensCompletion(req)
	if err != nil {
		return 0, err
	}

	switch req.Model {
	default:
		return 0, fmt.Errorf("model %s not yet supported", req.Model)
	case GPT3_5_turbo_4k, GPT3_5_turbo_4k_0301, GPT3_5_turbo_4k_0613:
		return int(Context4K) - numTokens, nil
	case GPT3_5_turbo_16k_0613, GPT3_5_turbo_16k:
		return int(Context16K) - numTokens, nil
	case GPT4_8k, GPT4_8k_0613:
		return int(Context8K) - numTokens, nil
	case GPT4_32k, GPT4_32k_0613:
		return int(Context32K) - numTokens, nil
	}
}

func CountTokensCompletion(req *ChatCompletionRequest) (int, error) {
	messages := req.Messages

	var tokenPerMessage int
	var encoding string

	switch req.Model {
	default:
		return 0, fmt.Errorf("model %s not implemented yet for GetMaxRemainingTokens", req.Model)
	case GPT3_5_turbo_4k, GPT3_5_turbo_4k_0301, GPT3_5_turbo_4k_0613, GPT3_5_turbo_16k_0613, GPT3_5_turbo_16k:
		encoding = "cl100k_base"

		// every message follows <im_start>{role/name}\n{content}<im_end>\n
		// See https://github.com/openai/openai-cookbook/blob/main/examples/How_to_count_tokens_with_tiktoken.ipynb
		tokenPerMessage = 4

	case GPT4_8k, GPT4_32k, GPT4_8k_0613, GPT4_32k_0613:
		encoding = "cl100k_base"
		tokenPerMessage = 3
	}

	var numTokens int
	for _, message := range messages {
		numTokens += tokenPerMessage
		switch t := message.Content.(type) {
		default:
			return 0, fmt.Errorf("our current implementation does not support a message content of type %T", t)
		case nil:
		case string:
			if t != "" {
				tokencount, err := pythontool.CountTokens(encoding, t)
				if err != nil {
					return 0, err
				}
				numTokens += tokencount
			}
		case []ContentPart:
			for _, cp := range t {
				switch cp.Type {
				default:
					return 0, fmt.Errorf("our current implementation does not support a message content part of type %s", cp.Type)
				case "text":
					if cp.Text != "" {
						tokencount, err := pythontool.CountTokens(encoding, cp.Text)
						if err != nil {
							return 0, err
						}
						numTokens += tokencount
					}
				case "image_url":
					// TODO
				}
			}
		}

		for _, tc := range message.ToolCalls {
			switch tc.Type {
			default:
				return 0, fmt.Errorf("our current implementation does not support a message content part of type %s", tc.Type)
			case "function":
				if tc.Function == nil {
					return 0, fmt.Errorf("we have a message with a toolcall of type function but without a defined function")
				}
				numTokens += 12
				b, err := json.Marshal(tc.Function)
				if err != nil {
					return 0, err
				}
				tokencount, err := pythontool.CountTokens(encoding, string(b))
				if err != nil {
					return 0, err
				}
				numTokens += tokencount
			}
		}
	}
	numTokens += 3 // every reply is primed with <|start|>assistant<|message|>

	for _, cf := range req.Tools {
		switch cf.Type {
		default:
			return 0, fmt.Errorf("our current implementation does not support, in the request, a toolcall of type %s", cf.Type)
		case "function":
			if cf.Function == nil {
				return 0, fmt.Errorf("we have a request with a toolcall of type function but without a defined function")
			}
			if cf.Function.Name != "" {
				tokencount, err := pythontool.CountTokens(encoding, cf.Function.Name)
				if err != nil {
					return 0, err
				}
				numTokens += tokencount
			}
			if cf.Function.Description != "" {
				tokencount, err := pythontool.CountTokens(encoding, cf.Function.Description)
				if err != nil {
					return 0, err
				}
				numTokens += tokencount
			}
			if cf.Function.Parameters != nil {
				numTokens += 11

				for propName, prop := range cf.Function.Parameters.Properties {
					tokencount, err := pythontool.CountTokens(encoding, propName)
					if err != nil {
						return 0, err
					}
					numTokens += tokencount

					if prop.Type != "" {
						numTokens += 2
						tokencount, err := pythontool.CountTokens(encoding, prop.Type)
						if err != nil {
							return 0, err
						}
						numTokens += tokencount
					}

					if prop.Type != "" {
						numTokens += 2
						tokencount, err := pythontool.CountTokens(encoding, prop.Type)
						if err != nil {
							return 0, err
						}
						numTokens += tokencount
					}

					if len(prop.Enum) > 0 {
						numTokens -= 3
						for _, en := range prop.Enum {
							numTokens += 3
							tokencount, err := pythontool.CountTokens(encoding, en)
							if err != nil {
								return 0, err
							}
							numTokens += tokencount
						}
					}
				}
			}
		}
	}

	// We do not seem to get it quite right in some scenario
	numTokens += 50

	return numTokens, nil
}
