Correctif du moteur ExLlama arrivé en 0.18.13.

## Correction

- **Modèle llama.cpp qui ne démarre plus après un modèle ExLlama** : un preset ExLlama enregistrait ses cartes dans le réglage de cartes de la machine, qui est conservé d'un modèle à l'autre. En revenant ensuite sur un modèle llama.cpp réglé pour un autre ordre de cartes, la répartition était inversée et le chargement échouait par manque de mémoire. Les cartes d'un preset ExLlama sont maintenant rangées à part, et le réglage de la machine n'est plus modifié. Un preset ExLlama enregistré avec la 0.18.13 est corrigé automatiquement à la bascule.

Une machine déjà touchée garde l'ordre de cartes laissé par le preset ExLlama : le remettre avec `ajean gpu <ordre>` (par exemple `ajean gpu 1,0`), ou dans l'éditeur du modèle llama.cpp concerné.

## Mise à jour

```
ajean update
```

Testé sur un serveur Linux à deux cartes : bascule d'un preset ExLlama vers un preset llama.cpp en tensor-split, puis retour.
