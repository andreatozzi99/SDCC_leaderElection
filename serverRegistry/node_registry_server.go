package main

import (
	"fmt"
	"net"
	"net/rpc"
	"sync"
)

// NodeRegistry Struttura per il registro dei nodi
type NodeRegistry struct {
	nodes  map[int]Node
	lastID int
	mutex  sync.Mutex
}

// RegisterNode Metodo per registrare un nodo nel registro
func (nr *NodeRegistry) RegisterNode(node Node, reply *int) error {
	nr.mutex.Lock()
	defer nr.mutex.Unlock()

	// Incrementa l'ID solo se il nodo non ha già un ID assegnato
	if node.ID == 0 {
		nr.lastID++         // Incrementa l'ID
		node.ID = nr.lastID // Assegna il nuovo ID al nodo
	}

	nr.nodes[node.ID] = node
	*reply = node.ID
	fmt.Printf("Node %d registered successfully\n", node.ID)
	return nil
}

// GetRegisteredNodes Metodo per ottenere l'elenco dei nodi registrati
func (nr *NodeRegistry) GetRegisteredNodes(_, reply *map[int]Node) error {
	nr.mutex.Lock()
	defer nr.mutex.Unlock()
	*reply = nr.nodes
	return nil
}

// ######## Attualmente sulla porta 8080, AGGIUNGERE FILE CONFIGURAZIONE ######
func main() {
	// Inizializzazione del registro dei nodi
	registry := &NodeRegistry{
		nodes: make(map[int]Node),
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
	fmt.Println("Server di registrazione nodi in esecuzione su localhost:8080")
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Errore nell'accettare la connessione:", err)
			continue
		}
		// Gestione delle connessioni in nuovo thread
		go server.ServeConn(conn)
	}
}
