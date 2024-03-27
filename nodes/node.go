package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"strconv"
	"time"
)

// Assunzioni preliminari:
// Siamo in un sistema distribuito SINCRONO: conosciamo il tempo massimo di trasmissione.
// Timeout da attendere senza nessuna risposta prima di diventare Leader in seguito a un processo di ELEZIONE

// ------------- Variabili che rappresentano lo stato (variabile) di un nodo --------------
var nodeList []Node    // Lista dei nodi di cui un nodo è a conoscenza
var leaderID = -1      // ID dell'attuale leader
var leaderAddress = "" // Indirizzo dell'attuale leader+Porta, inizialmente sconosciuto
var election = false   // True = Elezione in corso | False = Nessuna Elezione in corso
//var verbose = true // Prendere da file di configurazione per attivare/disattivare commenti aggiuntivi
// ----------------------------------------------------------------------------------------

// Node Struttura per rappresentare un nodo
type Node struct {
	ID        int
	IPAddress string
	Port      int
}

// ELECTION Metodo invocato come rpc dagli altri nodi della rete
// In tal modo notificano la loro volontà di diventare Leader
func (n *Node) ELECTION(senderNode Node, reply *bool) error {
	if true { //########## modificare con verbose
		fmt.Println("Ricevuto messaggio ELECTION da:", senderNode.ID)
	}

	// Se l'ID del chiamante è più basso di quello del nodo corrente, invoca il metodo "STOP" del chiamante
	if senderNode.ID < n.ID {
		// Connessione
		client, err := rpc.Dial("tcp", fmt.Sprintf("%s:%d", n.IPAddress, n.Port))
		if err != nil {
			fmt.Println("Errore nella connessione con il nodo :", senderNode.ID, ",", senderNode.IPAddress, ":", senderNode.Port)
			return err
		}
		defer func(client *rpc.Client) {
			err := client.Close()
			if err != nil {
				return
			}
		}(client)

		// Invocazione metodo STOP sul nodo, dato che ID sender < ID locale
		*reply = false
		err = client.Call("Node.Stop", 0, &reply)
		if err != nil {
			fmt.Println("Errore durante la chiamata RPC per invocare il metodo 'STOP' sul nodo", senderNode.ID)
			return err
		}
	}
	// Non fare nulla se l' ID del chiamante è più alto dell' ID locale
	*reply = true
	return nil
}

// COORDINATOR viene invocata da un nodo nella rete,
// per comunicare che è lui il NUOVO LEADER
func (n *Node) COORDINATOR(senderNode Node, reply *bool) error {

	// Cerca il senderNode nella lista dei nodi. Per eliminare eventuali problemi nella rete.
	// Notare che per un grande numero di nodi nella rete, eseguire questo controllo per ogni messaggio
	// Coordinator, potrebbe generare problemi.
	found := false
	for _, node := range nodeList {
		if node.ID == senderNode.ID {
			// Il senderNode è già presente nella lista dei nodi
			found = true
			break
		}
	}
	// Se senderNode non era presente nella lista dei nodi
	if !found {
		// Aggiungi il senderNode alla lista dei nodi
		nodeList = append(nodeList, senderNode)
	}
	// --------------------- Aggiorno Leader -----------------------

	// Imposta i parametri del senderNode come parametri locali del leader
	leaderID = senderNode.ID
	leaderAddress = senderNode.IPAddress + ":" + strconv.Itoa(senderNode.Port)
	// ################### AGGIUNGERE VERBOSE #############à
	fmt.Printf("Ho ricevuto COORDINATOR da parte del nodo -->%d, è lui il nuovo leader \n", senderNode.ID)
	// Restituisci true per indicare che il leader è stato aggiornato con successo
	*reply = true

	return nil
}

// STOP verrà invocata dai nodi della rete per fermare il nostro proposito di diventare LEADER
func (n *Node) STOP(senderNode Node, reply *bool) {
	// TODO
	// Devo fare qualcosa per bloccare la funzione "startElection"
	// che sta inviando ancora messaggi o attendendo il timeout prima di autoproclamarsi LEADER
}

// HEARTBEAT Metodo per contattare il nodo Leader e verificare se esse è ancora attivo
func (n *Node) HEARTBEAT(senderID int, reply *bool) error {
	*reply = true // Se la funzione viene avviata, vuol dire che il nodo è funzionante
	// ################### AGGIUNGERE VERBOSE #############à
	fmt.Printf("LEADER %d :Msg di HEARTBEAT da parte di -->%d\n", n.ID, senderID)
	return nil
}

func main() {
	// Creazione struttura nodo
	node := &Node{
		ID:        -1, // cosi il nodeRegistry assegna il giusto ID
		IPAddress: "localhost",
	}
	// Genera una porta casuale disponibile per il nodo
	node.Port = findAvailablePort()
	if node.Port == 0 {
		fmt.Println("Errore nella ricerca di una porta")
	}

	// Avvio nodo (Ingresso nella rete tramite nodeRegistry ed esposizione dei servizi remoti)
	node.start()

	// Mantiene il programma in esecuzione
	select {}
}

// Metodo per avviare il nodo e registrarne l'indirizzo nel server di registrazione
// e recuperare la lista dei nodi in rete
func (n *Node) start() {
	// ------------- Avvio goRoutine per simulare il crash ------------------
	go emulateCrash()

	// ------------- Esposizione dei metodi per le chiamate RPC -------------
	err := rpc.Register(n)
	if err != nil {
		return
	}

	// Crea un listener RPC
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", n.Port))
	if err != nil {
		fmt.Println("Errore durante la creazione del listener RPC:", err)
		return
	}
	defer func(listener net.Listener) {
		err := listener.Close()
		if err != nil {
		}
	}(listener)

	// Accetta le connessioni RPC in arrivo
	fmt.Printf("Nodo in ascolto su porta %d per le chiamate RPC...\n", n.Port)
	rpc.Accept(listener)
	// ------------------------------------

	// ------------- Connessione al server di registrazione -------------------
	client, err := rpc.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("Errore nella connessione al server di registrazione:", err)
		return
	}
	// Chiusura connessione a fine di questa funzione
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			return
		}
	}(client)

	// Chiamata RPC per registrare il nodo nel server di registrazione
	var reply bool
	// ############ Nome servizio su file CONF ###################
	err = client.Call("NodeRegistry.RegisterNode", n, &reply)
	if err != nil {
		fmt.Println("Errore durante la registrazione del nodo:", err)
		return
	}
	if !reply {
		fmt.Println("Il server di registrazione ha rifiutato la registrazione del nodo")
		return
	}

	// Visualizza l'ID e la porta assegnati al nodo
	fmt.Printf("Regisrazione sulla rete avvenuta. ID %d: Assigned port %d\n", n.ID, n.Port)

	// Chiamata RPC per richiedere la lista dei nodi al server di registrazione
	// ############ Nome servizio su file CONF ###################
	err = client.Call("NodeRegistry.GetNodeList", 0, &nodeList)
	if err != nil {
		fmt.Println("Errore durante la richiesta della lista dei nodi:", err)
		return
	}
	// Funzione ausiliaria per stampa del vettore dei nodi restituita dal nodeRegistry
	printNodeList()

	// Contro: Più messaggi sulla rete, ma non richiede ulteriore funzioni e si sfrutta l'algoritmo di elezione
	n.startElection() // Non è possibile eseguirla in una goRoutine, le operazioni sottostanti dipendono dall'indirizzo del Leader

	if leaderID == -1 {
		_ = fmt.Errorf("Node:%d ERRORE, nessun leader riscontrato a seguito di una Elezione\n", n.ID)
	}
	if n.ID != leaderID {
		// Avvia la goroutine per heartbeat
		go n.heartBeatRoutine()
		return
	}
	// Se il leader sono io, non devo andare in heartBeatRoutine
	return
}

// Meccanismo di Failure Detection:
// Contattare il nodo leader periodicamente (Invocazione del metodo remoto: HEARTBEAT)
func (n *Node) heartBeatRoutine() {
	// Questa routine dovrebbe essere interrotta durante il processo di Elezione di un nuovo leader(?)
	if leaderID != -1 {
		// Non dovrebbe essere stata avviata questa funzione se non c'è un leader
		fmt.Printf("Nodo:%d Errore! Sono nella heartBeatRoutine, ma non ho un leader definito\n", n.ID)
		return
	}
	for {
		// Verifica se il nodo leader è raggiungibile
		if !n.isLeaderAlive() {
			// se il leader non è raggiungibile, ne deve essere eletto un altro
			go n.startElection()

			// la heartBeatRoutine deve essere interrotta fino a che non ci sarà un nuovo leader
			return
		}

		// Attendi un intervallo casuale prima di effettuare un nuovo heartbeat
		// ######### Selezione intervallo su file di configurazione ##########
		randomSeconds := rand.Intn(10-1) + 1
		interval := time.Duration(randomSeconds) * time.Second
		time.Sleep(interval)
	}
}

// Metodo per contattare il nodo leader tramite la chiamata rpc HEARTBEAT esposta dai nodi
// @return: True se il leader risponde, False altrimenti.
func (n *Node) isLeaderAlive() bool {

	// Se il nodo stesso è il leader, esci
	if n.ID == leaderID {
		return true
	}

	// Una nuova connessione con il nodo leader per ogni ciclo di heartbeat?
	client, err := rpc.Dial("tcp", leaderAddress)
	if err != nil {
		// Se non è possibile contattare il leader, avvia un processo di elezione
		fmt.Println("Node", n.ID, "failed to contact leader Node", leaderID, ":", leaderAddress)
		return false
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			return
		}
	}(client)

	// Chiamata RPC HEARTBEAT per il meccanismo di failure detection al nodo leader
	var reply bool
	err = client.Call("Node.HEARTBEAT", n.ID, &reply)
	if err != nil {
		// Se il leader non risponde, avvia un processo di elezione
		fmt.Println("Node", n.ID, "failed to contact leader Node", leaderID)
		return false
	}
	//chiamata andata a buon fine
	return true
}

// Metodo per avviare un processo di elezione,
// e invocare la chiamata rpc ELECTION solo sui nodi nella rete con ID superiore all' ID locale
func (n *Node) startElection() {

	// Canale per ricevere i risultati delle goroutine
	resultCh := make(chan bool, len(nodeList)-1)

	for _, node := range nodeList {
		if node.ID > n.ID {
			// Avvia goroutine per contattare il nodo con ID maggiore dell'ID locale
			go func(node Node) {
				resultCh <- n.sendElectionMessage(node)
			}(node)
		}
	}

	// Controlla i risultati dalle goroutine
	for range nodeList[1:] {
		if !<-resultCh {
			// Se anche solo uno dei risultati è falso = STOP, devo abbandonare il proposito di diventare Leader
			return
		}
	}
	// Inviati tutt i messaggi ELECTION
	// dovrei assicurarmi di non aver ricevuto una rpc STOP
	// --------------- Timer prima dell'auto elezione -----------------
	// Attualmente non serve perché sto utilizzando i valori di funzione

	// Sono io il leader, devo inviare messaggio di COORDINATOR a tutti i nodi nella rete
	dfnrngs
}

// Invoca procedura remota ELECTION su node(secondo parametro),
// @return: true se posso continuare elezione, false per interrompere processo di elezione
func (n *Node) sendElectionMessage(node Node) bool {
	// Effettua una chiamata RPC al nodo specificato per avviare un processo di elezione
	client, err := rpc.Dial("tcp", fmt.Sprintf("%s:%d", n.IPAddress, n.Port))
	if err != nil {
		fmt.Println("Errore nella chiamata RPC ELECTION sul nodo, non è raggiungibile", node.IPAddress, ":", node.Port)
		return true
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			return
		}
	}(client)

	var reply = true
	err = client.Call("Node.ELECTION", n.ID, &reply)
	if err != nil {
		fmt.Println("Errore durante la chiamata RPC per avviare l'elezione sul nodo, non è raggiungibile", node.ID)
		return true
	}

	// Utilizzo per il momento il valore di ritorno della procedura ELECTION
	if !reply {
		// Il nodo con ID superiore mi ha bloccato, abbandono il proposito di diventare leader
		return false
	}
	return true
}

// Funzione per trovare una porta disponibile casuale
func findAvailablePort() int {
	// Cerca una porta disponibile partendo da una porta casuale
	startPort := 30000
	for port := startPort; port < 65535; port++ {
		_, err := net.Listen("tcp", ":"+strconv.Itoa(port))
		if err == nil {
			return port
		}
	}
	return 0 // Ritorna 0 se non viene trovata alcuna porta disponibile
}

func emulateCrash() {
	//TODO
}

// Stampa dei nodi nella rete
func printNodeList() {
	fmt.Print("I nodi sulla rete sono:")
	for _, node := range nodeList {
		fmt.Printf("\nID: %d, IP: %s, Port: %d", node.ID, node.IPAddress, node.Port)
	}
}
