package ajean

import (
	"context"
	"fmt"
	"os"
)

// cmdJeanSleepTest : banc d'essai du sommeil, sur une COPIE de la mémoire
// (lancer avec AJEAN_HOME pointant sur la copie). Pas de planification, pas de
// bascule de preset : le moteur déjà chargé sert.
//
//	ajean jean-sleep-test v1|v2          fait dormir Jean et affiche le bilan
//	ajean jean-sleep-test v1|v2 --dump   affiche seulement ce que le modèle lirait
func cmdJeanSleepTest(args []string) error {
	if len(args) == 0 || (args[0] != "v1" && args[0] != "v2") {
		return fmt.Errorf("usage : ajean jean-sleep-test v1|v2 [--dump]")
	}
	if os.Getenv("AJEAN_HOME") == "" {
		return fmt.Errorf("AJEAN_HOME doit pointer sur une copie, jamais sur la vraie mémoire")
	}
	jeanSleepV1 = args[0] == "v1"
	if len(args) > 1 && args[1] == "--dump" {
		d, err := jeanMemoryDump()
		if err != nil {
			return err
		}
		fmt.Println(d)
		for i, g := range jeanFicheGroups(jeanFiches(true)) {
			size := 0
			var ns []string
			for _, f := range g {
				size += jeanFicheLen(f)
				ns = append(ns, f.Name)
			}
			fmt.Fprintf(os.Stderr, "[bench] groupe %d (%d car.) : %v\n", i+1, size, ns)
		}
		fmt.Fprintf(os.Stderr, "[bench] %s : %d groupe(s), %d caractères\n", args[0], jeanSleepGroups, len([]rune(d)))
		return nil
	}
	jeanReflect.mu.Lock()
	caps, temp := jeanTurnCapsLocked()
	jeanReflect.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), jeanSleepTimeout())
	defer cancel()
	ctx = context.WithValue(ctx, jeanReflectKey{}, true)
	sum, err := jeanConsolidateRun(ctx, cancel, caps, temp)
	if err != nil {
		return err
	}
	fmt.Println(sum)
	return nil
}
