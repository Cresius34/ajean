package ajean

// jean_consolidate_v2.go — le sommeil, version 2.
//
// Constat sur une vraie mémoire (33 fiches, 190 000 caractères) : au-delà de
// jeanConsolidateFullChars, la v1 ne donne que la LISTE des fiches. Le modèle ne
// voit pas leur contenu, ne repère aucun doublon et finit avec 0 écriture alors
// que quatre fiches parlent de la même newsletter.
//
// La v2 :
//   - repère côté serveur les fiches qui se ressemblent (nom ou contenu) et les
//     donne EN ENTIER, par groupes, avec la consigne de les traiter d'abord ;
//   - donne aussi en entier les fiches changées depuis le dernier sommeil, dans
//     un budget de caractères ;
//   - laisse plus d'actions quand il y a des groupes à fusionner ;
//   - est tout ou rien : un sommeil coupé (arrêt, panne, redémarrage) remet la
//     mémoire telle qu'avant ;
//   - note toujours son bilan dans le journal, même « rien à changer ».

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	jeanSleepV2Budget  = 110000 // caractères de fiches donnés en entier au plus (~30 000 tokens)
	jeanSleepV2MaxIter = 60     // actions quand il y a des groupes à traiter
	jeanSleepNameSim   = 1      // mots de nom en commun pour rapprocher deux fiches
	jeanSleepBodySim   = 0.5    // part des mots de la petite fiche présents dans la grande
)

// jeanSleepCtxKey : le tour est un sommeil. Plus strict qu'une révision : aucun
// fichier écrit (vu au banc : le sommeil « testait » l'outil d'écriture).
type jeanSleepCtxKey struct{}

func isJeanSleepCtx(ctx context.Context) bool { v, _ := ctx.Value(jeanSleepCtxKey{}).(bool); return v }

// jeanSleepTimeout : durée max d'un sommeil. La v2 fait jusqu'à 60 actions à
// ~30 s chacune sur un gros modèle : 15 min coupaient au milieu.
func jeanSleepTimeout() time.Duration {
	if jeanSleepV1 {
		return 15 * time.Minute
	}
	return 40 * time.Minute
}

// jeanSleepV1 : force l'ancien sommeil (banc d'essai, comparaison).
var jeanSleepV1 = os.Getenv("AJEAN_JEAN_SLEEP_V1") != ""

// Mots de nom trop généraux pour rapprocher deux fiches à eux seuls.
var jeanSleepNameStop = map[string]bool{"fiche": true, "jean": true, "ajean": true, "suivi": true, "etat": true, "lire": true, "note": true, "liste": true, "faire": true, "propre": true, "dernier": true, "envoi": true}

func jeanNameWords(name string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Split(foldSearch(name), "-") {
		if len([]rune(w)) >= 4 && !jeanSleepNameStop[w] {
			out[w] = true
		}
	}
	return out
}

// jeanFicheGroups : les fiches qui se ressemblent, par groupes de 2 ou plus.
func jeanFicheGroups(fs []jeanFiche) [][]jeanFiche {
	n := len(fs)
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	find := func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	names := make([]map[string]bool, n)
	bodies := make([]map[string]bool, n)
	for i, f := range fs {
		names[i] = jeanNameWords(f.Name)
		bodies[i] = jeanWords(f.When + " " + f.Content)
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			shared := 0
			for w := range names[i] {
				if names[j][w] {
					shared++
				}
			}
			small, big := bodies[i], bodies[j]
			if len(small) > len(big) {
				small, big = big, small
			}
			if shared >= jeanSleepNameSim || len(small) >= 20 && jeanSimilar(small, big) >= jeanSleepBodySim {
				parent[find(i)] = find(j)
			}
		}
	}
	byRoot := map[int][]jeanFiche{}
	for i, f := range fs {
		byRoot[find(i)] = append(byRoot[find(i)], f)
	}
	var out [][]jeanFiche
	for _, g := range byRoot {
		if len(g) >= 2 {
			out = append(out, g)
		}
	}
	// Les plus petits groupes d'abord : ils tiennent dans le budget et se
	// fusionnent sûrement ; un énorme groupe passe après.
	sort.Slice(out, func(a, b int) bool {
		if len(out[a]) != len(out[b]) {
			return len(out[a]) < len(out[b])
		}
		return out[a][0].Name < out[b][0].Name
	})
	return out
}

// jeanFicheLimit : taille max d'une fiche après écriture. Une fiche déjà plus
// grande que la limite ordinaire (fruit d'une fusion) peut être retouchée ou
// réécrite tant qu'elle ne grossit pas, sans dépasser la limite d'une fusion :
// sinon même une retouche qui la raccourcit était refusée.
func jeanFicheLimit(current int) int { return min(max(jeanFicheMaxChars, current), jeanMergeMaxChars) }

func jeanFicheLen(f jeanFiche) int { return len([]rune(f.Content)) }

// jeanSleepV2Fiches écrit la partie FICHES du sommeil v2. Renvoie le nombre de
// groupes donnés en entier.
func jeanSleepV2Fiches(b *strings.Builder, fs []jeanFiche, usage map[string]*jeanFicheUse) int {
	userLoc := loc(defaultTaskTZ())
	head := func(f jeanFiche) string {
		changed := time.UnixMilli(f.ModTime).In(userLoc).Format("2006-01-02 15:04")
		return fmt.Sprintf("%s (when: %s) · changed %s · %s", f.Name, f.When, changed, jeanFicheUsageLine(f.Name, usage))
	}
	total := 0
	for _, f := range fs {
		total += jeanFicheLen(f)
	}
	if total <= jeanConsolidateFullChars {
		b.WriteString("\n## FICHES (most recently changed first)\n")
		for _, f := range fs {
			fmt.Fprintf(b, "\n### fiche %s\n%s\n", head(f), f.Content)
		}
		return 0
	}
	full := map[string]bool{}
	budget := jeanSleepV2Budget
	groups := 0
	var gb strings.Builder
	for _, g := range jeanFicheGroups(fs) {
		size := 0
		for _, f := range g {
			size += jeanFicheLen(f)
		}
		if size > budget {
			continue
		}
		budget -= size
		groups++
		var names []string
		for _, f := range g {
			names = append(names, f.Name)
		}
		fmt.Fprintf(&gb, "\n### GROUP %d: %s\n", groups, strings.Join(names, ", "))
		for _, f := range g {
			full[f.Name] = true
			fmt.Fprintf(&gb, "\n#### fiche %s\n%s\n", head(f), f.Content)
		}
	}
	if groups > 0 {
		b.WriteString("\n## FICHES THAT LOOK RELATED (given in full: deal with these FIRST)\n")
		b.WriteString(`Each group was matched by name or content; their full text is here, do not jean_read them again. For each pair in a group, decide:
- SAME TASK (one is a step, an older version, a status log or a follow-up of the other; the user would ask for them the same way): MERGE. Pick the reference (the most complete fiche, or the one other fiches already point to), then ONE call: jean_merge from=<other> into=<reference>. The server copies the other fiche in full as a new section and archives it: nothing is lost, the size limit is higher for a merge. Then, if the new section repeats lines of the reference, remove the repeats with jean_patch. A pointer line is NOT a merge.
- NEIGHBOURS (different tasks that only share a topic or a tool): keep both. Remove from each what belongs to the other (keep it only where it belongs), at most one pointer line.
Key test: when one fiche's "when" already covers the other's request (a broad fiche "any request about X" and a narrow fiche about one part of X, one past run of X, or one status of X), they are the SAME TASK: the narrow one becomes a section of the broad one. Two fiches the user would reach with the same question must never coexist: future you would open the wrong one.
Expect at least one merge when two fiches describe the same thing; never merge neighbours.
`)
		b.WriteString(gb.String())
	}
	// Fiches changées depuis le dernier sommeil, dans ce qui reste du budget.
	last, _ := jeanMemTimes()
	var recent []string
	for _, f := range fs {
		if full[f.Name] || !time.UnixMilli(f.ModTime).After(last) || jeanFicheLen(f) > budget {
			continue
		}
		if len(recent) == 0 {
			b.WriteString("\n## FICHES CHANGED SINCE THE LAST SLEEP (given in full)\n")
		}
		budget -= jeanFicheLen(f)
		full[f.Name] = true
		recent = append(recent, f.Name)
		fmt.Fprintf(b, "\n### fiche %s\n%s\n", head(f), f.Content)
	}
	b.WriteString("\n## OTHER FICHES (index only, jean_read to open)\n")
	for _, f := range fs {
		if !full[f.Name] {
			fmt.Fprintf(b, "- %s · %d chars\n", head(f), jeanFicheLen(f))
		}
	}
	return groups
}

// jeanSleepV2Prompt : la consigne v1 plus ce que Sillage a appris.
func jeanSleepV2Prompt() string {
	p := jeanConsolidatePrompt()
	extra := `
Also:
- Start with FICHES THAT LOOK RELATED when that section is there: it is where the duplicates are.
- A lesson that is only a fact about the user (a name, a date, a taste, with no mistake behind it): jean_remember it in the profile, then jean_forget that lesson.
- Take a line out only with jean_forget or jean_patch: a line that disappears any other way (a whole fiche rewritten without it) is put back by the memory guard.
- No dash as punctuation (a comma or a colon instead).`
	return strings.Replace(p, "\nRules: never invent anything;", extra+"\nRules: never invent anything;", 1)
}

// jeanRollback remet la mémoire telle que dans snap : fichiers réécrits, fiches
// créées pendant le sommeil retirées, fiches archivées pendant le sommeil
// ressorties.
func jeanRollback(snap jeanMemSnap) error {
	jeanMu.Lock()
	defer jeanMu.Unlock()
	dir := jeanDir()
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(n, jeanFichePrefix) && strings.HasSuffix(n, ".md") {
			if _, ok := snap[n]; !ok {
				_ = os.Remove(filepath.Join(dir, n))
			}
		}
		if strings.HasPrefix(n, jeanArchivePrefix) {
			orig := jeanFichePrefix + strings.TrimPrefix(n, jeanArchivePrefix)
			if _, ok := snap[orig]; ok {
				_ = os.Remove(filepath.Join(dir, n))
			}
		}
	}
	for n, s := range snap {
		out, err := encodeMemContent([]byte(s))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, n), out, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// jeanMergeMaxChars : une fusion peut dépasser la limite d'une fiche ordinaire
// (vu au banc : la fiche principale à 16 600 car. ne pouvait recevoir aucune de
// ses petites sœurs sous 20 000, le sommeil renonçait à fusionner).
const (
	jeanMergeMaxChars = 32000
	jeanMergeWhenMax  = 400 // la phrase « quand » d'une fiche fusionnée cumule les déclencheurs
)

// JeanMerge (outil jean_merge) : verse une fiche ENTIÈRE dans une autre, comme
// nouvelle section, puis archive la source. Rien ne se perd ; un sommeil suivant
// peut resserrer la section (jean_patch).
func JeanMerge(from, into string) (string, error) {
	from, into = normFicheName(from), normFicheName(into)
	if from == "" || into == "" || from == into {
		return "", fmt.Errorf("donne deux fiches différentes : from (absorbée) et into (référence)")
	}
	jeanMu.Lock()
	src, err := jeanRead(ficheFile(from))
	if err == nil && src == "" {
		err = fmt.Errorf("fiche « %s » introuvable", from)
	}
	dst, err2 := jeanRead(ficheFile(into))
	if err == nil && err2 == nil && dst == "" {
		err2 = fmt.Errorf("fiche « %s » introuvable", into)
	}
	if err != nil || err2 != nil {
		jeanMu.Unlock()
		if err != nil {
			return "", err
		}
		return "", err2
	}
	sf, df := parseFiche(from, src), parseFiche(into, dst)
	// Les titres de la source descendent d'un niveau : ils deviennent des
	// sous-sections de la nouvelle section.
	var body strings.Builder
	for _, l := range strings.Split(strings.TrimSpace(sf.Content), "\n") {
		if strings.HasPrefix(l, "#") {
			l = "#" + l
		}
		body.WriteString(l + "\n")
	}
	// Titre court (le nom de la fiche absorbée), la phrase « quand » juste dessous.
	sec := "\n\n## " + strings.ReplaceAll(from, "-", " ") + " (ex-fiche " + from + ")\n"
	if sf.When != "" {
		sec += "_Quand : " + sf.When + "_\n\n"
	}
	df.Content = strings.TrimSpace(df.Content) + sec + strings.TrimSpace(body.String())
	// Les déclencheurs de la fiche absorbée rejoignent ceux de la référence : sinon
	// la question qui menait à elle ne mène plus nulle part.
	if sf.When != "" && !strings.Contains(df.When, sf.When) {
		df.When = clipRunes(strings.TrimSpace(df.When)+" ; aussi : "+sf.When, jeanMergeWhenMax)
	}
	if n := len([]rune(df.Content)); n > jeanMergeMaxChars {
		jeanMu.Unlock()
		return "", fmt.Errorf("fusion trop longue (%d car., max %d) : resserre d'abord une des deux fiches", n, jeanMergeMaxChars)
	}
	err = jeanWrite(ficheFile(into), "# "+into+"\nwhen: "+df.When+"\n\n"+df.Content+"\n")
	if err == nil {
		_ = jeanAppendLocked("fiche", fmt.Sprintf("Fiche « %s » fusionnée dans « %s »", from, into))
	}
	jeanMu.Unlock()
	if err != nil {
		return "", err
	}
	if err := jeanArchiveFiche(from); err != nil {
		return "", err
	}
	return fmt.Sprintf("[ok] « %s » versée en entier dans « %s » (nouvelle section), puis archivée. Resserre la section avec jean_patch si des lignes y répètent la fiche.", from, into), nil
}
