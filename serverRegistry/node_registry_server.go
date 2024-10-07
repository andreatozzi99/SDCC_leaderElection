// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

import (
	"fmt"
	"net"
	"net/rpc"
	"sync"
)

var (
	manualTopology  = false    // Se true, la topologia della rete è definita manualmente
	adjacencyMatrix = [][]int{ // Matrice di adiacenza per la topologia manuale
		{1, 1, 1, 1, 0, 0}, // Nodo 1 conosce: 1, 2, 3, 4
		{1, 1, 1, 1, 0, 0}, // Nodo 2 conosce: 1, 2, 3, 4
		{1, 1, 1, 1, 0, 0}, // Nodo 3 conosce: 1, 2, 3, 4
		{1, 1, 1, 1, 1, 1}, // Nodo 4 conosce: 1, 2, 3, 4, 5, 6
		{0, 0, 0, 1, 1, 1}, // Nodo 5 conosce: 4, 5, 6
		{0, 0, 0, 1, 1, 1}, // Nodo 6 conosce: 4, 5, 6
		// ! La dimensione della matrice deve corrispondere al numero di nodi avviati !
	}
)

type Node struct {
	ID        int
	IPAddress string
	Port      int
}

// NodeRegistry Struttura per il registro dei nodi
type NodeRegistry struct {
	nodes  []Node
	lastID int
	mutex  sync.Mutex
}

// RegisterNode Metodo per registrare un nodo nella rete se non è già presente
// Funzione esportata per essere utilizzata come servizio RPC
func (nr *NodeRegistry) RegisterNode(node Node, reply *int) error {
	nr.mutex.Lock() // Blocca l'accesso alla lista dei nodi registrati per evitare conflitti
	defer nr.mutex.Unlock()

	// Incrementa l'ID solo se il nodo non ha già un ID assegnato (node.ID = -1)
	if node.ID == -1 {
		nr.lastID++         // Incrementa l'ID
		node.ID = nr.lastID // Assegna il nuovo ID al nodo
	}
	if node.ID > 0 {
		// Controlla se il nodo è già registrato
		for _, n := range nr.nodes {
			if n.ID == node.ID {
				fmt.Printf("\nNodo: %d già registrato (Recovery) \n", node.ID)
				// aggiorno i suoi valori
				n.IPAddress = node.IPAddress
				n.Port = node.Port
				*reply = node.ID
				return nil
			}
		}
	}

	// Aggiunge il nodo alla lista dei nodi registrati
	nr.nodes = append(nr.nodes, node) // Aggiunge il nodo alla lista dei nodi registrati
	*reply = node.ID                  // Il valore di reply è l'ID del nodo che si sta registrando
	fmt.Printf("\nNodo: %d Registrato correttamente \n", node.ID)
	go nr.printNodeList()
	return nil
}

// GetRegisteredNodes Metodo per ottenere l'elenco dei nodi registrati nella rete
// Funzione esportata per essere utilizzata come servizio RPC
func (nr *NodeRegistry) GetRegisteredNodes(input Node, reply *[]Node) error {
	if manualTopology {
		return nr.getKnownNodes(input, reply)
	}
	// Se la richiesta arriva da un nodo con ID > 0, allora è un nodo già registrato
	if input.ID > 0 {
		nr.mutex.Lock()
		defer nr.mutex.Unlock()
		*reply = nr.nodes // Restituisce l'elenco dei nodi registrati
		return nil
	}
	// Se la richiesta arriva da un nodo con ID = 0 o -1, allora è un nuovo nodo
	// TODO Decidere cosa fare in questo caso, attualmente non genera problemi, è solo una sicurezza
	return nil
}

// GetKnownNodes Restituisce l'insieme dei nodi conosciuti da un dato nodo secondo la topologia manuale
func (nr *NodeRegistry) getKnownNodes(input Node, reply *[]Node) error {
	nr.mutex.Lock()
	defer nr.mutex.Unlock()

	// Assicurati che l'ID del nodo sia valido e che ci siano nodi registrati
	if input.ID > 0 && input.ID <= len(adjacencyMatrix) && len(nr.nodes) >= len(adjacencyMatrix[input.ID-1]) && manualTopology {
		// Estraggo i nodi conosciuti dal nodo corrente secondo la matrice di adiacenza
		knownNodes := []Node{}
		for i, knows := range adjacencyMatrix[input.ID-1] {
			if knows == 1 && i != input.ID-1 && i < len(nr.nodes) { // Verifica che l'indice sia valido
				knownNodes = append(knownNodes, nr.nodes[i])
			}
		}
		*reply = knownNodes
		return nil
	}

	// Se manualTopology è false o l'ID è fuori dai limiti, restituisci tutti i nodi
	*reply = nr.nodes
	return nil
}

// ######## TODO Attualmente sulla porta 8080, AGGIUNGERE FILE CONFIGURAZIONE ######
func main() {
	// Inizializzazione del registro dei nodi
	registry := &NodeRegistry{
		nodes:  make([]Node, 0), // Inizializza con slice vuota
		lastID: 0,
	}

	// Creazione di un nuovo server RPC
	server := rpc.NewServer()

	// Registrazione del servizio RPC
	err := server.Register(registry)
	if err != nil {
		fmt.Println("Errore nella registrazione del servizio RPC")
		return
	}

	// Creazione del listener per le richieste RPC
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Errore nella creazione del listener:", err)
		return
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
			return
		}
	}(listener)

	// Avvio del server RPC
	fmt.Println("Server di registrazione dei nodi in esecuzione su localhost:8080")
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Errore nell'accettare la connessione:", err)
			continue
		}
		// Gestione delle connessioni in un nuovo thread
		go server.ServeConn(conn)
	}
}

// Stampa dei nodi nella rete
func (nr *NodeRegistry) printNodeList() {
	fmt.Print("\nI nodi registrati sono:")
	for _, node := range nr.nodes {
		fmt.Printf("ID: %d, IP: %s, Port: %d\n", node.ID, node.IPAddress, node.Port)
	}
	// esegui il flush del buffer di output
	fmt.Println()
}
