#!/bin/bash

pkill konsole
# Avvia il server di registro dei nodi
konsole -e "bash -c 'cd serverRegistry/ && go run node_registry_server.go; exec bash'" &

# Attendi finché il server di registrazione dei nodi non è completamente avviato
sleep 1

# Ottieni il numero di nodi da avviare come argomento da riga di comando
NUM_NODES=5

# Avvia i nodi in terminali separati
for ((i = 1; i <= NUM_NODES; i++)); do
    konsole -e "bash -c 'cd nodes/ && go run node.go; exec bash'" &
done
