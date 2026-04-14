package engram

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bubustack/bobrapet/pkg/storage"
	sdkengram "github.com/bubustack/bubu-sdk-go/engram"
	"github.com/bubustack/core/contracts"
	"github.com/bubustack/materialize-engram/pkg/config"
)

func TestStreamAcceptsPayloadInputAndEmitsResolvedPayload(t *testing.T) {
	engine := New()
	if err := engine.Init(context.Background(), config.Config{}, nil); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	in := make(chan sdkengram.InboundMessage, 1)
	out := make(chan sdkengram.StreamMessage, 1)
	in <- sdkengram.NewInboundMessage(sdkengram.StreamMessage{
		Metadata: map[string]string{"source": "test"},
		Payload:  []byte(`{"mode":"object","template":{"value":"payload-win"},"vars":{}}`),
		Binary: &sdkengram.BinaryFrame{
			Payload:  []byte(`{"mode":"object","template":{"value":"binary-lose"},"vars":{}}`),
			MimeType: "application/json",
		},
	})
	close(in)

	if err := engine.Stream(context.Background(), in, out); err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}

	select {
	case msg := <-out:
		var decoded map[string]any
		if err := json.Unmarshal(msg.Payload, &decoded); err != nil {
			t.Fatalf("failed to decode output payload: %v", err)
		}
		result, ok := decoded["result"].(map[string]any)
		if !ok {
			t.Fatalf("expected result object, got %#v", decoded["result"])
		}
		if result["value"] != "payload-win" {
			t.Fatalf("expected payload input to win, got %#v", result["value"])
		}
		if msg.Binary == nil {
			t.Fatal("expected binary mirror for structured output")
		}
	default:
		t.Fatal("expected a materialize stream output")
	}
}

func TestProcessInlineVarsDoesNotDependOnStorageManager(t *testing.T) {
	t.Setenv(contracts.StorageProviderEnv, "invalid-provider")
	storage.ResetSharedManagerCacheForTests()
	defer storage.ResetSharedManagerCacheForTests()

	engine := New()
	if err := engine.Init(context.Background(), config.Config{}, nil); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	res, err := engine.Process(context.Background(), nil, config.Inputs{
		Mode:     "condition",
		Template: "{{ eq .name \"bob\" }}",
		Vars: map[string]any{
			"name": "bob",
		},
	})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	data, ok := res.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map output, got %T", res.Data)
	}
	if data["result"] != true {
		t.Fatalf("expected condition result true, got %#v", data["result"])
	}
}

func TestProcessFailsWhenHydrationRequiredButStorageManagerUnavailable(t *testing.T) {
	t.Setenv(contracts.StorageProviderEnv, "invalid-provider")
	storage.ResetSharedManagerCacheForTests()
	defer storage.ResetSharedManagerCacheForTests()

	engine := New()
	if err := engine.Init(context.Background(), config.Config{}, nil); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	_, err := engine.Process(context.Background(), nil, config.Inputs{
		Mode:     "object",
		Template: "{{ .payload }}",
		Vars: map[string]any{
			"payload": map[string]any{
				storage.StorageRefKey: "inputs/missing.json",
			},
		},
	})
	if err == nil {
		t.Fatal("expected storage manager error, got nil")
	}
	if !strings.Contains(err.Error(), "storage manager unavailable") {
		t.Fatalf("expected storage manager unavailable error, got: %v", err)
	}
}

func TestStreamSkipsControlAndEmptyFrames(t *testing.T) {
	engine := New()
	if err := engine.Init(context.Background(), config.Config{}, nil); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	in := make(chan sdkengram.InboundMessage, 2)
	out := make(chan sdkengram.StreamMessage, 2)
	in <- sdkengram.NewInboundMessage(sdkengram.StreamMessage{
		Kind: "heartbeat",
	})
	in <- sdkengram.NewInboundMessage(sdkengram.StreamMessage{
		Payload: []byte(`{"mode":"object","template":{"value":"ok"},"vars":{}}`),
	})
	close(in)

	if err := engine.Stream(context.Background(), in, out); err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}

	if len(out) != 1 {
		t.Fatalf("expected exactly one output frame, got %d", len(out))
	}
}
