Installation d'ExLlama depuis l'interface, binaires CUDA officiels sous Linux et affichage du chat plus fluide.

## Corrections

- **ExLlama : l'installation échouait depuis l'interface** avec « write /dev/stderr: Descripteur non valide » à l'étape de création de l'environnement Python, alors que `ajean exl3 install` fonctionnait dans un terminal. Sans console (application lancée en fenêtre ou en service), la copie du journal vers le terminal échouait et interrompait toute l'installation. Cette copie ignore maintenant l'absence de console ; le journal de l'interface reçoit toujours tout. Merci pour le rapport (#177).

## Moteur llama.cpp

- **Linux avec carte NVIDIA : binaires CUDA officiels.** llama.cpp publie désormais des builds CUDA pour Linux. L'installation rapide les utilise (avec les bibliothèques CUDA fournies) au lieu de la version Vulkan, plus lente. La version CUDA la plus récente acceptée par le pilote est choisie. La compilation locale n'est plus conseillée par défaut dans ce cas.
- Compatibilité vérifiée avec la dernière version de llama.cpp (b11936) : options du serveur et routes utilisées par AJEAN inchangées. Les anciennes options `--mlock` / `--no-mmap` des presets restent converties automatiquement en `--load-mode`.

## Interface

- **Défilement du chat plus doux** pendant la génération : la vue suit le texte en glissant au lieu d'avancer par saccades.
- **Texte qui s'écrit sans clignoter** : seuls les passages qui changent sont redessinés, les mots apparaissent en fondu, et le Markdown incomplet (tableau, titre, gras en cours) n'est affiché qu'une fois sa forme connue.
- **iPhone : les boutons répondent pendant une réponse.** Le fil ne bouge plus tant qu'un doigt touche l'écran, ce qui faisait annuler les appuis par Safari.

## Mise à jour

```
ajean update
```

Testé par les tests automatiques. Les binaires CUDA Linux n'ont pas encore été installés sur une vraie machine Linux avec carte NVIDIA, et le correctif ExLlama n'a pas été rejoué sur un Windows lancé sans console.
