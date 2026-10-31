package ajean

// Ce test joue le catalogue vivant du moteur avec le code Go qui le lira en
// service. Il ne se contente pas de verifier qu'un python sait repondre : il
// verifie que NOTRE decodage rend les familles, les couts et l'avis de la
// machine, et que le repli sur nos tables fonctionne quand le serveur n'est pas
// la. S'il echoue, le panneau tombe silencieusement sur la liste a quatre
// tailles : c'est exactement la regression qu'on ne veut pas.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// strataEngineForTest : l'arbre du moteur pose sur CE poste, ou skip.
//
// On ne cable aucun chemin : la regle du depot (decablage du 28/10/2026) est de
// TROUVER, parce que les installations sont portables et que le nom du dossier
// porte le millésime. On cherche donc les dossiers « strata-*-* » de moeHome()
// qui portent réellement le serveur de catalogue, du plus récent au plus ancien.
func strataEngineForTest(t *testing.T) string {
	t.Helper()
	racine := moeHome()
	entrees, err := os.ReadDir(racine)
	if err != nil {
		t.Skipf("pas de dossier moteur sous %s : %v", racine, err)
		return ""
	}
	var candidats []string
	for _, e := range entrees {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), "strata-") {
			continue
		}
		d := filepath.Join(racine, e.Name())
		if strataFileExists(filepath.Join(d, "tools", "strata_mcp.py")) &&
			strataFileExists(filepath.Join(d, "python", "python.exe")) {
			candidats = append(candidats, d)
		}
	}
	if len(candidats) == 0 {
		t.Skip("aucun arbre de moteur avec serveur de catalogue sur ce poste")
		return ""
	}
	// Le nom porte la version (« strata-0.1.54-hip ») : on trie en sens inverse
	// pour jouer le moteur le plus récent, celui qui tourne.
	sort.Sort(sort.Reverse(sort.StringSlice(candidats)))
	t.Logf("arbre moteur joué : %s", candidats[0])
	return candidats[0]
}

func TestCatalogueVivantServeur(t *testing.T) {
	src := strataEngineForTest(t)
	cat, err := strataAskModels(src)
	if err != nil {
		t.Fatalf("le serveur de catalogue du moteur n'a pas repondu : %v", err)
	}
	if len(cat.Families) < 8 {
		t.Errorf("familles = %d, on en attend au moins 8 (la liste a 2 valeurs est le defaut qu'on repare)", len(cat.Families))
	}
	if cat.Source == "" {
		t.Error("source absente : impossible de savoir d'ou vient la liste")
	}

	// Le compte, pas juste « non vide » : c'est la mesure du 31/10/2026.
	tailles := 0
	vus := map[string]bool{}
	for _, f := range cat.Families {
		tailles += len(f.Sizes)
		vus[f.Family] = true
	}
	t.Logf("catalogu : %d familles, %d tailles, source %q, contextes %v",
		len(cat.Families), tailles, cat.Source, cat.Contexts)
	for _, want := range []string{"qwen", "swift", "coder", "unsloth", "gyro", "nvfp4", "qwen36"} {
		if !vus[want] {
			t.Errorf("famille %s absente du catalogue vivant", want)
		}
	}

	// Un cout doit etre reel. Le piege qu'on ferme : une taille absente de nos
	// tables recopiees rendrait moeQuants = zero octet, donc « ca tient ».
	for _, f := range cat.Families {
		for _, s := range f.Sizes {
			if s.DownloadGB <= 0 {
				t.Errorf("%s %s : download_gb = %v, un cout a zero ferait annoncer « ca tient » a tort",
					f.Family, s.Model, s.DownloadGB)
			}
		}
	}

	// Les trois NVFP4 doivent etre ecartes sur notre carte AMD, meme si le
	// serveur les rend « fits » : c'est la mesure du 31/10/2026.
	if why := strataFamilyBlocked("nvfp4", "hip"); why == "" {
		t.Error("nvfp4 n'est pas bloque en HIP : le moteur HIP n'a pas de noyaux NVFP4 (docs/QUANTS.md)")
	}
	if why := strataFamilyBlocked("nvfp4", "cuda"); why != "" {
		t.Errorf("nvfp4 ne doit pas etre bloque en CUDA, on a eu : %q", why)
	}
	if why := strataFamilyBlocked("swift", "hip"); why != "" {
		t.Errorf("swift ne doit pas etre bloque, on a eu : %q", why)
	}
}

func TestCatalogueVivantContreNosTables(t *testing.T) {
	src := strataEngineForTest(t)
	cat, err := strataAskModels(src)
	if err != nil {
		t.Fatalf("serveur muet : %v", err)
	}
	// Ce que le panneau doit proposer desormais pour swift : tout ce que le
	// moteur connait, pas les quatre de moeQuants.
	qs, why := strataQuantsFor(cat, "swift", "hip")
	if why != "" {
		t.Fatalf("swift ecarte a tort : %s", why)
	}
	if len(qs) <= len(moeFamilies[0].Quants) {
		t.Errorf("tailles swift = %d, nos tables en ont %d : le catalogue n'apporte rien",
			len(qs), len(moeFamilies[0].Quants))
	}
	t.Logf("swift : %d tailles du moteur contre %d dans nos tables", len(qs), len(moeFamilies[0].Quants))

	// Une taille que nos tables ne nomment pas doit quand meme etre chiffrable.
	var inconnue *strataSizeInfo
	for _, f := range cat.Families {
		if f.Family != "qwen36" {
			continue
		}
		for i := range f.Sizes {
			if !moeHas(moeQuantsKeys(), f.Sizes[i].Model) {
				inconnue = &f.Sizes[i]
				break
			}
		}
	}
	if inconnue == nil {
		t.Log("toutes les tailles qwen36 sont deja dans nos tables ; rien a verifier")
		return
	}
	q := strataQuant(inconnue)
	if q.DownloadGB <= 0 || q.ID == "" {
		t.Errorf("%s : cout vide apres conversion", inconnue.Model)
	}
	t.Logf("taille inconnue de nos tables : %s · %.1f Go a telecharger · avis moteur %q · moteur mini %s",
		q.ID, q.DownloadGB, inconnue.OnThisPC, strataEngineMin(inconnue.EngineMin))
}

// moeQuantsKeys : les tailles que nos tables recopiees connaissent.
func moeQuantsKeys() []string {
	var out []string
	for k := range moeQuants {
		out = append(out, k)
	}
	return out
}

func TestCatalogueVivantRepli(t *testing.T) {
	// Le serveur n'est pas la : on doit retomber sur nos tables sans erreur
	// bloquante, et le panneau doit le dire.
	if _, err := strataAskModels(`C:\ProgramData\il-n-y-a-rien-ici`); err == nil {
		t.Fatal("un arbre inexistant ne doit pas rendre de catalogue")
	}
	// strataFamilies(nil) doit rendre vide : c'est ce qui declenche le repli
	// `src = moeFamilies` dans le handler.
	if got := strataFamilies(nil, "hip"); got != nil {
		t.Errorf("strataFamilies(nil) = %v, on attend nil pour que le repli se declenche", got)
	}
	if got, _ := strataQuantsFor(nil, "swift", "hip"); len(got) != 0 {
		t.Errorf("strataQuantsFor(nil) doit etre vide")
	}
	if got := strataSize(nil, "swift", "IQ3_S"); got != nil {
		t.Error("strataSize(nil) doit etre nil")
	}
}

// TestEstimationPlaceDéjàLà : les deux facons de compter ce qui est descendu
// doivent tomber d'accord sur les couples réellement installés. Si la nouvelle
// (celle du catalogue, qui doit marcher pour huit familles) sous-estimait, le
// panneau annoncerait un téléchargement complet de poids déjà présents ; si elle
// surestimait, l'installation échouerait en route faute de place.
func TestEstimationPlaceDéjàLà(t *testing.T) {
	src := strataEngineForTest(t)
	cat, err := strataAskModels(src)
	if err != nil {
		t.Fatalf("serveur muet : %v", err)
	}
	installés := 0
	for _, f := range cat.Families {
		for _, s := range f.Sizes {
			if !s.Installed {
				continue
			}
			installés++
			ancien := moeHaveGB(f.Family, s.Model)
			nouveau := strataHaveGB(cat, f.Family, s.Model)
			t.Logf("%-9s %-14s ancien %6.1f Go · catalogue %6.1f Go · écart %+6.1f Go",
				f.Family, s.Model, ancien, nouveau, nouveau-ancien)
			if nouveau < ancien*0.5 {
				t.Errorf("%s %s : le catalogue ne voit que %.1f Go sur %.1f Go installes",
					f.Family, s.Model, nouveau, ancien)
			}
		}
	}
	if installés == 0 {
		t.Log("aucun poids marqué installé par le moteur sur ce poste")
	}
}

// TestPanneauRendLeCatalogue : ce que l'application recoit vraiment. Sans ce
// test, on ne saurait pas si le panneau recoit la liste vivante ou la liste
// recopiée — et la regression serait silencieuse.
func TestPanneauRendLeCatalogue(t *testing.T) {
	if strataEngineForTest(t) == "" {
		return
	}
	cat, catErr := strataCatalogueGet()
	e := moeStrataEnvironment("")
	src := strataFamilies(cat, e.Backend)
	if len(src) == 0 {
		t.Skip("catalogue indisponible : rien à vérifier sur ce poste")
	}
	resp := strataPanelResponse(e, src, nil, nil, cat, catErr)
	if resp["catalogue"] != "moteur" {
		t.Errorf("catalogue = %v, attendu « moteur »", resp["catalogue"])
	}
	if resp["catalogue_source"] != "setup.py" {
		t.Errorf("source = %v, attendu setup.py", resp["catalogue_source"])
	}
	if _, ok := resp["contexts"]; !ok {
		t.Error("contexts absents : la fenetre proposee au curseur ne viendrait pas du moteur")
	}
	if n := len(src); n < 8 {
		t.Errorf("familles rendues au panneau = %d", n)
	}
	t.Logf("panneau : %d familles, source %v, contextes %v",
		len(src), resp["catalogue_source"], resp["contexts"])
	// Le repli doit etre visible, pas silencieux.
	repli := strataPanelResponse(e, moeFamilies, nil, nil, nil, errors.New("serveur absent"))
	if repli["catalogue"] != "local" || repli["catalogue_error"] == nil {
		t.Error("le repli ne dit pas que le catalogue du moteur est indisponible")
	}
}

// TestNommageSansCollision : avec huit familles, deux couples differents ne
// doivent jamais produire le meme nom de preset ni le meme tag de dossier.
//
// Ce qu'on protege ici, mesure le 31/10/2026 : le preset en service s'appelle
// « FLASH NEXT CLASSIQUE IQ3_S · HIP », et son MOE_CONFIG pointe sur
// « strata-iq3_s.json » (minuscules, parce que setup.py ecrit ce tag en minuscules
// dans le nom du fichier). Ces deux chaines ne doivent pas bouger : un preset
// renomme est un preset perdu, une config renommee est un moteur qui ne demarre plus.
func TestNommageSansCollision(t *testing.T) {
	couples := []struct{ fam, quant string }{
		{"qwen", "IQ1_M"}, {"coder", "IQ1_M"}, {"unsloth", "IQ1_M"},
		{"swift", "IQ3_S"}, {"qwen", "IQ3_S"}, {"qwen36", "UD-Q4_K_XL"},
		{"gyro", "Gyro-S"}, {"nvfp4", "NVFP4"},
	}
	presets := map[string]string{}
	tags := map[string]string{}
	for _, c := range couples {
		p := strataPresetName(c.fam, c.quant, "hip")
		g := moeTag(c.fam, c.quant)
		if autre, ok := presets[p]; ok {
			t.Errorf("collision de preset : %q porte par %s et %s", p, autre, c.fam+" "+c.quant)
		}
		if autre, ok := tags[g]; ok {
			t.Errorf("collision de dossier de poids : %q porte par %s et %s", g, autre, c.fam+" "+c.quant)
		}
		presets[p] = c.fam + " " + c.quant
		tags[g] = c.fam + " " + c.quant
	}
	if len(presets) != len(couples) || len(tags) != len(couples) {
		t.Errorf("%d couples mais %d presets / %d tags", len(couples), len(presets), len(tags))
	}
	// Les chaines en service sur ce poste, a l'octet pres.
	if got := strataPresetName("qwen", "IQ3_S", "hip"); got != "FLASH NEXT CLASSIQUE IQ3_S · HIP" {
		t.Errorf("nom du preset en service a change : %q", got)
	}
	if got := strataPresetName("swift", "IQ3_S", "hip"); got != "FLASH NEXT SWIFT 1.5 IQ3_S · HIP" {
		t.Errorf("nom du preset slider-meister a change : %q", got)
	}
	if got := moeTag("qwen", "IQ3_S"); got != "IQ3_S" {
		t.Errorf("tag de poids en service a change : %q (data\\models\\IQ3_S existe)", got)
	}
	if got := moeTag("swift", "IQ3_S"); got != "swift-IQ3_S" {
		t.Errorf("tag swift a change : %q", got)
	}
	// Ce que le catalogue vivant doit rendre pour une famille que nos tables
	// ne nomment pas : un tag qualifie, sinon il partagerait son dossier.
	for _, c := range []struct{ fam, quant, want string }{
		{"coder", "IQ1_M", "coder-IQ1_M"}, {"unsloth", "IQ1_M", "unsloth-IQ1_M"},
		{"qwen36", "UD-Q4_K_XL", "qwen36-UD-Q4_K_XL"}, {"gyro", "Gyro-S", "gyro-Gyro-S"},
	} {
		if got := moeTag(c.fam, c.quant); got != c.want {
			t.Errorf("tag %s %s = %q, attendu %q", c.fam, c.quant, got, c.want)
		}
		if got := strataPresetName(c.fam, c.quant, "hip"); got == strataPresetName("qwen", c.quant, "hip") {
			t.Errorf("preset %s %s identique a celui de qwen : %q", c.fam, c.quant, got)
		}
	}
	t.Logf("%d couples -> %d presets distincts, %d dossiers distincts", len(couples), len(presets), len(tags))
}

// TestPanneauFitsJamaisNull : le contrat JSON que l'interface attend.
//
// La régression, mesurée le 31/10/2026 sur le poste en service avec la révision 4.
// L'interface lit `f.fits.find(...)` sur TOUTES les familles, sans filtre :
//   - 05-status.js L500, dans loadCfg(), la branche ENGINE=moe :
//     `for(const f of s.families){ const x=f.fits.find(x=>x.active); … }`
//     Une famille à `fits: null` y lève → swapContent() jamais appelé → le panneau
//     « CONFIGURATION » reste bloqué sur son squelette de chargement.
//   - 10b-moe.js L56, dans openMoe(presetId) : la boucle qui cherche le preset
//     cliqué traverse les familles sans s'arrêter → la première famille à `null`
//     fait tomber l'ouverture AVANT l'affichage. Les trois engrenages des presets
//     MoE ne répondent plus ; le quatrième, llama.cpp, passe par openPreset() et
//     survit. C'est exactement ce que l'humain a vu et décrit.
//
// La cause : strataFamilyBlocked écarte NVFP4 hors CUDA (docs/QUANTS.md : « the HIP
// and SYCL engines have no NVFP4 kernels »), la famille ne rend donc AUCUNE taille,
// et un slice Go nil se marshall en `null` — pas en `[]`. Un seul défaut, deux
// symptômes, et les tests Go de la révision 4 ne l'ont pas vu : ils vérifiaient les
// tables, jamais ce que encoding/json en fait. D'où ce test, joué SANS moteur.
func TestPanneauFitsJamaisNull(t *testing.T) {
	e := moeStrataEnvironment("hip")
	if e.Backend != "hip" {
		t.Skipf("backend résolu = %q : ce test suppose un poste AMD", e.Backend)
	}
	if why := strataFamilyBlocked("nvfp4", e.Backend); why == "" {
		t.Skip("cette plateforme n'écarte aucune famille : rien à vérifier")
	}

	// Une famille que nos tables chiffrèrent, et une famille bloquée à zéro taille.
	src := []moeFamily{
		{ID: "qwen", Label: "Qwen3.8-Flash-Next", About: "a", Quants: []string{"IQ3_S"}},
		{ID: "nvfp4", Label: "Qwen3.8-Flash-Next (NVFP4)", About: "b", Quants: nil},
	}
	fams := strataPanelFamilies(e, nil, errors.New("catalogue non consulté : on teste le contrat, pas le moteur"),
		src, map[string]map[string]string{}, map[string]bool{})

	if len(fams) != 2 {
		t.Fatalf("familles fabriquées = %d, attendu 2", len(fams))
	}

	brut, err := json.Marshal(strataPanelResponse(e, fams, nil, nil, nil, nil))
	if err != nil {
		t.Fatalf("marshage du panneau : %v", err)
	}

	// Le symptôme, pris à la source : le mot que le JavaScript ne sait pas lire.
	if strings.Contains(string(brut), `"fits":null`) {
		t.Errorf("le panneau rend « \"fits\":null » : loadCfg() et openMoe() vont lever sur cette famille")
	}

	// Et la forme attendue, famille par famille, relue comme le navigateur la reçoit.
	var relu struct {
		Families []struct {
			ID      string          `json:"id"`
			Fits    json.RawMessage `json:"fits"`
			Blocked string          `json:"blocked"`
		} `json:"families"`
	}
	if err := json.Unmarshal(brut, &relu); err != nil {
		t.Fatalf("relecture du panneau : %v", err)
	}
	if len(relu.Families) != 2 {
		t.Fatalf("familles relues = %d", len(relu.Families))
	}
	for _, f := range relu.Families {
		if len(f.Fits) == 0 {
			t.Errorf("%s : la clé « fits » est absente de la réponse", f.ID)
			continue
		}
		if string(f.Fits) == "null" {
			t.Errorf("%s : fits = null, le JavaScript lira « Cannot read properties of null »", f.ID)
		}
		if !bytes.HasPrefix(f.Fits, []byte("[")) {
			t.Errorf("%s : fits = %s, attendu un tableau", f.ID, string(f.Fits))
		}
		if f.ID == "nvfp4" {
			if string(f.Fits) != "[]" {
				t.Errorf("nvfp4 : fits = %s, attendu [] (la famille est écartée, ses tailles aussi)", string(f.Fits))
			}
			if f.Blocked == "" {
				t.Error("nvfp4 : écartée mais sans motif — le panneau afficherait une famille muette")
			}
			t.Logf("nvfp4 : fits=[] · blocked = %q", f.Blocked)
		}
		if f.ID == "qwen" && string(f.Fits) == "[]" {
			t.Error("qwen : fits vide alors que la famille a une taille chiffrée dans nos tables")
		}
	}
	t.Logf("contrat vérifié sur %d octets de réponse : « fits » est un tableau partout", len(brut))
}
