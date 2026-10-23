Le navigateur piloté par l'IA sait remplir les formulaires des sites modernes, et s'arrête quand un site le bloque.

## Corrections

- **Choix impossibles à cocher sur certains sites** : sur une boutique en ligne, les options d'un produit (couleur, capacité…) n'apparaissaient pas dans la liste des éléments, et un clic dessus restait sans effet. Le bouton suivant restait grisé, et l'IA recommençait sans fin. Ces cases cachées derrière une étiquette sont maintenant listées et cliquées à travers l'étiquette, et le cochage est vérifié.
- **États visibles** : la liste des éléments indique ce qui est coché, sélectionné ou désactivé. Un clic sur un bouton désactivé est refusé, avec l'explication qu'un choix obligatoire manque.
- **Lecture du texte de la page** : la recherche dans la page trouve aussi le texte simple (une date de livraison, un prix, un message), et plus seulement les boutons et les liens.
- **Fin de tâche plus nette** : quand un site répond par une page d'erreur ou de blocage après une action normale, l'IA en est avertie et passe à une autre piste au lieu d'insister. Une même action répétée sans effet est signalée, et un rappel de conclure arrive au bout de 25 actions.
- **Mode Jean, juste après l'envoi d'un message** : après une mise à jour ou un rafraîchissement de la page, la scène affichait quelques secondes « Clique sur moi pour me parler » au lieu de « Lit le message… ».

## Mise à jour

```
ajean update
```

Testé sur un serveur Linux avec un modèle local, sur une vraie boutique en ligne.
