// Studente Andrea Tozzi, MATRICOLA: 0350270
package main

import (
	"fmt"
	"net/rpc"
	"strconv"
	"time"
)

// * -------- Implementazione metodi per la definizione dell'algoritmo Bully --------- *

// ELECTION Metodo invocato come rpc dagli altri nodi della rete
// In tal modo notificano la loro volontà di diventare Leader
// Se ID del senderNode è minore dell' ID locale, invio messaggio STOP e avvio nuova Elezione
func (n *Node) ELECTION(senderNode Node, reply *bool) error {
	addNodeInNodeList(senderNode)
	//########## modificare con verbose
	fmt.Printf("Nodo %d <-- ELECTION Nodo %d\n", n.ID, senderNode.ID)

	/*if n.ID == leaderID {// Se sono il leader, non devo avviare un elezione
	return nil}
	*/ //############## Eliminato dato che in seguito a eventi di crash e resume un nodo potrebbe cercare il leader anche se l'attuale leader è attivo

	// Devo interrompere la routine di HeartBeat, è preferibile non contattare il leader,
	// porterebbe alla scoperta del crash anche per questo nodo, con conseguente elezione
	// go interruptHeartBeatRoutine()
	leaderID = -1 // Per evitare che la routine di HeartBeat contatti il leader
	// ######### Può portare problemi perché un nodo potrebbe ricevere msg election dopo aver impostato correttamente i valori, e perdere il riferimento al leader
	leaderAddress = ""
	// ---------------- Se l'ID del chiamante è più basso di quello del nodo locale ----------------
	if senderNode.ID < n.ID {
		// ------------- Connessione con senderNode --------------
		client, err := rpc.Dial("tcp", fmt.Sprintf("%s:%d", senderNode.IPAddress, senderNode.Port))
		if err != nil {
			fmt.Println("Errore nella connessione con il nodo per metodo STOP :", senderNode.ID, ",", senderNode.IPAddress, ":", senderNode.Port)
			return err
		}
		defer func(client *rpc.Client) {
			err := client.Close()
			if err != nil {
			}
		}(client)
		// -------------- Invocazione metodo STOP sul senderNode ------------------------
		// non può diventare leader se il suo ID non è il massimo nella rete
		*reply = false
		err = client.Call("Node.STOP", n, &reply)
		if err != nil {
			fmt.Println("Errore durante la chiamata RPC per invocare il metodo 'STOP' sul nodo", senderNode.ID, err)
		}
		fmt.Printf("Nodo %d: STOP --> Nodo %d\n", n.ID, senderNode.ID)
		// -------------- Avvio Elezione ---------------------
		// #################: non devo avviare un elezione se sono già stato interrotto (STOP) una volta ################
		// Esempio: sono 2, crash nodo Leader 5, inizio elezione, 4 mi invia STOP, fermo elezione,
		// 			il nodo 1 mi invia ELECTION, inizio elezione=> NON VA BENE
		go n.startBullyElection()
	}
	// ------------ Se l' ID del chiamante è più alto dell' ID locale --------------------
	// Questo non dovrebbe succedere secondo l'algoritmo bully che invia messaggi di election solo a nodi con id maggiore
	*reply = true
	return nil
}

// COORDINATOR viene invocata da un nodo nella rete,
// per comunicare che è lui il NUOVO LEADER-> cambio valori di leaderID e leaderAddress.
func (n *Node) COORDINATOR(senderNode Node, reply *bool) error {
	// --------------------- Interrompi un eventuale elezione in corso --------------
	electionMutex.Lock()
	election = false // Indagato per bug: eletto nodo con id non massimo
	electionMutex.Unlock()
	//  ---------- Aggiungo senderNode nella nodeList in caso non sia già presente -------------
	// Notare che per un grande numero di nodi nella rete, eseguire questo controllo per ogni messaggio Coordinator, potrebbe generare problemi.
	go addNodeInNodeList(senderNode)

	// --------------------- Aggiorno il riferimento al Leader -----------------------
	leaderID = senderNode.ID
	leaderAddress = senderNode.IPAddress + ":" + strconv.Itoa(senderNode.Port)
	// ################### AGGIUNGERE VERBOSE ###################
	fmt.Printf("Nodo %d <-- COORDINATOR Nodo %d . Leader = %d \n", n.ID, senderNode.ID, senderNode.ID)

	// --------- Riattiva routine di HeartBeat ----------
	// n.startHeartBeatRoutine()
	// Restituisci true per indicare che il leader è stato aggiornato con successo
	*reply = true
	return nil
}

// STOP verrà invocata dai nodi della rete per fermare il nostro proposito di diventare LEADER
func (n *Node) STOP(senderNode Node, reply *bool) error {
	// Imposta la variabile election come FALSE per interrompere l'elezione in corso
	electionMutex.Lock()
	if !election {
		// Non c'è un elezione in corso, ma è stato invocato il metodo STOP -> Print per analizzare il sistema
		fmt.Printf("Nodo %d <-- STOP Nodo %d.\n", n.ID, senderNode.ID)
		electionMutex.Unlock()
		*reply = true
		return nil
	}
	election = false
	electionMutex.Unlock()
	fmt.Printf("Nodo %d <-- STOP Nodo %d. Elezione interrotta.\n", n.ID, senderNode.ID)
	*reply = true
	return nil
}

// Metodo per avviare un processo di elezione, invocato dalla routine di
// invio messaggio ELECTION solo sui nodi nella rete con ID superiore all' ID locale
func (n *Node) startBullyElection() {
	// ---------------- Verifico se c'è già un elezione in atto --------------
	electionMutex.Lock()
	if election { // C'è già un elezione in corso, non devo avviare un altro processo di elezione
		electionMutex.Unlock()
		return
	} else { // Non c'era un elezione in corso, si procede con l'elezione
		election = true
		leaderID = -1
		leaderAddress = ""
	}
	electionMutex.Unlock()
	// --------- Invia messaggi ELECTION a nodi con ID maggiore dell'ID locale ----------
	fmt.Println("Avvio un processo di elezione")
	i := 0
	for _, node := range nodeList {
		if node.ID > n.ID {
			// Avvia goroutine per inviare messaggio ELECTION al nodo con ID maggiore
			go n.sendElectionMessage(node)
			i++
		}
	}
	// ---------- Avvia un timer per attendere messaggi di tipo STOP ------------
	if i != 0 {
		time.Sleep(time.Duration(1) * time.Second)
		// ---------- Se l'elezione è stata interrotta, termina la funzione -------------
		electionMutex.Lock()
		if !election {
			electionMutex.Unlock() // Il print non serve
			fmt.Println("L'elezione è stata interrotta, non posso diventare il Leader")
			return
		}
		electionMutex.Unlock()
	}
	// ----------- SONO IL LEADER ------------
	fmt.Printf("\nNodo %d SONO LEADER. COORDINATOR --> tutta la rete.\n", n.ID)
	leaderID = n.ID
	leaderAddress = n.IPAddress + ":" + strconv.Itoa(n.Port)
	go n.sendCoordinatorMessages()
	//go n.startHeartBeatRoutine()
	// ---------------- Imposto election come false, elezione terminata --------------
	electionMutex.Lock()
	election = false // PROVA: SPOSTATO SOTTO LA PRINT, INVECE CHE PRIMA
	electionMutex.Unlock()
}

// Invoca procedura remota ELECTION sul nodo inserito come parametro
// @return: true se posso continuare elezione, false per interrompere processo di elezione
func (n *Node) sendElectionMessage(node Node) bool {
	// Effettua una chiamata RPC al nodo specificato per avviare un processo di elezione
	client, err := rpc.Dial("tcp", fmt.Sprintf("%s:%d", node.IPAddress, node.Port))
	if err != nil {
		fmt.Println("Errore nella chiamata ELECTION sul nodo", node.ID, ",non è raggiungibile", node.IPAddress, ":", node.Port, "ERRORE:", err)
		return false
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			return
		}
	}(client)

	var reply = true
	err = client.Call("Node.ELECTION", n, &reply)
	if err != nil {
		fmt.Println("Errore durante la chiamata RPC per avviare l'elezione sul nodo, non è raggiungibile", node.ID)
		return true
	}
	return true
}

// Invoca la procedura remota COORDINATOR su tutti i nodi nella nodeList per notificare il nuovo Leader
func (n *Node) sendCoordinatorMessages() {
	// Itera su tutti i nodi nella nodeList
	for _, node := range nodeList {
		if node.ID != n.ID { // Se il nodo nella è diverso da me stesso
			// Avvia una goroutine per invocare la procedura remota COORDINATOR sul nodo
			go func(node Node) {
				// Effettua una chiamata RPC per invocare la procedura remota COORDINATOR
				client, err := rpc.Dial("tcp", fmt.Sprintf("%s:%d", node.IPAddress, node.Port))
				if err != nil {
					fmt.Printf("Errore durante la connessione al nodo %d: %v\n", node.ID, err)
					return
				}
				defer func(client *rpc.Client) {
					err := client.Close()
					if err != nil {
						fmt.Printf("Errore durante la chiusura della connessione al nodo %d: %v\n", node.ID, err)
					}
				}(client)
				// Variabile per memorizzare la risposta remota
				var reply bool
				// Effettua la chiamata RPC
				err = client.Call("Node.COORDINATOR", n, &reply)
				if err != nil {
					fmt.Printf("Errore durante la chiamata RPC COORDINATOR al nodo %d: %v\n", node.ID, err)
					return
				}
				// Stampa un messaggio di conferma
				fmt.Printf("Chiamata RPC COORDINATOR al nodo %d completata con successo\n", node.ID)
			}(node)
		}
	}
}
