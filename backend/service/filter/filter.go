package filter

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ActionAllow = "allow"
	ActionBlock = "block"

	maxContentRunes = 4000

	cacheKeyPrefix = "jev-cache-v1\x00"

	maxCachedModelLen = 200
	cacheGetTimeout   = 250 * time.Millisecond
	cachePutTimeout   = 500 * time.Millisecond

	profanityThreshold      = 0.65
	sexualThreshold         = 0.5
	discriminationThreshold = 0.5
	harassmentThreshold     = 0.65
	politicsThreshold       = 1.5
)

var (
	ErrUnavailable    = errors.New("filter API unavailable")
	ErrEmptyContent   = errors.New("content is empty")
	ErrContentTooLong = errors.New("content is too long")
)

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type CategoryResult struct {
	Label      string   `json:"label"`
	Type       string   `json:"type"`
	Value      float64  `json:"value"`
	Threshold  float64  `json:"threshold"`
	Triggered  bool     `json:"triggered"`
	Confidence *float64 `json:"confidence,omitempty"`
}

type Result struct {
	Action     string                    `json:"action"`
	Blocked    bool                      `json:"blocked"`
	Reasons    []string                  `json:"reasons"`
	Categories map[string]CategoryResult `json:"categories"`
	Model      string                    `json:"model"`
	Usage      Usage                     `json:"usage"`
	LatencyMS  int64                     `json:"latency_ms"`
}

type Cache interface {
	Get(ctx context.Context, key []byte) ([]byte, bool, error)
	Put(ctx context.Context, key, result []byte) error
}

type categorySpec struct {
	name      string
	label     string
	kind      string
	threshold float64
}

var categorySpecs = []categorySpec{
	{name: "profanity", label: "Excessive profanity", kind: "noul", threshold: profanityThreshold},
	{name: "sexual", label: "Sexual content", kind: "noul", threshold: sexualThreshold},
	{name: "discrimination", label: "Discrimination or hate", kind: "noul", threshold: discriminationThreshold},
	{name: "harassment", label: "Severe harassment", kind: "noul", threshold: harassmentThreshold},
	{name: "politics", label: "Politics", kind: "score", threshold: politicsThreshold},
}

type classifier struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
	cache    Cache
}

type requestState struct {
	Platform string `json:"platform"`
	Content  string `json:"content"`
}

type noulCriteria struct {
	True  string `json:"true"`
	False string `json:"false"`
}

type question struct {
	Type         string `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type systemOneRequest struct {
	Model     string              `json:"model"`
	State     requestState        `json:"state"`
	Questions map[string]question `json:"questions"`
}

type answer struct {
	Type       string   `json:"type"`
	Noul       *float64 `json:"noul"`
	Score      *float64 `json:"score"`
	Confidence *float64 `json:"confidence"`
}

type systemOneResponse struct {
	Model   string            `json:"model"`
	Answers map[string]answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}

func buildQuestions() map[string]question {
	return map[string]question{
		"profanity": {
			Type:         "noul",
			Instructions: "Does the content contain profanity that goes beyond casual swearing?",
			Criteria: noulCriteria{
				True:  `Severe, gratuitous or degrading profanity, including sexual insults and terms such as "whore" or "cunt", especially when used to demean or shock someone.`,
				False: `No profanity, or only casual profanity such as "damn", "hell" or "shit" used without targeting or degrading anyone.`,
			},
		},
		"sexual": {
			Type:         "noul",
			Instructions: "Does the content contain sexual content?",
			Criteria: noulCriteria{
				True:  "Any sexual content, including sexual acts, sexual solicitation, explicit or suggestive sexual descriptions, fetish content, and sexualized descriptions of bodies or minors.",
				False: "No sexual content. Innocent affection, non-sexual relationships, and neutral references to anatomy are not sexual.",
			},
		},
		"discrimination": {
			Type:         "noul",
			Instructions: "Does the content contain racism, homophobia, xenophobia, or any other discrimination or hate toward a group of people?",
			Criteria: noulCriteria{
				True:  "Racism, homophobia, xenophobia, or discrimination or hate based on race, ethnicity, nationality, religion, sex, gender identity, sexual orientation, disability, or similar characteristics, including slurs, stereotypes, exclusion, threats, or hateful claims.",
				False: "No discrimination or hate toward any group. Discussing these topics without attacking a group does not count.",
			},
		},
		"harassment": {
			Type:         "noul",
			Instructions: "Does the content contain severe harassment or a personal attack?",
			Criteria: noulCriteria{
				True:  "Severe harassment or a personal attack: sustained insults, humiliation, threats, degrading attacks, telling someone to harm or kill themselves, or targeted abuse.",
				False: `Mild criticism, disagreement, or competitive trash talk such as "you are so bad" or "you suck at this game", with no degradation, threat, or targeted abuse.`,
			},
		},
		"politics": {
			Type:         "score",
			Instructions: "How political is the content?",
			Criteria: []string{
				"None: no political content.",
				"Incidental: a brief or passing political reference that is not the main point.",
				"Substantive: political discussion, advocacy, campaigning, or partisan messaging is a meaningful part of the content.",
			},
		},
	}
}

func (c *classifier) classify(ctx context.Context, content string) (*Result, error) {
	started := time.Now()
	key := cacheKey(c.model, content)
	if cached := c.cachedResult(ctx, key); cached != nil {
		cached.LatencyMS = time.Since(started).Milliseconds()
		return cached, nil
	}

	response, err := c.ask(ctx, content)
	if err != nil {
		return nil, err
	}

	result, err := buildResult(response, started)
	if err != nil {
		return nil, err
	}
	c.storeResult(ctx, key, result)
	return result, nil
}

func buildResult(response *systemOneResponse, started time.Time) (*Result, error) {
	categories := make(map[string]CategoryResult, len(categorySpecs))
	reasons := make([]string, 0, len(categorySpecs))
	for _, spec := range categorySpecs {
		value, err := answerValue(response.Answers, spec)
		if err != nil {
			return nil, err
		}
		triggered := value >= spec.threshold
		category := CategoryResult{
			Label:     spec.label,
			Type:      spec.kind,
			Value:     value,
			Threshold: spec.threshold,
			Triggered: triggered,
		}
		if spec.kind == "score" {
			category.Confidence = response.Answers[spec.name].Confidence
		}
		categories[spec.name] = category
		if triggered {
			reasons = append(reasons, spec.label)
		}
	}

	blocked := len(reasons) > 0
	action := ActionAllow
	if blocked {
		action = ActionBlock
	}
	return &Result{
		Action:     action,
		Blocked:    blocked,
		Reasons:    reasons,
		Categories: categories,
		Model:      response.Model,
		Usage:      response.Usage,
		LatencyMS:  time.Since(started).Milliseconds(),
	}, nil
}

func answerValue(answers map[string]answer, spec categorySpec) (float64, error) {
	value, ok := answers[spec.name]
	if !ok {
		return 0, fmt.Errorf("missing %s answer", spec.name)
	}
	if spec.kind == "score" {
		if value.Score == nil {
			return 0, fmt.Errorf("missing %s answer", spec.name)
		}
		return *value.Score, nil
	}
	if value.Noul == nil {
		return 0, fmt.Errorf("missing %s answer", spec.name)
	}
	return *value.Noul, nil
}

func cacheKey(model, content string) [32]byte {
	hash := sha256.New()
	_, _ = hash.Write([]byte(cacheKeyPrefix))
	_, _ = hash.Write([]byte(model))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(content))
	var key [32]byte
	copy(key[:], hash.Sum(nil))
	return key
}

func (c *classifier) cachedResult(ctx context.Context, key [32]byte) *Result {
	if c.cache == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, cacheGetTimeout)
	defer cancel()
	blob, found, err := c.cache.Get(ctx, key[:])
	if err != nil || !found {
		return nil
	}
	return decodeCachedResult(blob)
}

func (c *classifier) storeResult(ctx context.Context, key [32]byte, result *Result) {
	if c.cache == nil {
		return
	}
	blob := encodeCachedResult(result)
	if blob == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cachePutTimeout)
	defer cancel()
	_ = c.cache.Put(ctx, key[:], blob)
}

func encodeCachedResult(result *Result) []byte {
	if result == nil || len(result.Model) == 0 || len(result.Model) > maxCachedModelLen {
		return nil
	}
	blob := make([]byte, 1+len(result.Model)+8*len(categorySpecs)+8)
	blob[0] = byte(len(result.Model))
	copy(blob[1:], result.Model)
	offset := 1 + len(result.Model)
	for _, spec := range categorySpecs {
		category, ok := result.Categories[spec.name]
		if !ok {
			return nil
		}
		binary.LittleEndian.PutUint64(blob[offset:], math.Float64bits(category.Value))
		offset += 8
	}
	confidence := math.NaN()
	if politics, ok := result.Categories["politics"]; ok && politics.Confidence != nil {
		confidence = *politics.Confidence
	}
	binary.LittleEndian.PutUint64(blob[offset:], math.Float64bits(confidence))
	return blob
}

func decodeCachedResult(blob []byte) *Result {
	if len(blob) < 2 {
		return nil
	}
	modelLen := int(blob[0])
	modelEnd := 1 + modelLen
	if modelLen == 0 || modelLen > maxCachedModelLen || len(blob) != modelEnd+8*len(categorySpecs)+8 {
		return nil
	}
	model := string(blob[1:modelEnd])
	offset := modelEnd
	categories := make(map[string]CategoryResult, len(categorySpecs))
	reasons := make([]string, 0, len(categorySpecs))
	for _, spec := range categorySpecs {
		value := math.Float64frombits(binary.LittleEndian.Uint64(blob[offset:]))
		offset += 8
		triggered := value >= spec.threshold
		categories[spec.name] = CategoryResult{
			Label:     spec.label,
			Type:      spec.kind,
			Value:     value,
			Threshold: spec.threshold,
			Triggered: triggered,
		}
		if triggered {
			reasons = append(reasons, spec.label)
		}
	}
	if politics, ok := categories["politics"]; ok {
		confidence := math.Float64frombits(binary.LittleEndian.Uint64(blob[offset:]))
		if !math.IsNaN(confidence) {
			politics.Confidence = &confidence
		}
		categories["politics"] = politics
	}

	blocked := len(reasons) > 0
	action := ActionAllow
	if blocked {
		action = ActionBlock
	}
	return &Result{
		Action:     action,
		Blocked:    blocked,
		Reasons:    reasons,
		Categories: categories,
		Model:      model,
	}
}

func (c *classifier) ask(ctx context.Context, content string) (*systemOneResponse, error) {
	payload := systemOneRequest{
		Model: c.model,
		State: requestState{
			Platform: "VERTEXIA, an online gaming social platform",
			Content:  content,
		},
		Questions: buildQuestions(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode TypeSafe request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build TypeSafe request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call TypeSafe: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("TypeSafe returned %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}

	var decoded systemOneResponse
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode TypeSafe response: %w", err)
	}
	if len(decoded.Answers) == 0 {
		return nil, errors.New("TypeSafe returned no answers")
	}
	return &decoded, nil
}

func validateContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", ErrEmptyContent
	}
	if utf8.RuneCountInString(content) > maxContentRunes {
		return "", ErrContentTooLong
	}
	return content, nil
}
