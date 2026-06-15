package json

import (
	"bytes"
	encjson "encoding/json"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/PlakarKorp/kloset/events"
	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/google/uuid"
)

func TestSanitizeDataPreservesPrimitives(t *testing.T) {
	in := map[string]any{
		"path":  "/var",
		"count": 42,
		"flag":  true,
	}
	got := sanitizeData(in)
	if got["path"] != "/var" || got["count"] != 42 || got["flag"] != true {
		t.Fatalf("primitives mutated: %+v", got)
	}
}

func TestSanitizeDataConvertsErrorToString(t *testing.T) {
	got := sanitizeData(map[string]any{"err": errors.New("boom")})
	if got["err"] != "boom" {
		t.Fatalf("err = %v, want %q", got["err"], "boom")
	}
}

func TestSanitizeDataConvertsDurationToMillis(t *testing.T) {
	got := sanitizeData(map[string]any{"d": 250 * time.Millisecond})
	if got["d"] != int64(250) {
		t.Fatalf("d = %v (%T), want int64(250)", got["d"], got["d"])
	}
}

func TestSanitizeDataReturnsNewMap(t *testing.T) {
	in := map[string]any{"k": "v"}
	got := sanitizeData(in)
	got["k"] = "changed"
	if in["k"] != "v" {
		t.Fatalf("sanitizeData must not mutate input map; input is now %+v", in)
	}
}

func TestNewReturnsNonNilUI(t *testing.T) {
	ctx := appcontext.NewAppContext()
	u := New(ctx)
	if u == nil {
		t.Fatal("New returned nil")
	}
}

func TestLifecycleAccessors(t *testing.T) {
	ctx := appcontext.NewAppContext()
	u := New(ctx)
	if u.Stdout() != io.Discard {
		t.Fatalf("Stdout = %v, want io.Discard", u.Stdout())
	}
	if u.Stderr() != os.Stderr {
		t.Fatalf("Stderr = %v, want os.Stderr", u.Stderr())
	}
	u.SetRepository(nil) // doesn't panic
	u.Stop()             // no-op, doesn't panic
}

func TestRunDrainsEventsAndExitsOnBusClose(t *testing.T) {
	ctx := appcontext.NewAppContext()
	u := New(ctx)

	if err := u.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Publish an event so the goroutine's loop body runs and dispatches through
	// handleEvent before the bus is closed.
	emitter := ctx.Events().NewRepositoryEmitter(uuid.Nil, "test")
	emitter.PathOk("/x")

	// Closing the events bus closes the channel returned by Listen(), which
	// terminates the renderer goroutine.
	ctx.Events().Close()

	done := make(chan error, 1)
	go func() { done <- u.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return after bus close")
	}
}

// failWriter always fails, so the encoder's Encode returns an error.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestHandleEvent_EncodeErrorIsSwallowed(t *testing.T) {
	ctx := appcontext.NewAppContext()
	r := &jsonRenderer{
		ctx:     ctx,
		encoder: encjson.NewEncoder(failWriter{}),
	}

	// handleEvent must return cleanly even when the underlying writer errors.
	r.handleEvent(&events.Event{Level: "info", Type: "path.ok", Data: map[string]any{"path": "/x"}})
}

func TestJSONEventEncodes(t *testing.T) {
	// Smoke test: jsonEvent struct should encode to JSON containing the fields
	// we expect.
	ev := jsonEvent{
		Version:   1,
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Level:     "info",
		Workflow:  "backup",
		Type:      "path",
		Data:      map[string]any{"k": "v"},
	}
	var buf bytes.Buffer
	if err := encjson.NewEncoder(&buf).Encode(ev); err != nil {
		t.Fatalf("encode: %v", err)
	}
	var back map[string]any
	if err := encjson.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if back["level"] != "info" || back["workflow"] != "backup" || back["type"] != "path" {
		t.Fatalf("unexpected encoding: %+v", back)
	}
}

// newRendererTo returns a renderer whose encoder writes to buf, so tests can
// inspect what handleEvent emits without racing on os.Stdout.
func newRendererTo(ctx *appcontext.AppContext, buf *bytes.Buffer) *jsonRenderer {
	return &jsonRenderer{
		ctx:     ctx,
		encoder: encjson.NewEncoder(buf),
	}
}

func TestHandleEvent_EncodesInfo(t *testing.T) {
	ctx := appcontext.NewAppContext()
	var buf bytes.Buffer
	r := newRendererTo(ctx, &buf)

	r.handleEvent(&events.Event{
		Version:   1,
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Level:     "info",
		Workflow:  "backup",
		Type:      "path.ok",
		Data:      map[string]any{"path": "/etc/hosts"},
	})

	var out map[string]any
	if err := encjson.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v (raw=%q)", err, buf.String())
	}
	if out["level"] != "info" || out["type"] != "path.ok" {
		t.Fatalf("unexpected output: %+v", out)
	}
	data, _ := out["data"].(map[string]any)
	if data == nil || data["path"] != "/etc/hosts" {
		t.Fatalf("data not encoded: %+v", out)
	}
}

func TestHandleEvent_SilentSuppresses(t *testing.T) {
	ctx := appcontext.NewAppContext()
	ctx.Silent = true
	var buf bytes.Buffer
	r := newRendererTo(ctx, &buf)

	r.handleEvent(&events.Event{Level: "info", Type: "path"})

	if buf.Len() != 0 {
		t.Fatalf("Silent should suppress output, got %q", buf.String())
	}
}

func TestHandleEvent_QuietSuppressesInfoOnly(t *testing.T) {
	ctx := appcontext.NewAppContext()
	ctx.Quiet = true
	var buf bytes.Buffer
	r := newRendererTo(ctx, &buf)

	// info level: suppressed
	r.handleEvent(&events.Event{Level: "info", Type: "path"})
	if buf.Len() != 0 {
		t.Fatalf("Quiet should suppress info, got %q", buf.String())
	}

	// non-info level (e.g. error): still emitted
	r.handleEvent(&events.Event{Level: "error", Type: "path.error"})
	if buf.Len() == 0 {
		t.Fatal("Quiet should not suppress non-info events")
	}
}

func TestHandleEvent_NoDataOmitsField(t *testing.T) {
	ctx := appcontext.NewAppContext()
	var buf bytes.Buffer
	r := newRendererTo(ctx, &buf)

	r.handleEvent(&events.Event{Level: "info", Type: "workflow.start"})

	var out map[string]any
	if err := encjson.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := out["data"]; ok {
		t.Fatalf("expected no data field, got %+v", out)
	}
}
