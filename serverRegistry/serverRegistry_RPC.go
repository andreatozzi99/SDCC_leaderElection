// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

// NodeRegistryInterface Interfaccia per le chiamate RPC del serverRegistry
type NodeRegistryInterface interface {
	RegisterNode(node Node, reply *int) error
	GetRegisteredNodes(_, reply *map[string]Node) error
}
