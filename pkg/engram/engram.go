package engram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bubustack/bobrapet/pkg/storage"
	sdkengram "github.com/bubustack/bubu-sdk-go/engram"
	"github.com/bubustack/core/templating"
	"github.com/bubustack/materialize-engram/pkg/config"
)

const (
	modeCondition = "condition"
	modeObject    = "object"
)

type Engram struct {
	evaluator *templating.Evaluator
}

func New() *Engram {
	return &Engram{}
}

func (e *Engram) Init(ctx context.Context, _ config.Config, _ *sdkengram.Secrets) error {
	eval, err := templating.New(templating.Config{
		EvaluationTimeout: 30 * time.Second,
		MaxOutputBytes:    0, // no limit — runs in-pod after S3 hydration
		Deterministic:     false,
	})
	if err != nil {
		return err
	}
	e.evaluator = eval
	return nil
}

func (e *Engram) Process(
	ctx context.Context,
	_ *sdkengram.ExecutionContext,
	inputs config.Inputs,
) (*sdkengram.Result, error) {
	result, err := e.evaluate(ctx, inputs)
	if err != nil {
		return nil, err
	}
	return sdkengram.NewResultFrom(map[string]any{"result": result}), nil
}

func (e *Engram) Stream(
	ctx context.Context,
	in <-chan sdkengram.InboundMessage,
	out chan<- sdkengram.StreamMessage,
) error {
	logger := slog.Default()
	for msg := range in {
		raw := bytes.TrimSpace(streamInputBytes(msg))
		if len(raw) == 0 {
			msg.Done()
			continue
		}
		if isControlFrame(msg) {
			msg.Done()
			continue
		}
		var req config.Inputs
		if err := json.Unmarshal(raw, &req); err != nil {
			msg.Done()
			return fmt.Errorf("failed to decode materialize inputs: %w", err)
		}
		result, err := e.evaluate(ctx, req)
		if err != nil {
			msg.Done()
			return err
		}
		encoded, err := json.Marshal(map[string]any{"result": result})
		if err != nil {
			msg.Done()
			return fmt.Errorf("failed to encode materialize result: %w", err)
		}
		response := newStreamResultMessage(msg, encoded)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- response:
			msg.Done()
		}
	}
	logger.Info("materialize stream completed")
	return nil
}

func (e *Engram) evaluate(ctx context.Context, inputs config.Inputs) (any, error) {
	mode := strings.TrimSpace(inputs.Mode)
	if mode == "" {
		mode = modeObject
	}
	vars := inputs.Vars
	if vars == nil {
		vars = map[string]any{}
	}
	if requiresHydration(vars) {
		manager, err := storage.SharedManager(ctx)
		if err != nil {
			return nil, fmt.Errorf("storage manager unavailable: %w", err)
		}
		if manager == nil {
			return nil, fmt.Errorf("storage manager unavailable")
		}
		hydrated, err := manager.Hydrate(ctx, vars)
		if err != nil {
			return nil, fmt.Errorf("failed to hydrate vars: %w", err)
		}
		if hydrated != nil {
			m, ok := hydrated.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("hydrated vars must be object, got %T", hydrated)
			}
			vars = m
		}
	}
	eval := e.evaluator
	if eval == nil {
		var err error
		eval, err = templating.New(templating.Config{
			EvaluationTimeout: 30 * time.Second,
			MaxOutputBytes:    0, // no limit — runs in-pod after S3 hydration
			Deterministic:     false,
		})
		if err != nil {
			return nil, err
		}
	}

	switch mode {
	case modeCondition:
		expr, ok := inputs.Template.(string)
		if !ok {
			return nil, fmt.Errorf("template must be string for condition mode")
		}
		return eval.EvaluateCondition(ctx, expr, vars)
	case modeObject:
		return eval.ResolveValue(ctx, inputs.Template, vars)
	default:
		return nil, fmt.Errorf("unsupported materialize mode %q", mode)
	}
}

func streamInputBytes(msg sdkengram.InboundMessage) []byte {
	if len(msg.Inputs) > 0 {
		return msg.Inputs
	}
	if len(msg.Payload) > 0 {
		return msg.Payload
	}
	if msg.Binary != nil && len(msg.Binary.Payload) > 0 {
		return msg.Binary.Payload
	}
	return nil
}

func newStreamResultMessage(msg sdkengram.InboundMessage, encoded []byte) sdkengram.StreamMessage {
	body := append([]byte(nil), encoded...)
	return sdkengram.StreamMessage{
		Metadata: cloneMetadata(msg.Metadata),
		Inputs:   append([]byte(nil), body...),
		Payload:  body,
		Binary: &sdkengram.BinaryFrame{
			Payload:  append([]byte(nil), body...),
			MimeType: "application/json",
		},
	}
}

func cloneMetadata(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if strings.TrimSpace(k) == "" {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func requiresHydration(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		if hasHydrationRefKeys(typed) {
			return true
		}
		for _, nested := range typed {
			if requiresHydration(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if requiresHydration(nested) {
				return true
			}
		}
	}
	return false
}

func hasHydrationRefKeys(values map[string]any) bool {
	for key := range values {
		switch key {
		case storage.StorageRefKey, "$bubuConfigMapRef", "$bubuSecretRef":
			return true
		}
	}
	return false
}

func isControlFrame(msg sdkengram.InboundMessage) bool {
	if isControlMarker(msg.Kind) {
		return true
	}
	for _, key := range []string{"type", "event", "kind"} {
		if isControlMarker(msg.Metadata[key]) {
			return true
		}
	}
	return false
}

func isControlMarker(value string) bool {
	marker := strings.ToLower(strings.TrimSpace(value))
	if marker == "" {
		return false
	}
	return strings.Contains(marker, "control") ||
		strings.Contains(marker, "heartbeat") ||
		strings.Contains(marker, "keepalive") ||
		marker == "ping" ||
		marker == "pong" ||
		marker == "ack"
}
