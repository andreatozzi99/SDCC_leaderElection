package main

// Node Struttura per rappresentare un nodo
type Node struct {
	ID        int
	IPAddress string
	Port      int
}

// NodeRegistryInterface Interfaccia per le chiamate RPC del serverRegistry
type NodeRegistryInterface interface {
	RegisterNode(node Node, reply *int) error
	GetRegisteredNodes(_, reply *map[string]Node) error
}
