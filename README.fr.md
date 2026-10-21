# AJEAN

[English](README.md) · **Français**

![Interface web d'AJEAN](docs/ui.png)

> **Projet dérivé, usage personnel.** Ce dépôt est le **fork** de
> [**nathaninline/ajean**](https://github.com/nathaninline/ajean), créé et maintenu par
> **nathaninline** — le projet principal, et la seule source des versions publiées d'AJEAN.
> Ce qui suit décrit AJEAN **en général**. Ce fork-ci porte une **ligne de correctifs
> privée** (**0.18.4.2 Cresius**) répondant aux besoins d'une machine précise : moteur MoE
> **Strata** sur AMD, recherche par embeddings, correctifs Crawl4AI. Ces ajouts ne sont
> **pas** des versions de l'amont, ne sont **pas** testés ailleurs et ne sont **pas**
> garantis transposables. Pour installer AJEAN, prenez les versions publiées sur
> [nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).
> Le moteur MoE utilisé ici provient de [Niko1221/Strata](https://github.com/Niko1221/Strata).
> Détail de ce que ce dépôt modifie : [NOTICE.md](NOTICE.md).

**Votre propre IA, chez vous.** AJEAN est un programme unique qui transforme un ordinateur équipé d'une carte graphique (ou d'un simple processeur) en assistant IA complet : une interface de discussion, une mémoire, des outils, l'accès au web, des tâches planifiées et l'accès depuis votre téléphone, sans que rien ne quitte votre machine.

AJEAN s'occupe de tout ce qui entoure le modèle. Le modèle lui-même tourne sur [llama.cpp](https://github.com/ggml-org/llama.cpp), qu'AJEAN installe et tient à jour pour vous.

- **Un seul fichier, aucune dépendance.** Ni Docker, ni Python, ni environnement à installer. Linux, Windows et macOS.
- **Un assistant, pas seulement un modèle.** Il se souvient, range le travail en projets, exécute des commandes, lit le web, pilote un navigateur et travaille seul selon un planning.
- **Vos données restent chez vous.** Tout tourne sur votre machine. La mémoire et les conversations peuvent être chiffrées sur le disque, et l'accès à distance est chiffré de bout en bout.
- **Utilisable de partout.** Les mêmes conversations sur l'ordinateur et le téléphone, en direct.
- **Se branche sur vos outils.** Un point d'accès compatible OpenAI pour n'importe quelle application tierce.

---
