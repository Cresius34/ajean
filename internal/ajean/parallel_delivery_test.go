package ajean

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// Exercise the served embedded page, rather than the template on disk. A stale
// generated UI or a wrong entry point must not pass a portable delivery check.
func TestParallelDeliveryServesMemoryControlsAndRevision(t *testing.T) {
	parallelTestHome(t)
	w := httptest.NewRecorder()
	handleIndex(w, httptest.NewRequest("GET", "http://127.0.0.1/", nil))
	if w.Code != 200 || !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("page non servie sans cache : %d %v", w.Code, w.Header())
	}
	body := w.Body.String()
	for _, marker := range []string{displayVersion(), "TEST mémoire · R5", `id="memory-system"`, `id="m-memory-block"`, `id="m-memory-device"`, `id="m-memory-image"`, `id="m-memory-audio"`, `id="m-memory-video"`, "function setMemorySystem", "function populateMemoryPreset"} {
		if !strings.Contains(body, marker) {
			t.Errorf("interface embarquée incomplète : %s absent", marker)
		}
	}
	vision := strings.Index(body, `id="s-mmproj-cpu-row"`)
	memory := strings.Index(body, `id="m-memory-block"`)
	if vision < 0 || memory <= vision {
		t.Error("réglages mémoire absents sous Vision")
	}
	BuildFlavor = ""
	w = httptest.NewRecorder()
	handleIndex(w, httptest.NewRequest("GET", "http://127.0.0.1/", nil))
	if strings.Contains(w.Body.String(), "TEST mémoire · R5") {
		t.Error("badge TEST affiché dans le point d'entrée normal")
	}
}

func TestParallelDeliveryStatusIdentifiesRunningExecutable(t *testing.T) {
	parallelTestHome(t)
	w := httptest.NewRecorder()
	handleEmbeddingSettings(w, httptest.NewRequest("GET", "http://127.0.0.1/api/embeddings", nil))
	var status struct {
		Version    string `json:"version"`
		Executable string `json:"executable"`
		UIRevision int    `json:"memory_ui_revision"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Version != displayVersion() || status.Executable == "" || status.UIRevision != parallelMemoryRevision {
		t.Fatalf("instance non identifiable : %+v", status)
	}
	if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
		t.Error("statut de version susceptible d'être mis en cache")
	}
}
