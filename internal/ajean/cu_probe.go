package ajean

import (
	"fmt"
	"strings"
)

// cmdCUProbe : banc d'essai des outils du navigateur, sans modèle. Chaque
// argument est une étape « open:URL », « find:texte », « click:N », « snap »,
// « scroll:down » ; le résultat de chaque outil est affiché tel que le modèle le
// recevrait.
//
//	ajean cu-probe open:https://exemple.fr find:"256 Go" click:32 snap
func cmdCUProbe(args []string) error {
	defer func() {
		cdpMu.Lock()
		if cdpCur != nil {
			cdpCur.close()
			cdpCur = nil
		}
		cdpMu.Unlock()
	}()
	for _, a := range args {
		verb, val, _ := strings.Cut(a, ":")
		var out string
		switch verb {
		case "open":
			out = toolCUOpen(map[string]any{"url": val})
		case "find":
			out = toolCUFind(map[string]any{"text": val})
		case "click":
			var n int
			fmt.Sscanf(val, "%d", &n)
			out = toolCUClick(map[string]any{"ref": float64(n)})
		case "scroll":
			out = toolCUScroll(map[string]any{"direction": val})
		case "pick": // cherche puis clique le premier résultat
			f := toolCUFind(map[string]any{"text": val})
			var n int
			if i := strings.Index(f, "\n["); i >= 0 {
				fmt.Sscanf(f[i+2:], "%d", &n)
			}
			out = f + "\n--> " + toolCUClick(map[string]any{"ref": float64(n)})
		case "snap":
			out = toolCUSnapshot()
		default:
			return fmt.Errorf("étape inconnue : %s", a)
		}
		fmt.Printf("\n===== %s\n%s\n", a, out)
	}
	return nil
}
