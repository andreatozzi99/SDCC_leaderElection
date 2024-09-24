// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"time"
)

// Utilizzata per simulare un crash e validare gli algoritmi (Interrompe e riavvia il listener RPC)
func emulateCrash(node *Node, listener net.Listener) {
	// Un nodo che è in crash:
	// 1) SMETTE DI INVOCARE RPC SUGLI ALTRI NODI
	// 2) NON ACCETTA CHIAMATE RPC DA ALTRI NODI
	// Interrompere l'esposizione delle chiamate RPC (e riattivarla quando il nodo torna operativo)
	// Per stabilire quando interrompere o riattivare il servizio RPC utilizzo
	// un ciclo for che genera un numero random(compreso tra 0 e Numero definito come variabile globale)
	// ogni 5 secondi, se il numero generato è uguale a 1, il nodo esegue lo switch di stato (Crashed oppure Attivo)
	// Inizializza il generatore di numeri casuali con un seme univoco
	randSource := rand.NewSource(time.Now().UnixNano())
	randGen := rand.New(randSource)
	active := true // Variabile booleana per tenere traccia dello stato del nodo (true = attivo, false = in crash)
	for {
		time.Sleep(5 * time.Second) // Attendi 5 secondi prima di ogni iterazione
		// Genera un numero random tra 0 e 10
		randomNum := randGen.Intn(11)
		// Se il numero generato è 1, cambia lo stato del nodo da attivo a in crash o viceversa
		if randomNum == 1 {
			if active {
				leaderID = -1 // Cambio valori del leader perché non saranno più validi al momento del rientro
				leaderAddress = ""
				//interruptHeartBeatRoutine() // Interrompi heartBeatRoutine per evitare che il nodo comunichi con il Leader
				// Cambia lo stato del nodo da attivo a in crash
				active = false
				// Interrompi il listener RPC
				err := listener.Close()
				if err != nil {
				}
				fmt.Println(" \n-------------------- IL NODO E' IN CRASH --------------------\n ")

			} else {
				// Cambia lo stato del nodo da in crash ad attivo
				fmt.Println(" \n-------------------- IL NODO E' ATTIVO ---------------------\n ")
				active = true
				// Riavvia il listener RPC
				newListener, err := net.Listen("tcp", fmt.Sprintf(":%d", node.Port))
				if err != nil {
					fmt.Println("Errore durante la riapertura del listener RPC:", err)
					return
				}
				fmt.Printf("Nuovo listener RPC aperto sulla porta %d\n", node.Port)
				listener = newListener // Aggiorna il listener con quello appena creato
				// Avvia l'accettazione delle chiamate RPC sul nuovo listener
				go rpc.Accept(listener)

				// CONOSCO I NODI NELLA RETE E SONO REGISTRATO SULLA RETE, NON DEVO ESEGUIRE FUNZIONE DI START
				// NON CONOSCO IL NUOVO LEADER DELLA RETE -> Inizio un elezione
				if electionAlg == "Bully" {
					node.startBullyElection()
				}
				if electionAlg == "Raft" {
					//todo implementazione Raft
				}
			}
		}
	}
}

// Scrive su file JSON ID, IP,PORT
func saveState(id int, addr string, port int) error {
	// Crea una mappa con lo stato del nodo
	state := map[string]interface{}{
		"ID":        id,
		"IPAddress": addr,
		"Port":      port,
	}

	// Apre il file di log in modalità scrittura
	file, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Codifica lo stato del nodo in formato JSON e scrivilo nel file
	encoder := json.NewEncoder(file)
	if err := encoder.Encode(state); err != nil {
		return err
	}

	return nil
}

// Funzione per recuperare lo stato del nodo da un file JSON
func recoverState(path string) (int, string, int, error) {
	// Verifica se il file esiste e non è vuoto
	fileInfo, err := os.Stat(path)
	if err == nil { // Se il file esiste
		if fileInfo.Size() == 0 { // Se il file è vuoto
			return -1, "", 0, nil // Nessuno stato da recuperare, e nessun errore
		}
	} else {
		return -1, "", 0, err // Errore durante la verifica dell'esistenza del file
	}

	// Apre il file di log in modalità lettura
	file, err := os.Open(logFilePath)
	if err != nil {
		return -1, "", 0, err
	}
	defer file.Close()

	// Decodifica lo stato del nodo dal file JSON
	state := make(map[string]interface{})
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&state); err != nil {
		return -1, "", 0, err
	}

	// Estrae i valori dallo stato del nodo
	id := int(state["ID"].(float64))
	ipAddress := state["IPAddress"].(string)
	port := int(state["Port"].(float64))

	return id, ipAddress, port, nil
}

// Funzione ausiliaria per aggiungere il senderNode se non è presente nella nodeList
func addNodeInNodeList(senderNode Node) {
	found := false
	for _, node := range nodeList {
		if node.ID == senderNode.ID {
			// Il senderNode è già presente nella lista dei nodi
			// Aggiorna l'indirizzo IP e la porta del nodo
			node.IPAddress = senderNode.IPAddress
			node.Port = senderNode.Port
			found = true
			break
		}
	}
	// Se senderNode non è presente in nodeList
	if !found {
		// Aggiungi il senderNode alla lista dei nodi
		nodeList = append(nodeList, senderNode)
	}
}

// Funzione per trovare una porta disponibile casuale
func findAvailablePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			return
		}
	}(listener)
	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// Funzione per fare il bind su una porta specificata
func bindToSpecificPort(port int) (int, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return 0, err
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			return
		}
	}(listener)
	return port, nil
}

// Funzione per ottenere l'indirizzo IP locale
func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("non è stato possibile trovare l'indirizzo IP locale")
}
