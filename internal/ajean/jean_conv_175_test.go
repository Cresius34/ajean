package ajean

import (
	"encoding/json"
	"strings"
	"testing"

	bolt "go.etcd.io/bbolt"
)

// #175 : la RAM a perdu le fil Jean (lecture ratée au démarrage) alors que le
// fil vivant enregistré est à jour. Ouvrir Jean ne doit pas le remplacer par
// l'archive, plus ancienne.
func TestOpenJeanGardeLeFilVivant(t *testing.T) {
	testHome(t)
	ensureDefaultProject()
	conv.mu.Lock()
	conv.ID, conv.Mode, conv.Messages, conv.Log = "", "", nil, nil
	conv.mu.Unlock()
	conv.JeanPost("vieux message") // fil Jean pas affiché : il part dans l'archive
	if _, ok := loadArchive(jeanConvID); !ok {
		t.Fatal("archive Jean absente")
	}
	live := map[string]any{"id": jeanConvID, "mode": "jean", "seq": 999,
		"messages": []map[string]any{{"role": "user", "content": "a"}, {"role": "assistant", "content": "b"}, {"role": "user", "content": "récent"}},
		"log":      []any{}}
	b, _ := json.Marshal(live)
	if err := putStoreBytes(bkChat, "conversation", b); err != nil {
		t.Fatal(err)
	}
	// Lecture ratée au démarrage : RAM vide, id neuf.
	conv.mu.Lock()
	conv.ID, conv.Mode, conv.Messages, conv.Log, conv.Seq = "123", "", nil, nil, 0
	conv.mu.Unlock()
	convLoadSuspect.Store(true)

	if err := conv.OpenJean(); err != nil {
		t.Fatal(err)
	}
	conv.mu.Lock()
	id, n, seq := conv.ID, len(conv.Messages), conv.Seq
	conv.mu.Unlock()
	if id != jeanConvID || n != 3 || seq != 999 {
		t.Fatalf("fil Jean = id %q, %d messages, seq %d ; attendu le fil vivant (3, 999)", id, n, seq)
	}
	disk := liveConvOnDisk()
	if disk == nil || len(disk.Messages) != 3 {
		t.Fatal("le fil vivant enregistré a été écrasé")
	}
}

// Une conversation enregistrée illisible est mise de côté avant d'être écrasée.
func TestConversationIllisibleCopieDeSecours(t *testing.T) {
	testHome(t)
	ensureDefaultProject()
	if err := putBytes(bkChat, "conversation", []byte(`{"id":"x","messages":[{`)); err != nil {
		t.Fatal(err)
	}
	convLoadSuspect.Store(false)
	LoadConversation()
	if !convLoadSuspect.Load() {
		t.Fatal("lecture ratée non signalée")
	}
	conv.persist()
	found := false
	_ = view(bkChat, func(bk *bolt.Bucket) error {
		return bk.ForEach(func(k, v []byte) error {
			if strings.HasPrefix(string(k), "conversation.secours.") && strings.Contains(string(v), `"id":"x"`) {
				found = true
			}
			return nil
		})
	})
	if !found {
		t.Fatal("aucune copie de secours avant réécriture")
	}
}
