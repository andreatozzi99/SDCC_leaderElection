// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

import (
	"flag"
	"fmt"
	"strconv"
	"sync"
)

// ---------------- Variabili che rappresentano lo stato di un nodo ------------------
var (
	nodeList      = make([]NodeBully, 0) // Lista dei nodi di cui un nodo è a conoscenza
	leaderID      = -1                   // ID dell'attuale leader. Se < 0 => Leader sconosciuto
	leaderAddress = ""                   // Indirizzo dell'attuale leader+Porta, inizialmente sconosciuto
	stopped       = false                // True = Nodo fermato | False = Nodo in esecuzione
	election      = false                // True = Elezione in corso | False = Nessuna Elezione in corso
	electionMutex sync.Mutex             // Lucchetto per l'accesso alla variabile election
	hbState       = false                // True = HeartBeatRoutine in esecuzione | False = HeartBeatRoutine interrotta
	hbStateMutex  sync.Mutex             // Lucchetto per l'accesso alla variabile hbState
)

// #################### Crea il nodo adattato all'algoritmo selezionato ####################
// Esegue la fase di recovery se c'è bisogno
// Avvia il nodo
func main() {
	// Creazione del nodo o del RaftNode in base all'algoritmo di elezione
	// Definisci il flag della riga di comando
	var customValue string
	flag.StringVar(&customValue, "value", "", "ID opzionale del nodo che si vuole avviare (solo per test)")

	// Analizza i flag della riga di comando
	flag.Parse()

	ID := -1
	if customValue != "" {
		fmt.Println("ID fornito da riga di comando (solo per test):", customValue)
		var err error
		ID, err = strconv.Atoi(customValue)
		if err != nil {
			fmt.Println("Errore nella conversione in intero del valore da riga di comando:", err)
			ID = -1
		}
	}

	var node interface{}
	if electionAlg == "Bully" {
		node = &NodeBully{
			ID:        ID, // Per il nodeRegistry, che assegnerà un nuovo ID
			IPAddress: localAddress,
		}
	} else if electionAlg == "Raft" {
		node = &RaftNode{
			ID:          ID, // Per il nodeRegistry, che assegnerà un nuovo ID
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
	localIP := ""
	var id int
	var port int
	if runInContainer { // Fase di recovery ammissibile solo se il nodo è in un container
		var err error
		localIP, err = getLocalIP()
		if err != nil {
			fmt.Println("Errore durante l'ottenimento dell'indirizzo IP locale:", err)
			return
		}
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
	case *NodeBully:
		if failureDetected {
			n.ID = id // Contatterà il nodeRegistry e verrà riconosciuto
			n.Port = port
			n.IPAddress = localIP
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
			fmt.Println("Failure detected")
			n.ID = id //Contatterà il nodeRegistry e verrà riconosciuto
			n.Port = port
			n.IPAddress = localIP
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
