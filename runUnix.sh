#!/bin/bash

# Numero di nodi da avviare
NUM_NODES=2

pkill konsole

# Avvia il server di registro dei nodi in un nuovo terminale Console
konsole -e "bash -c 'cd serverRegistry/ && go run .; exec bash'" &

# Attendi finché il server di registrazione dei nodi non è completamente avviato
sleep 1

# Avvia gli altri nodi uno alla volta in nuovi terminali Console
for ((i = 1; i <= NUM_NODES; i++)); do
    konsole -e "bash -c 'cd nodes/ && go run .; exec bash'" &
done
