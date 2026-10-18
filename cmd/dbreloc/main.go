// dbreloc — utilitaire ponctuel : rend portable une base ajean.db copiée.
//
// Dans la version parallèle (BuildFlavor embedding-test), parallelHome() est
// <dossier de l'exe>\data, et les clés MOE_CONFIG / MOE_SOURCE / MOE_PYTHON /
// MEMORY_EMBED_* doivent y être relatives pour qu'une copie de l'installation
// fonctionne où qu'elle soit posée. Une base recopiée telle quelle garde les
// chemins absolus de l'installation d'origine : ce programme les convertit.
//
// Usage : dbreloc <chemin ajean.db> [--write]
// Sans --write : lecture seule, affiche le diagnostic.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	bolt "go.etcd.io/bbolt"
)

var relocKeys = []string{
	"MOE_CONFIG", "MOE_SOURCE", "MOE_PYTHON",
	"MEMORY_EMBED_BIN", "MEMORY_EMBED_MODEL",
	"BIN", "MODEL", "MMPROJ", "MODEL_DRAFT",
}

// racineData : pour un exe posé à la racine du dossier d'installation, la base
// vit dans <dossier>\data\ajean.db -> la racine des données est son dossier parent.
func racineData(dbFile string) string {
	return filepath.Dir(dbFile)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: dbreloc <chemin ajean.db> [--write]")
		os.Exit(2)
	}
	dbFile, err := filepath.Abs(os.Args[1])
	if err != nil {
		fmt.Println("ERREUR:", err)
		os.Exit(1)
	}
	write := len(os.Args) > 2 && os.Args[2] == "--write"
	root := racineData(dbFile)

	db, err := bolt.Open(dbFile, 0o600, nil)
	if err != nil {
		fmt.Println("ERREUR ouverture:", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()

	fmt.Println("base   :", dbFile)
	fmt.Println("racine :", root)

	var plan []string
	var changes []func(*bolt.Tx) error

	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		if b == nil {
			fmt.Println("AUCUN bucket config (base neuve ?)")
			return nil
		}
		keys := make([]string, 0, 64)
		_ = b.ForEach(func(k, v []byte) error {
			keys = append(keys, string(k))
			return nil
		})
		sort.Strings(keys)
		fmt.Println("--- config ---")
		for _, k := range keys {
			fmt.Printf("  %-22s %s\n", k, string(b.Get([]byte(k))))
		}
		for _, k := range keys {
			v := string(b.Get([]byte(k)))
			for _, want := range relocKeys {
				if k != want || v == "" || !filepath.IsAbs(v) {
					continue
				}
				rel, rerr := filepath.Rel(root, v)
				if rerr != nil || strings.HasPrefix(rel, "..") {
					fmt.Printf("  ! %s : hors racine, laissé tel quel (%v)\n", k, rerr)
					continue
				}
				if _, serr := os.Stat(v); serr != nil {
					fmt.Printf("  ! %s : cible absente %s\n", k, v)
				}
				plan = append(plan, fmt.Sprintf("%s = %s", k, rel))
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println("ERREUR lecture:", err)
		os.Exit(1)
	}

	if len(plan) == 0 {
		fmt.Println("RIEN À FAIRE : base déjà portable.")
		return
	}
	fmt.Println("--- plan ---")
	for _, p := range plan {
		fmt.Println("  ", p)
	}
	if !write {
		fmt.Println("(lecture seule : relancer avec --write)")
		return
	}

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		if b == nil {
			return fmt.Errorf("bucket config absent")
		}
		for _, p := range plan {
			k, v, _ := strings.Cut(p, " = ")
			if err := b.Put([]byte(k), []byte(v)); err != nil {
				return err
			}
			_ = changes
		}
		return nil
	})
	if err != nil {
		fmt.Println("ERREUR écriture:", err)
		os.Exit(1)
	}
	fmt.Println("ÉCRIT:", len(plan), "clés rendues relatives.")

	_ = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		if b == nil {
			return nil
		}
		fmt.Println("--- config après ---")
		for _, k := range relocKeys {
			if v := b.Get([]byte(k)); v != nil {
				fmt.Printf("  %-22s %s\n", k, string(v))
			}
		}
		return nil
	})
}
