Outils plus robustes avec les modèles locaux : appels réparés, recherche web qui ne s'arrête plus à un moteur, lecture ciblée des longues pages.

## Nouveautés

- **Recherche web avec moteurs de secours** : quand DuckDuckGo bloque (il le fait après quelques requêtes rapprochées) ou répond trop peu, la recherche continue sur Brave, Yahoo et Wikipédia, puis Bing, sans clé ni service à installer. Chaque moteur garde son propre rythme et reste mis de côté un moment après un refus. Bing est écarté quand ses résultats ignorent la requête, ce qu'il fait après trop de requêtes. Le résultat indique quels moteurs ont répondu.
- **Lecture ciblée d'une longue page** : `web_open` accepte un paramètre `focus` (par exemple « horaires d'ouverture ») et renvoie seulement les passages de la page qui y répondent, avec leurs numéros de ligne pour lire autour. Un petit modèle n'a plus à feuilleter menus et actualités pour trouver l'information en bas de page.

## Corrections

- **Appels d'outils presque valides refusés** : un argument JSON avec une virgule en trop, des guillemets simples, un bloc Markdown autour ou une accolade manquante faisait refuser l'appel, et le modèle réécrivait souvent la même erreur. Ces erreurs sont maintenant réparées et l'appel s'exécute. Un appel coupé à la limite de longueur reste refusé, pour ne jamais écrire un fichier à moitié.
- **Édition de fichier qui échouait sur des espaces** : un passage recopié avec une autre indentation (espaces au lieu de tabulations, espace final perdu) n'était pas trouvé. Il est maintenant retrouvé quand il est unique, et le nouveau texte prend l'indentation du fichier. Quand rien ne correspond, l'erreur montre le passage le plus proche à recopier.
- **Sortie des commandes illisible ou incomplète** : les codes couleur et les barres de progression (npm, pip) remplissaient le résultat, et seule la fin d'une longue sortie était gardée. La sortie est maintenant nettoyée, garde son début et sa fin, et la sortie complète est enregistrée dans un fichier temporaire que l'IA peut consulter au lieu de relancer la commande.

## Mise à jour

```
ajean update
```
