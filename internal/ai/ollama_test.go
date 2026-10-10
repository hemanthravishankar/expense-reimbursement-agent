package ai

import "testing"

func TestResolveOllamaURLUsesDefault(t *testing.T) {
	t.Setenv("OLLAMA_URL", "")
	t.Setenv("OLLAMA_BASE_URL", "")

	if got := resolveOllamaURL(); got != OllamaURL {
		t.Fatalf("resolveOllamaURL() = %q, want %q", got, OllamaURL)
	}
}

func TestResolveOllamaURLUsesBaseURL(t *testing.T) {
	t.Setenv("OLLAMA_URL", "")
	t.Setenv("OLLAMA_BASE_URL", "http://host.docker.internal:11434/")

	want := "http://host.docker.internal:11434/api/chat"

	if got := resolveOllamaURL(); got != want {
		t.Fatalf("resolveOllamaURL() = %q, want %q", got, want)
	}
}

func TestResolveOllamaURLAcceptsFullChatURL(t *testing.T) {
	t.Setenv("OLLAMA_URL", "")
	t.Setenv(
		"OLLAMA_BASE_URL",
		"http://ollama.example:11434/api/chat",
	)

	want := "http://ollama.example:11434/api/chat"

	if got := resolveOllamaURL(); got != want {
		t.Fatalf("resolveOllamaURL() = %q, want %q", got, want)
	}
}

func TestResolveOllamaURLFullEndpointOverridesBase(t *testing.T) {
	t.Setenv(
		"OLLAMA_URL",
		"http://custom-ollama:11434/api/chat",
	)
	t.Setenv(
		"OLLAMA_BASE_URL",
		"http://ignored-ollama:11434",
	)

	want := "http://custom-ollama:11434/api/chat"

	if got := resolveOllamaURL(); got != want {
		t.Fatalf("resolveOllamaURL() = %q, want %q", got, want)
	}
}
