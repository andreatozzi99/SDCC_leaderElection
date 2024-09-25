// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

import "fmt"

// #################### Crea il nodo adattato all'algoritmo selezionato ####################
// Esegue la fase di recovery se c'è bisogno
// Avvia il nodo
func main() {
	// Creazione del nodo o del RaftNode in base all'algoritmo di elezione
	var node interface{}
	if electionAlg == "Bully" {
		node = &Node{
			ID:        -1, // Per il nodeRegistry, che assegnerà un nuovo ID
			IPAddress: localAddress,
		}
	} else if electionAlg == "Raft" {
		node = &RaftNode{
			ID:          -1, // Per il nodeRegistry, che assegnerà un nuovo ID
			IPAddress:   localAddress,
			CurrentTerm: 0,
			VotedFor:    -1,
		}
	} else {
		fmt.Println("Algoritmo di elezione non supportato:", electionAlg)
		return
	}
	fmt.Println("Algoritmo di elezione selezionato:", electionAlg)

	// #################### Verifica stato di recovery ####################
	failureDetected := false
	var id int
	var port int
	if runInContainer { // Fase di recovery ammissibile solo se il nodo è in un container
		var err error
		id, _, port, err = recoverState(logFilePath)
		if err != nil {
			fmt.Println("Errore durante il recupero dello stato:", err)
			return
		}
		if id != -1 { // Solo se è stato recuperato uno stato
			failureDetected = true
			bindToSpecificPort(port)
		}
	}
	// Verifica il tipo di nodo e imposta la porta
	switch n := node.(type) {
	case *Node:
		if failureDetected {
			n.ID = id // Contatterà il nodeRegistry e verrà riconosciuto
			n.Port = port
		} else {
			var err error
			n.Port, err = findAvailablePort()
			if err != nil {
				fmt.Println("Errore nella ricerca di una porta:", err)
				return
			}
		}
		n.start() // Start di un nodo con algoritmo Bully
	case *RaftNode:
		if failureDetected {
			n.ID = id //Contatterà il nodeRegistry e verrà riconosciuto
			n.Port = port
		} else {
			var err error
			n.Port, err = findAvailablePort()
			if err != nil {
				fmt.Println("Errore nella ricerca di una porta:", err)
				return
			}
		}
		n.start() // Start di un nodo Raft
	}
	// Posso iniziare l'elezione in base al tipo di nodo
	select {} // Mantiene il programma in esecuzione
}
