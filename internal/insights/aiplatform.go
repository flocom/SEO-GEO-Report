package insights

import (
	"net/url"
	"strings"
)

// AI platform names returned by DetectAIPlatform.
const (
	PlatformChatGPT    = "ChatGPT"
	PlatformPerplexity = "Perplexity"
	PlatformGemini     = "Gemini"
	PlatformCopilot    = "Copilot"
	PlatformClaude     = "Claude"
	PlatformMistral    = "Mistral Le Chat"
	PlatformDeepSeek   = "DeepSeek"
	PlatformYou        = "You.com"
	PlatformPhind      = "Phind"
	PlatformPoe        = "Poe"
	PlatformMetaAI     = "Meta AI"
	PlatformGrok       = "Grok"
	PlatformKagi       = "Kagi Assistant"
	PlatformHugging    = "HuggingChat"
	PlatformQwen       = "Qwen"
	PlatformKimi       = "Kimi"
	PlatformPi         = "Pi"
	PlatformDuckAI     = "Duck.ai"
	PlatformNotebookLM = "NotebookLM"
	PlatformFelo       = "Felo"
	PlatformIAsk       = "iAsk"
	PlatformAndi       = "Andi"
	PlatformCharacter  = "Character.AI"
)

// aiHosts maps a host (matched exactly or as a parent domain) optionally
// followed by a path prefix, to a platform. Order matters: more specific
// entries first.
var aiHosts = []struct {
	host, path, platform string
}{
	{"chatgpt.com", "", PlatformChatGPT},
	{"chat.openai.com", "", PlatformChatGPT},
	{"openai.com", "", PlatformChatGPT},
	{"perplexity.ai", "", PlatformPerplexity},
	{"perplexity.com", "", PlatformPerplexity},
	{"notebooklm.google.com", "", PlatformNotebookLM},
	{"notebooklm.google", "", PlatformNotebookLM},
	{"gemini.google.com", "", PlatformGemini},
	{"gemini.google", "", PlatformGemini},
	{"bard.google.com", "", PlatformGemini},
	{"aistudio.google.com", "", PlatformGemini},
	{"copilot.microsoft.com", "", PlatformCopilot},
	{"copilot.cloud.microsoft", "", PlatformCopilot},
	{"copilot.com", "", PlatformCopilot},
	{"edgeservices.bing.com", "", PlatformCopilot},
	{"bing.com", "/chat", PlatformCopilot},
	{"claude.ai", "", PlatformClaude},
	{"anthropic.com", "", PlatformClaude},
	{"chat.mistral.ai", "", PlatformMistral},
	{"mistral.ai", "", PlatformMistral},
	{"chat.deepseek.com", "", PlatformDeepSeek},
	{"deepseek.com", "", PlatformDeepSeek},
	{"you.com", "", PlatformYou},
	{"phind.com", "", PlatformPhind},
	{"poe.com", "", PlatformPoe},
	{"meta.ai", "", PlatformMetaAI},
	{"grok.com", "", PlatformGrok},
	{"x.ai", "", PlatformGrok},
	{"x.com", "/i/grok", PlatformGrok},
	{"kagi.com", "/assistant", PlatformKagi},
	{"huggingface.co", "/chat", PlatformHugging},
	{"hf.co", "/chat", PlatformHugging},
	{"chat.qwen.ai", "", PlatformQwen},
	{"tongyi.aliyun.com", "", PlatformQwen},
	{"kimi.moonshot.cn", "", PlatformKimi},
	{"kimi.com", "", PlatformKimi},
	{"kimi.ai", "", PlatformKimi},
	{"pi.ai", "", PlatformPi},
	{"duck.ai", "", PlatformDuckAI},
	{"duckduckgo.com", "/aichat", PlatformDuckAI},
	{"felo.ai", "", PlatformFelo},
	{"iask.ai", "", PlatformIAsk},
	{"andisearch.com", "", PlatformAndi},
	{"character.ai", "", PlatformCharacter},
}

// aiKeywords maps bare source names (utm_source values, manual labels) to
// platforms. A keyword matches the whole source or one of its words.
var aiKeywords = []struct {
	word, platform string
}{
	{"chatgpt", PlatformChatGPT},
	{"openai", PlatformChatGPT},
	{"searchgpt", PlatformChatGPT},
	{"perplexity", PlatformPerplexity},
	{"gemini", PlatformGemini},
	{"bard", PlatformGemini},
	{"copilot", PlatformCopilot},
	{"bingchat", PlatformCopilot},
	{"claude", PlatformClaude},
	{"anthropic", PlatformClaude},
	{"mistral", PlatformMistral},
	{"lechat", PlatformMistral},
	{"deepseek", PlatformDeepSeek},
	{"phind", PlatformPhind},
	{"grok", PlatformGrok},
	{"metaai", PlatformMetaAI},
	{"huggingchat", PlatformHugging},
	{"qwen", PlatformQwen},
	{"kimi", PlatformKimi},
	{"notebooklm", PlatformNotebookLM},
}

func detectAIPlatform(source string) string {
	s := strings.ToLower(strings.TrimSpace(source))
	if s == "" {
		return ""
	}
	// utm_source=chatgpt.com anywhere in the string (URL, query string...).
	if i := strings.Index(s, "utm_source="); i >= 0 {
		v := s[i+len("utm_source="):]
		if j := strings.IndexAny(v, "&# "); j >= 0 {
			v = v[:j]
		}
		if dec, err := url.QueryUnescape(v); err == nil {
			v = dec
		}
		return detectAIPlatform(v)
	}
	// "chatgpt.com / referral": drop the medium.
	if i := strings.Index(s, " / "); i >= 0 {
		s = s[:i]
	} else if i := strings.Index(s, " /"); i >= 0 {
		s = s[:i]
	} else if i := strings.Index(s, "/ "); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	for _, p := range []string{"https://", "http://", "//"} {
		s = strings.TrimPrefix(s, p)
	}
	host, path := s, ""
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		host, path = s[:i], s[i:]
	}
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "."), "www.")
	if strings.Contains(host, ".") {
		for _, h := range aiHosts {
			if (host == h.host || strings.HasSuffix(host, "."+h.host)) && strings.HasPrefix(path, h.path) {
				return h.platform
			}
		}
	}
	// Keyword match on the whole value or on its words ("kagi assistant",
	// "le chat mistral", "meta ai").
	compact := strings.NewReplacer(" ", "", "-", "", "_", "").Replace(s)
	switch compact {
	case "kagiassistant":
		return PlatformKagi
	case "metaai":
		return PlatformMetaAI
	case "youcom", "you":
		return PlatformYou
	case "poe":
		return PlatformPoe
	case "duckai":
		return PlatformDuckAI
	}
	words := strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	for _, k := range aiKeywords {
		if compact == k.word {
			return k.platform
		}
		for _, w := range words {
			if w == k.word {
				// A bare domain such as "gemini.com" is not Google Gemini:
				// keyword matching applies to labels, not to unknown hosts,
				// except for distinctive brand names.
				if strings.Contains(host, ".") && !distinctive(k.word) {
					continue
				}
				return k.platform
			}
		}
	}
	return ""
}

// distinctive reports whether a keyword is specific enough to identify an AI
// platform even inside an unknown host name.
func distinctive(w string) bool {
	switch w {
	case "chatgpt", "openai", "perplexity", "anthropic", "deepseek", "searchgpt", "huggingchat", "notebooklm":
		return true
	}
	return false
}
