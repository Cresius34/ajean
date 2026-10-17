Applications créées par l'IA, et une API Anthropic pour brancher Claude Code sur n'importe quel modèle.

## Applications

- **Nouvel écran Applications** (icône dans le menu latéral) : en mode agent, il suffit de demander « Crée-moi une app qui… ». L'IA écrit une petite application (HTML, CSS, JavaScript) qui s'ouvre en plein écran dans AJEAN, en local comme sur app.ajean.link.
- **Les données passent par des scripts** que l'IA écrit et teste pour chaque app (Python ou autre interpréteur présent sur la machine). L'app les appelle, garde sa propre mémoire et ses propres fichiers, et peut demander une réponse au modèle actif.
- **Bac à sable** : une app n'a aucun accès au réseau, ni à la page d'AJEAN, ni aux autres apps. Elle ne voit que ses propres scripts et ses propres données. Ouvrir un lien externe demande une confirmation qui affiche l'adresse complète.
- **Versions** : chaque modification crée une nouvelle version, et une version précédente peut être rouverte depuis le menu de l'app. Une erreur dans une app s'affiche en haut de l'écran, avec un bouton pour demander la correction à l'IA.
- Une carte « Ouvrir » apparaît dans la conversation après chaque création ou modification.

## API Anthropic (Claude Code)

- **AJEAN répond aussi au format Anthropic** (`/v1/messages`), en plus du format OpenAI. Claude Code se branche directement sur le modèle actif :

  ```
  ANTHROPIC_BASE_URL=http://<machine>:8090 ANTHROPIC_AUTH_TOKEN=<clé API de la machine> claude
  ```

  L'adresse publique `https://<machine>.oai.ajean.link` fonctionne de la même façon quand l'accès OpenAI public est activé.
- **Plus besoin de retoucher le chat template du modèle.** Les requêtes de Claude Code étaient refusées par beaucoup de templates d'origine (prompt système en plusieurs blocs, résultats d'outils dans les messages utilisateur, réflexion rejouée). AJEAN remet désormais la conversation au format attendu avant de l'envoyer au moteur : un seul message système en tête, contenus aplatis, résultats d'outils à leur place.
- Fonctionne avec tous les presets : llama.cpp, moteur MoE, API externe et GPU Cloud. Les appels d'outils, la réflexion et le flux en direct sont traduits. Le nom de modèle demandé par Claude Code est ignoré : c'est le preset actif qui répond.
- La clé API de la machine est exigée (en-tête `x-api-key` ou `Authorization`). Sans clé configurée, seuls les appels depuis la machine elle-même sont acceptés.

## Assistant

- **Outils regroupés par domaine** (mémoire, web, navigateur, tâches, fichiers…) : le modèle voit un outil par domaine avec une action à choisir, au lieu d'une longue liste. Le prompt système est plus court, ce qui allège la lecture de chaque nouveau message.

## Mise à jour

```
ajean update
```

Non testé à ce jour : Claude Code via l'adresse publique `oai.ajean.link` (testé en réseau local seulement) et les Applications sous Windows et macOS (testées sous Linux).
