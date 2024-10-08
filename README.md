# Leader Election Algorithms

## Eseguire in locale (rete completamente connessa) - Bully e Raft

Per eseguire il progetto in locale, seguire i passaggi elencati:

1. **Modifica il file `config.go` nella cartella `nodes` per impostare i parametri desiderati.:**
    - Scegli l'algoritmo di elezione (Bully o Raft).
    - Scegli se simulare il crash automatico di un nodo tramite la variabile `emulateLocalCrash`.

2. **Configurare il numero di nodi:**
    - Modifica il campo `NUM_NODES` nel file `runUnix.sh` situato nella directory `startInLocalHost` per impostare il numero di nodi desiderati.

3. **Eseguire lo script:**
    - Dalla directory principale del progetto, esegui il file `runUnix.sh` situato nella cartella `startInLocalHost` con il comando:
      ```sh
      ./startInLocalHost/runUnix.sh
      ```

4. **Aggiungere un nuovo nodo o riattivare un nodo:**
    - In qualsiasi momento, puoi aggiungere un nuovo nodo alla rete avviata eseguendo il comando dalla directory principale del progetto:
      ```sh
      cd nodes && go run .
      ```
    - Per riattivare un nodo precedentemente arrestato, esegui il comando:
      ```sh
      cd nodes && go run . -value <ID>
      ```
      Sostituisci `<ID>` con l'ID del nodo che vuoi riattivare. (Solo per fase test/debug)


## Eseguire in locale una rete parzialmente connessa - Raft

Per eseguire il progetto con una rete parzialmente connessa, seguire i passaggi elencati:

1. **Modifica il file `node_registry_server.go` nella cartella `serverRegistry` per impostare:**
    - Il parametro `manualTopology` a `true`.
    - Definisci la topologia desiderata impostando la matrice di adiacenza `adjacencyMatrix`.
    - Assicurati che la dimensione della matrice(quadrata) di adiacenza sia uguale al numero di nodi avviati.