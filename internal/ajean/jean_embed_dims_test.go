package ajean

// Le journal de Jean et la mémoire sémantique partagent le même fichier de
// modèle. Le contrôle de largeur est le seul rempart contre un modèle
// différent : un autre encodeur répond avec un autre nombre de dimensions, ou
// des valeurs non finies, et l'index se remplit de vecteurs faux pendant que
// tous les autres voyants restent au vert. Ces tests vérifient que ce rejet a
// bien lieu.

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// fakeEmbedRawServer : un faux moteur qui répond exactement les vecteurs qu'on
// lui donne, largeur et valeurs comprises.
func fakeEmbedRawServer(t *testing.T, vecs func(n int) [][]float64) int {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input []string `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		out := vecs(len(req.Input))
		data := make([]map[string]any, 0, len(out))
		for _, v := range out {
			data = append(data, map[string]any{"embedding": v})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	p, _ := strconv.Atoi(u.Port())
	return p
}

// TestEG2DimensionsExigees : un moteur qui répond avec une autre largeur que
// jeanEmbedDims est un autre modèle, donc un autre espace vectoriel.
func TestEG2DimensionsExigees(t *testing.T) {
	for _, tc := range []struct {
		name string
		dims int
		want string // "" = accepté, sinon sous-chaîne attendue de l'erreur
	}{
		{"largeur attendue", jeanEmbedDims, ""},
		{"largeur de l'ancien modele", 512, "dimensions"},
		{"largeur inconnue", jeanEmbedDims - 1, "dimensions"},
		{"vecteur vide", 0, "dimensions"},
	} {
		jeanEmbedTestPort = fakeEmbedRawServer(t, func(n int) [][]float64 {
			vs := make([][]float64, n)
			for i := range vs {
				vs[i] = make([]float64, tc.dims)
				for j := range vs[i] {
					vs[i][j] = 0.001 * float64(j+1)
				}
			}
			return vs
		})
		got, err := jeanEmbed(context.Background(), []string{"title: none | text: un souvenir"})
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%s : r\u00e9ponse rejet\u00e9e \u00e0 tort : %v", tc.name, err)
		case tc.want == "" && len(got) != 1:
			t.Errorf("%s : %d vecteurs au lieu de 1", tc.name, len(got))
		case tc.want != "" && err == nil:
			t.Errorf("%s : r\u00e9ponse accept\u00e9e, elle devait \u00eatre rejet\u00e9e", tc.name)
		case tc.want != "" && !strings.Contains(err.Error(), tc.want):
			t.Errorf("%s : erreur %q, attendu %q", tc.name, err.Error(), tc.want)
		}
	}
	jeanEmbedTestPort = 0
}

// TestEG2VecteurNonFini : NaN ou ±Infini vient d'un moteur en difficulté. Il
// contaminerait le produit scalaire de toutes les comparaisons suivantes, et
// la quantification int8 en ferait un nombre arbitraire.
func TestEG2VecteurNonFini(t *testing.T) {
	for _, mauvais := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		jeanEmbedTestPort = fakeEmbedRawServer(t, func(n int) [][]float64 {
			vs := make([][]float64, n)
			for i := range vs {
				vs[i] = make([]float64, jeanEmbedDims)
				for j := range vs[i] {
					vs[i][j] = 0.01
				}
				vs[i][3] = mauvais
			}
			return vs
		})
		if _, err := jeanEmbed(context.Background(), []string{"title: none | text: un souvenir"}); err == nil {
			t.Errorf("vecteur non fini accept\u00e9 (%v)", mauvais)
		}
	}
	jeanEmbedTestPort = 0
}

// TestEG2NombreDeVecteurs : un moteur qui n'en renvoie pas un par texte
// décalerait chaque vecteur sur la mauvaise entrée du journal.
func TestEG2NombreDeVecteurs(t *testing.T) {
	jeanEmbedTestPort = fakeEmbedRawServer(t, func(int) [][]float64 {
		v := make([]float64, jeanEmbedDims)
		for j := range v {
			v[j] = 0.01
		}
		return [][]float64{v} // un seul, quelle que soit la demande
	})
	if _, err := jeanEmbed(context.Background(), []string{"a", "b", "c"}); err == nil {
		t.Error("trois textes pour un vecteur accept\u00e9s")
	}
	jeanEmbedTestPort = 0
}
