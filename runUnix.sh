#!/bin/bash

# Numero di nodi da avviare
NUM_NODES=5

# Avvia il server di registro dei nodi in un nuovo terminale Konsole
konsole -e "bash -c 'cd serverRegistry/ && go run node_registry_server.go; exec bash'" &

# Attendi finché il server di registrazione dei nodi non è completamente avviato
sleep 1

# Avvia il primo nodo in un nuovo terminale Konsole
konsole -e "bash -c 'cd nodes/ && go run node.go; exec bash'" &

# Attendi un po' di tempo prima di avviare gli altri nodi
sleep 1

# Avvia gli altri nodi uno alla volta in nuovi terminali Konsole
for ((i = 2; i <= NUM_NODES; i++)); do
    konsole -e "bash -c 'cd nodes/ && go run node.go; exec bash'" &
    sleep 1
done
