#!/bin/bash

# Numero di nodi da avviare
NUM_NODES=4

# Dimensioni delle finestre
WINDOW_WIDTH=400
WINDOW_HEIGHT=600

# Posizione iniziale delle finestre
START_X=0
START_Y=0

# Offset per la posizione delle finestre
OFFSET_X=400
OFFSET_Y=0

pkill konsole

# Avvia il server di registro dei nodi in un nuovo terminale Console
konsole --geometry ${WINDOW_WIDTH}x${WINDOW_HEIGHT}+${START_X}+${START_Y} -e "bash -c 'cd serverRegistry/ && go run .; exec bash'" &

# Attendi finché il server di registrazione dei nodi non è completamente avviato
sleep 1

# Avvia gli altri nodi uno alla volta in nuovi terminali Console
for ((i = 1; i <= NUM_NODES; i++)); do
    # Calcola la posizione per la finestra corrente
    POS_X=$((START_X + i * OFFSET_X))
    POS_Y=$((START_Y + i * OFFSET_Y))

    konsole --geometry ${WINDOW_WIDTH}x${WINDOW_HEIGHT}+${POS_X}+${POS_Y} -e "bash -c 'cd nodes/ && go run .; exec bash'" &
done