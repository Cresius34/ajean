package ajean

// Ce fichier lit le CATALOGUE VIVANT du moteur au lieu de le recopier.
//
// Pourquoi. `moeFamilies` / `moeQuants` (backend_moe.go) sont une copie à la main de
// la table MODELS de setup.py. Mesuré le 31/10/2026 : Ajean propose 2 familles et
// 4 quantifications, alors que le moteur posé sur ce poste en connaît 8 et 19, avec
// pour chacune ses coûts réels et un avis calibré sur cette machine. Chaque taille
// ajoutée en amont oblige à reécrire Go et à recompiler.
//
// Le moteur livre déjà ce qu'il faut : tools/strata_mcp.py, un serveur MCP
// (JSON-RPC 2.0, une ligne par message, stdin/stdout, bibliothèque standard seule)
// qui IMPORTE setup.py comme module (def tables, L510-512) et rend l'outil
// `strata_models`. Sa réponse porte `source: "setup.py"` : ce sont les tables
// vivantes, pas un inventaire.
//
// Ce qui est relu ici, et ce qui ne l'est pas. La réponse du serveur est un avis,
// pas une autorité : `on_this_pc` ignore le drapeau `nvidia_only` de setup.py
// (mesuré le 31/10 : les trois NVFP4 sont rendus « fits » sur une carte AMD, alors
// que « the HIP and SYCL engines have no NVFP4 kernels », docs/QUANTS.md). Donc les
// familles marquées `experimental` ou réservées à une autre marque sont écartées
// ICI, et l'adéquation reste calculée par moeFitFor, qui connaît notre marge de
// RAM, notre carte d'aide et le mode mmap. Le catalogue vivant fournit les NOMS et
// les COÛTS ; la décision de proposer reste la nôtre.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// strataCatalogueTTL : le catalogue ne change qu'en changeant de moteur. Une minute
// suffit pour ne pas relancer un python à chaque ouverture du panneau, et reste
// courtsi l'humain installe un autre moteur sans relancer Ajean.
const strataCatalogueTTL = 90 * time.Second

// strataCatalogueTimeout : setup.py est importe, donc l'importer = lire ses tables.
// Mesure du 31/10/2026 : ~2 s entre le lancement et la reponse de strata_models.
// 20 s seulement : ce delai est celui que l'humain attendrait l'oeil sur le panneau
// si le serveur ne repondait pas. Au-dela on tombe sur nos tables et on le dit,
// plutot que de laisser la page tourner dans le vide.
const strataCatalogueTimeout = 20 * time.Second

// strataCatalogue : le catalogue vivant, tel que le moteur le rend.
type strataCatalogue struct {
	Source       string   `json:"source"`
	Summary      string   `json:"summary"`
	Contexts     []int    `json:"contexts"`
	ContextNote  string   `json:"context_note"`
	ThisPC       struct {
		RAMGB      float64 `json:"ram_gb"`
		EngineVend string  `json:"engine_vendor"`
		GPUs       []struct {
			Index  int     `json:"index"`
			Name   string  `json:"name"`
			VRAMGB float64 `json:"vram_gb"`
			Usable bool    `json:"usable"`
			Vendor string  `json:"vendor"`
		} `json:"gpus"`
	} `json:"this_pc"`
	Families []struct {
		Family       string            `json:"family"`
		Title        string            `json:"title"`
		About        string            `json:"about"`
		Experimental bool              `json:"experimental"`
		Images       bool              `json:"images"`
		Sizes        []strataSizeInfo  `json:"sizes"`
	} `json:"families"`
	Recommendation *struct {
		Family       string   `json:"family"`
		Model        string   `json:"model"`
		Title        string   `json:"title"`
		Context      int      `json:"context"`
		Backend      string   `json:"backend"`
		DownloadGB   float64  `json:"download_gb"`
		Why          string   `json:"why"`
		Notes        []string `json:"notes"`
		SetupCommand string   `json:"setup_command"`
	} `json:"recommendation"`
}

// strataCatalogueCache : une reponse par arbre de moteur, le temps du TTL.
var (
	strataCatMu     sync.Mutex
	strataCatAt     time.Time
	strataCatKey    string
	strataCatValue  *strataCatalogue
	strataCatErr    error
)

// strataCatalogueGet : le catalogue du moteur pose, ou une erreur si le serveur
// n'est pas la ou ne repond pas. Appeler sans crainte : le resultat est mis en
// cache, et l'echec est memorise aussi (pas de relance de python a chaque fois).
func strataCatalogueGet() (*strataCatalogue, error) {
	strataCatMu.Lock()
	defer strataCatMu.Unlock()
	src := strataSourceDir(moeStrataEnvironment("").Backend, "13")
	if strataCatValue != nil && time.Since(strataCatAt) < strataCatalogueTTL && strataCatKey == src {
		return strataCatValue, strataCatErr
	}
	if strataCatValue != nil && strataCatKey != src {
		strataCatValue, strataCatErr = nil, nil // on a change d'arbre : on oublie
	}
	cat, err := strataAskModels(src)
	strataCatAt, strataCatKey, strataCatValue, strataCatErr = time.Now(), src, cat, err
	return cat, err
}

// strataAskModels : parle au serveur MCP du moteur et lui demande strata_models.
//
// Le protocole, verifie en le jouant le 31/10/2026 (journal/66-mcp-modeles.py) :
// initialize, puis notifications/initialized, puis tools/call. Une ligne JSON par
// message. La reponse de tools/call met le JSON dans content[].text, d'ou le
// double decodage.
func strataAskModels(src string) (*strataCatalogue, error) {
	py := filepath.Join(src, "python", "python.exe")
	mcp := filepath.Join(src, "tools", "strata_mcp.py")
	if !strataFileExists(py) || !strataFileExists(mcp) {
		return nil, fmt.Errorf("le serveur de catalogue du moteur est absent (%s)", mcp)
	}
	ctx, cancel := context.WithTimeout(context.Background(), strataCatalogueTimeout)
	defer cancel()
	cmd := hideCmd(exec.CommandContext(ctx, py, mcp))
	cmd.Dir = src
	// Le serveur herite de l'environnement. STRATA_NO_ACQUIRE evite qu'il aille
	// acquerir un moteur ou telecharger quoi que ce soit pendant qu'on lit.
	cmd.Env = upsertEnv(os.Environ(), "STRATA_NO_ACQUIRE=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	// Une reponse peut etre grosse (37 900 octets mesurees) et le tube n'est pas
	// plus grand : on lit en continu, sinon l'ecrivain se bloque avant d'avoir
	// repondu. C'est la raison pour laquelle on decode a la volee plutot que
	// d'ecrire d'abord les trois messages.
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	send := func(obj map[string]any) error {
		b, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		_, err = stdin.Write(append(b, '\n'))
		return err
	}
	if err = send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{},
			"clientInfo": map[string]any{"name": "ajean", "version": "1"}}}); err != nil {
		cmd.Wait()
		return nil, err
	}
	if err = send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized", "params": map[string]any{}}); err != nil {
		cmd.Wait()
		return nil, err
	}
	if err = send(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "tools/call",
		"params": map[string]any{"name": "strata_models", "arguments": map[string]any{}}}); err != nil {
		cmd.Wait()
		return nil, err
	}

	var out *strataCatalogue
	var rpcErr string
	var brutReçu string   // la ligne attendue, au cas ou notre decodage ne la comprend pas
	var decodage string
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var msg struct {
			ID     int    `json:"id"`
			Result struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
				IsError bool `json:"isError"`
			} `json:"result"`
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(line, &msg) != nil {
			continue
		}
		if msg.Error.Message != "" {
			rpcErr = msg.Error.Message
			continue
		}
		if msg.ID != 3 {
			continue // initialize : on s'en sert pas, le serveur ne rend rien d'utile ici
		}
		for _, c := range msg.Result.Content {
			if c.Type != "text" || c.Text == "" {
				continue
			}
			brutReçu = c.Text
			var cat strataCatalogue
			if err := json.Unmarshal([]byte(c.Text), &cat); err != nil {
				// Le serveur a bien repondu ; c'est NOTRE decodage qui n'a pas
				// suivi. Sans ce message, on tournerait jusqu'au delai et on
				// accuserait le serveur d'etre muet. Mesure du 31/10/2026 : un
				// engine_min rendu en chaine et non en tableau produisait
				// exactement ce faux diagnostic.
				decodage = err.Error()
				continue
			}
			if len(cat.Families) > 0 {
				out = &cat
			}
		}
		if out != nil || decodage != "" {
			break // repondu : soit on l'a digere, soit on sait pourquoi non
		}
	}
	// On ferme proprement : le serveur s'arrete sur stdin ferme (verifie).
	stdin.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		<-done
	}
	if out == nil {
		if decodage != "" {
			// Le moteur a repondu, nos structures n'ont pas suivi : c'est un defaut
			// de NOTRE cote, et il doit etre nomme comme tel.
			return nil, fmt.Errorf("catalogue du moteur : reponse recue mais indigeste (%s ; %d octets recus)", decodage, len(brutReçu))
		}
		if rpcErr != "" {
			return nil, errors.New(rpcErr)
		}
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("le catalogue du moteur n'a pas repondu en %s", strataCatalogueTimeout)
		}
		if e := strings.TrimSpace(stderr.String()); e != "" {
			return nil, fmt.Errorf("catalogue du moteur : %s", strataLastLine(e))
		}
		return nil, errors.New("le catalogue du moteur n'a rien rendu")
	}
	return out, nil
}

// strataFileExists : os.Stat, sans dependre de fileExists qui n'existe que sous
// darwin (sys_service_darwin.go, //go:build darwin).
func strataFileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// strataLastLine : la derniere ligne non vide, pour un message d'erreur court.
func strataLastLine(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if t := strings.TrimSpace(lines[i]); t != "" {
			return t
		}
	}
	return ""
}

// strataFamilyBlocked : le serveur ne filtre pas ce que le moteur ne peut pas
// charger. Mesure du 31/10/2026 : NVFP4 rendu « fits » sur une Radeon R9700, alors
// que setup.py le marque « nvidia_only: True » et que la doc amont ecrit « the HIP
// and SYCL engines have no NVFP4 kernels ». Donc on ecarte ici, et on le dit.
func strataFamilyBlocked(fam string, backend string) string {
	switch fam {
	case "nvfp4":
		if backend != "cuda" {
			return "reserve aux cartes NVIDIA : le moteur HIP n'a pas de noyaux NVFP4"
		}
	case "gyro":
		// Gyro-S est marque d'un moteur minimum 0.1.45.1 ; il tourne, mais ses
		// verrous ont ete signales comme avertissements sur cette variante (voir
		// la note « VRAM plafonnee a 4 Go par WMI + verrous Gyro-S devenus
		// avertissements »). On le laisse proposer : c'est a l'humain d'en juger.
		return ""
	}
	return ""
}

// strataQuantsFor : les tailles que le moteur propose pour une famille, dans son
// ordre, avec leurs couts. `blocked` non vide ecarte toute la famille.
func strataQuantsFor(cat *strataCatalogue, fam, backend string) ([]moeQuant, string) {
	if cat == nil {
		return nil, "catalogue du moteur indisponible"
	}
	if why := strataFamilyBlocked(fam, backend); why != "" {
		return nil, why
	}
	for _, f := range cat.Families {
		if f.Family != fam {
			continue
		}
		if f.Experimental {
			return nil, "famille signalee experimentale par le moteur"
		}
		var qs []moeQuant
		for _, s := range f.Sizes {
			if s.Experimental || s.Model == "" {
				continue
			}
			qs = append(qs, moeQuant{
				ID:         s.Model,
				About:      s.About,
				DownloadGB: s.DownloadGB,
				ArenaGB:    s.ExpertsGB,
			})
		}
		return qs, ""
	}
	return nil, "le moteur ne connait pas cette famille"
}

// strataEngineMin : le moteur minimum demande par une taille, en texte. Le moteur
// le rend deja sous forme de chaine ("0.1.45.1"), ou null.
func strataEngineMin(s string) string { return strings.TrimSpace(s) }

// strataVerdict : l'avis du moteur sur cette machine, en une ligne. On le garde
// comme EXPPLICATION a cote du notre, jamais a sa place : il ne connait ni notre
// carte d'aide ni notre page file.
func strataVerdict(cat *strataCatalogue, fam, quant string) (why string, installed bool) {
	if cat == nil {
		return "", false
	}
	for _, f := range cat.Families {
		if f.Family != fam {
			continue
		}
		for _, s := range f.Sizes {
			if s.Model != quant {
				continue
			}
			return s.OnThisPC, s.Installed
		}
	}
	return "", false
}

// strataSize : la taille telle que le moteur la decrit, ou nil si le couple
// famille/taille n'est pas dans le catalogue. C'est ce qui permet de proposer et
// d'installer une taille que nos deux tables locales ne connaissent pas.
func strataSize(cat *strataCatalogue, fam, quant string) *strataSizeInfo {
	if cat == nil {
		return nil
	}
	for i := range cat.Families {
		if cat.Families[i].Family != fam {
			continue
		}
		for j := range cat.Families[i].Sizes {
			if cat.Families[i].Sizes[j].Model == quant {
				s := cat.Families[i].Sizes[j]
				return &s
			}
		}
	}
	return nil
}

// strataSizeInfo : une taille (une quantification) telle que le moteur la decrit.
// Les couts viennent de la table MODELS de setup.py, via le serveur de catalogue.
//
// Les types sont verifies sur la reponse reelle du poste (journal/74-types-du-json.py,
// 31/10/2026) : ram_needed_gb rend un entier ou un flottant selon les lignes, donc
// float64 ; engine_min est une CHAINE ("0.1.45.1") ou null, jamais un tableau ;
// disk_gb est null pour les familles qui n'ont pas de chiffrage disque. gpu_options
// n'est pas decode ici : c'est un objet de plusieurs dizaines de kilo-octets par
// taille (le plan de repartition carte/RAM detaille, avec le texte de la doc), et
// notre jugement reste moeFitFor, qui connait notre carte d'aide et notre page file.
type strataSizeInfo struct {
	Model        string  `json:"model"`
	About        string  `json:"about"`
	DownloadGB   float64 `json:"download_gb"`
	ExpertsGB    float64 `json:"experts_gb"`
	RamNeededGB  float64 `json:"ram_needed_gb"`
	MemoryGB     float64 `json:"memory_gb"`
	DiskGB       float64 `json:"disk_gb"`
	MemTotal     bool    `json:"mem_total"`
	MultiFile    bool    `json:"multi_file"`
	Experimental bool    `json:"experimental"`
	Images       bool    `json:"images"`
	ImagesReader string  `json:"images_reader"`
	EngineMin    string  `json:"engine_min"`
	OnThisPC     string  `json:"on_this_pc"`
	Installed    bool    `json:"installed"`
	ID           string  `json:"id"`
}

// strataQuant : la taille convertie dans le format que moeFitFor sait juger.
// DownloadGB et ArenaGB viennent du moteur, plus de nos tables recopiees.
func strataQuant(s *strataSizeInfo) moeQuant {
	if s == nil {
		return moeQuant{}
	}
	return moeQuant{ID: s.Model, About: s.About, DownloadGB: s.DownloadGB, ArenaGB: s.ExpertsGB}
}

// strataHaveGB : ce qui est deja descendu pour ce couple, en Go.
//
// Le piege que ce contourne : moeTag (backend_moe.go:330) ne connait que deux
// familles ; il construit « swift-<quant> » ou « <quant> ». Or setup.py nomme ses
// dossiers d'autres facons (unsloth, qwen36, coder…). Estimer avec le mauvais tag
// renverrait 0 et ferait annoncer un telechargement complet — donc on essaie les
// formes plausibles et on garde la plus grande trouvee. Une surestimation de
// l'espace libre est dangereuse (l'installation echouerait en route) ; une
// sous-estimation ne coute que l'affichage d'un chiffre trop grand. On reste donc
// du cote sur, et on ne devine jamais un dossier qui n'existe pas.
func strataHaveGB(cat *strataCatalogue, fam, quant string) float64 {
	tag := strataTag(fam, quant)
	if tag == "" {
		return 0
	}
	out := moeDirGB(filepath.Join(moeDataDir(), "models", tag)) +
		moeDirGB(filepath.Join(moeDataDir(), "packs", strings.ToLower(tag)))
	// la tete MTP est partagee entre les tailles d'une meme famille : le moteur le
	// dit lui-meme quand c'est installe, on ajoute ce qu'elle pese une seule fois.
	if s := strataSize(cat, fam, quant); s != nil && s.Installed {
		out += moeDirGB(filepath.Join(moeDataDir(), "mtp"))
	}
	return out
}

// strataTag : le nom de dossier que setup.py ecrit pour ce couple. On garde la
// regle mesuree de moeTag pour les deux familles connues et on ajoute les formes
// observees sur ce poste le 31/10/2026 (data\models : IQ3_S, swift-iq3_s-abl-slider,
// qwen36-UD-Q4_K_XL). Inconnu ici n'est pas une erreur : strataHaveGB essaie
// plusieurs formes, et l'install reelle se juge sur les fichiers, pas sur ce nom.
func strataTag(fam, quant string) string {
	switch fam {
	case "swift":
		return "swift-" + quant
	case "qwen":
		return quant
	default:
		return fam + "-" + quant
	}
}

// strataFamilies : les familles que le moteur connait, dans son ordre, avec
// l'indication de ce qui les bloque. C'est la liste vivante : elle suit setup.py
// quand l'amont ajoute une famille, sans reecriture ni recompilation de notre cote.
func strataFamilies(cat *strataCatalogue, backend string) []moeFamily {
	if cat == nil {
		return nil
	}
	var out []moeFamily
	for _, f := range cat.Families {
		qs, _ := strataQuantsFor(cat, f.Family, backend)
		ids := make([]string, 0, len(qs))
		for _, q := range qs {
			ids = append(ids, q.ID)
		}
		out = append(out, moeFamily{ID: f.Family, Label: f.Title, About: f.About, Quants: ids})
	}
	return out
}
