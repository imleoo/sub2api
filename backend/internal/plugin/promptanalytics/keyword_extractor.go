package promptanalytics

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode"
)

// stopWords is a set of common words to exclude from keyword extraction.
var stopWords = map[string]bool{
	// English common words
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "is": true, "are": true, "was": true, "were": true, "be": true,
	"been": true, "being": true, "have": true, "has": true, "had": true, "do": true,
	"does": true, "did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "can": true, "that": true, "this": true, "these": true,
	"those": true, "it": true, "its": true, "as": true, "by": true, "from": true,
	"so": true, "if": true, "not": true, "what": true, "how": true, "when": true,
	"where": true, "why": true, "who": true, "which": true, "we": true, "you": true,
	"i": true, "he": true, "she": true, "they": true, "my": true, "your": true,
	"our": true, "their": true, "me": true, "him": true, "her": true, "us": true,
	"them": true, "all": true, "any": true, "each": true, "every": true, "no": true,
	"about": true, "into": true, "than": true, "then": true, "also": true,
	// Chinese common particles / function words
	"的": true, "了": true, "是": true, "在": true, "我": true, "有": true, "和": true,
	"就": true, "不": true, "人": true, "都": true, "一": true, "个": true, "上": true,
	"也": true, "这": true, "到": true, "他": true, "以": true, "说": true, "时": true,
	"来": true, "用": true, "把": true, "对": true, "很": true, "可": true, "但": true,
	"还": true, "就是": true, "没有": true, "因为": true, "可以": true, "这个": true,
}

// tokenRe splits text into word tokens for English / alphanumeric sequences.
var tokenRe = regexp.MustCompile(`[a-zA-Z0-9\x{4e00}-\x{9fff}\x{3400}-\x{4dbf}]+`)

// KeywordExtractor extracts significant keywords from prompt text.
type KeywordExtractor struct {
	maxKeywords int
}

// NewKeywordExtractor creates a new extractor with the given maximum keyword limit.
func NewKeywordExtractor(maxKeywords int) *KeywordExtractor {
	return &KeywordExtractor{maxKeywords: maxKeywords}
}

// ExtractKeywords parses the raw OpenAI-compatible request body and returns
// a deduplicated list of keywords. It never stores the original text.
func (e *KeywordExtractor) ExtractKeywords(body []byte) []string {
	text := extractTextFromBody(body)
	if text == "" {
		return nil
	}
	return e.tokenize(text)
}

// extractTextFromBody pulls "content" strings out of an OpenAI-compatible
// messages array. Handles both string and array-of-content-block formats.
func extractTextFromBody(body []byte) string {
	var req struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
		// Anthropic format
		System string `json:"system"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}

	var sb strings.Builder
	// Only extract from user messages to focus on actual prompts.
	for _, msg := range req.Messages {
		if msg.Role != "user" {
			continue
		}
		if len(msg.Content) == 0 {
			continue
		}
		// Try string first.
		var str string
		if json.Unmarshal(msg.Content, &str) == nil {
			_, _ = sb.WriteString(str)
			_ = sb.WriteByte(' ')
			continue
		}
		// Try array of content blocks.
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(msg.Content, &blocks) == nil {
			for _, b := range blocks {
				if b.Type == "text" {
					_, _ = sb.WriteString(b.Text)
					_ = sb.WriteByte(' ')
				}
			}
		}
	}
	return sb.String()
}

// tokenize splits text into cleaned, filtered tokens.
func (e *KeywordExtractor) tokenize(text string) []string {
	tokens := tokenRe.FindAllString(strings.ToLower(text), -1)

	seen := make(map[string]bool, len(tokens))
	result := make([]string, 0, e.maxKeywords)

	for _, tok := range tokens {
		if len(result) >= e.maxKeywords {
			break
		}
		// Filter: length 2-40, not a stop word, not purely numeric.
		runeCount := len([]rune(tok))
		if runeCount < 2 || runeCount > 40 {
			continue
		}
		if stopWords[tok] {
			continue
		}
		if isAllDigits(tok) {
			continue
		}
		// For CJK text, use non-overlapping 2-char terms to avoid cross-boundary noise
		// like “经网/器学”, and keep short full terms when useful.
		if isCJK([]rune(tok)[0]) {
			terms := extractCJKTerms(tok)
			for _, term := range terms {
				if len(result) >= e.maxKeywords {
					break
				}
				if !seen[term] && !stopWords[term] {
					seen[term] = true
					result = append(result, term)
				}
			}
		} else {
			if !seen[tok] {
				seen[tok] = true
				result = append(result, tok)
			}
		}
	}
	return result
}

func isAllDigits(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isCJK(r rune) bool {
	return (r >= 0x4e00 && r <= 0x9fff) || (r >= 0x3400 && r <= 0x4dbf)
}

// extractCJKTerms extracts CJK terms using a conservative strategy:
//   - for short phrases (2~4 chars), keep the whole phrase;
//   - split long phrases into non-overlapping 2-char terms.
//
// This avoids overlapping cross-boundary fragments such as “经网/器学”.
func extractCJKTerms(s string) []string {
	runes := []rune(s)
	n := len(runes)
	if n < 2 {
		return nil
	}

	terms := make([]string, 0, n/2+1)

	if n >= 2 && n <= 4 {
		terms = append(terms, s)
	}

	for i := 0; i+1 < n; i += 2 {
		terms = append(terms, string(runes[i:i+2]))
	}

	return terms
}
