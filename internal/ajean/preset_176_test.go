package ajean

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// #176 : modifier le preset actif doit être signalé pour être appliqué.
func TestSavePresetActifDemandeRechargement(t *testing.T) {
	home := testHome(t)
	_ = home
	id, err := SavePreset("", "Test", "MODEL=a.gguf\nCTX=4096\n")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := safePresetPath(id)
	if err := applyPresetFile(p); err != nil {
		t.Fatal(err)
	}
	_ = putStr(bkState, "active_preset", id)
	save := func(content string) map[string]any {
		body, _ := json.Marshal(map[string]any{"id": id, "name": "Test", "content": content})
		w := httptest.NewRecorder()
		handlePresetSave(w, httptest.NewRequest("POST", "/api/preset/save", bytes.NewReader(body)))
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return out
	}
	if out := save("MODEL=a.gguf\nCTX=8192\n"); out["reload"] != true {
		t.Fatalf("preset actif modifié : reload = %v, attendu true", out["reload"])
	}
	if b, _ := os.ReadFile(filepath.Clean(p)); !bytes.Contains(b, []byte("CTX=8192")) {
		t.Fatal("preset non enregistré")
	}
}
