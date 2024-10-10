package main

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
