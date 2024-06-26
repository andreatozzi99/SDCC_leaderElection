package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"strconv"
	"time"
)

// ---------------- Strutture: Argomenti e valori di ritorno delle Chiamate RPC ------------------

// HeartBeatArgs contiene gli argomenti per il messaggio di HEARTBEAT inviato dal Leader verso i follower
type HeartBeatArgs struct {
	Term     int // Termine corrente del leader IMPORTANTE
	LeaderID int // ID del leader [DA 0 A N-1] con N = numero nodi nella rete
}

// HeartBeatReply contiene la risposta al messaggio di HEARTBEAT
type HeartBeatReply struct {
	Term    int  // Termine corrente del nodo ricevente
	Success bool // True se l'HeartBeat è stato accettato, false altrimenti
}

// RequestVoteArgs contiene gli argomenti per la richiesta di voto
type RequestVoteArgs struct {
	Node RaftNode
}

// RequestVoteReply contiene la risposta alla richiesta di voto
type RequestVoteReply struct {
	Term        int  // Termine corrente del nodo ricevente
	VoteGranted bool // True se il voto è stato concesso, false altrimenti
}

// RaftNode ----------------------------
type RaftNode struct {
	ID        int
	IPAddress string
	Port      int
	//------------- Aggiunte rispetto un classico Node
	CurrentTerm int
	VotedFor    int
}

// --------------------- Metodi Esposti per RPC (Messaggi scambiati tra nodi) ----------------------------

// REQUESTVOTE gestisce la ricezione di un messaggio di Richiesta voto da parte di un altro nodo
func (n *RaftNode) REQUESTVOTE(args RequestVoteArgs, reply *RequestVoteReply) error {
	// Aggiungo il nodo alla lista dei nodi conosciuti
	senderNode := Node{
		ID:        args.Node.ID,
		IPAddress: args.Node.IPAddress,
		Port:      args.Node.Port,
	}
	go addNodeInNodeList(senderNode)
	fmt.Printf("Node %d (T:%d) <-- REQUESTVOTE from Node %d (T:%d)\n", n.ID, n.CurrentTerm, args.Node.ID, args.Node.CurrentTerm)

	if args.Node.CurrentTerm > n.CurrentTerm { // Se il termine del messaggio è maggiore del termine corrente del nodo locale
		n.becomeFollower(args.Node.CurrentTerm) // Aggiorna il termine corrente e diventa follower
	} else {
		if n.VotedFor != -1 {
			// Se ha già votato per un altro candidato, rifiuta la richiesta di voto
			*reply = RequestVoteReply{
				Term:        n.CurrentTerm,
				VoteGranted: false, // Non concede il voto
			}
			fmt.Println("Vote: No")
			return nil
		}
	}
	n.VotedFor = args.Node.ID
	*reply = RequestVoteReply{
		Term:        n.CurrentTerm,
		VoteGranted: true, // Concede il voto
	}
	fmt.Println("Vote: Yes")
	return nil
}

// HEARTBEAT gestisce la ricezione di un messaggio di HeartBeat da parte di un altro nodo
func (n *RaftNode) HEARTBEAT(args HeartBeatArgs, reply *HeartBeatReply) error {
	fmt.Printf("Node %d <-- HEARTBEAT from Leader %d\n", n.ID, args.LeaderID)
	n.resetElectionTimer() // Resetta il timer di elezione
	// Se il termine del messaggio è maggiore del termine corrente
	if args.Term > n.CurrentTerm {
		n.becomeFollower(args.Term) // Diventa follower e aggiorna il termine corrente
		*reply = HeartBeatReply{
			Term:    n.CurrentTerm,
			Success: true, // Accetta l'HeartBeat
		}
	} else {
		// Ignora il messaggio se il termine del messaggio è minore o uguale al termine corrente
		*reply = HeartBeatReply{
			Term:    n.CurrentTerm,
			Success: false, // Rifiuta l'HeartBeat
		}
	}
	return nil
}

//--------------------------------------------------------------------------------------------------------

// Metodo per avviare il nodo e registrarne l'indirizzo nel server di registrazione
// e recuperare la lista dei nodi in rete
func (n *RaftNode) start() {
	// ------------- Esposizione dei metodi per le chiamate RPC -------------
	err := rpc.Register(n)
	if err != nil {
		fmt.Println("Errore durante la registrazione dei metodi RPC:", err)
		return
	}
	// ------ Creazione listener RPC ------
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", n.Port))
	if err != nil {
		fmt.Println("Errore durante la creazione del listener RPC:", err)
		return
	}
	// -------- Accetta le connessioni per RPC in arrivo in una goroutine ------------
	fmt.Printf("Nodo in ascolto su porta %d per le chiamate RPC...\n", n.Port)
	go rpc.Accept(listener)
	// ------------- Ingresso nella rete tramite Node Registry ------------------
	client, err := rpc.Dial("tcp", serverAddressAndPort)
	if err != nil {
		fmt.Println("Errore nella connessione al NodeRegistry server:", err)
		return
	}
	// ---- Chiusura connessione con defer ----
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			fmt.Println("Errore nella chiusura della connessione con il NodeRegistry server:", err)
			return
		}
	}(client)
	// ---------- RPC per Registrare il nodo --------------
	var reply int
	// ? Devo creare una struttura node, impostare i parametri a partire da quelli di n, e inviare quelli al NodeRegistry
	node := &Node{
		ID:        -1, // Per il nodeRegistry, che assegnerà un nuovo ID
		IPAddress: localAddress,
		Port:      n.Port,
	}
	err = client.Call("NodeRegistry.RegisterNode", node, &reply)
	if err != nil {
		fmt.Println("Errore durante la registrazione del nodo:", err)
		return
	}
	if reply == -1 {
		fmt.Println("Il server di registrazione ha rifiutato la registrazione del nodo")
		return
	}
	// Imposto il mio nuovo ID (restituito dalla chiamata rpc al nodeRegistry)
	n.ID = reply
	// Visualizza l'ID e la porta assegnati al nodo
	fmt.Printf("-------- Registrato sulla rete con ID:%d, Porta:%d --------\n", n.ID, n.Port)

	// ---------- RPC per ricevere la lista dei nodi nella rete  ----------
	err = client.Call("NodeRegistry.GetRegisteredNodes", n, &nodeList)
	if err != nil {
		fmt.Println("Errore durante la richiesta della lista dei nodi:", err)
		return
	}
	// ------------ Divento un candidato, per scoprire la situazione della rete, non conosco il term attuale nella rete,
	// Gli altri nodi non sanno della mia esistenza, devo almeno contattarli ------------------------------------------
	go n.becomeCandidate()
	return
}

// Funzione InizioElezione(): Avvia un'elezione se non ricevo comunicazioni dal leader per un tempo superiore al timeout
func (n *RaftNode) startRaftElection() {
	fmt.Printf("Nodo %d Started RAFT-election\n", n.ID)
	electionMutex.Lock()
	if election {
		electionMutex.Unlock()
		return // Se sono già in corso elezioni, termina
	}
	// --------------- Avvia elezione ----------------
	election = true
	electionMutex.Unlock()
	// Incrementa il termine corrente e vota per se stesso
	n.CurrentTerm++
	n.VotedFor = n.ID
	votesReceived := 1 // Contatore per i voti ricevuti
	// --------- Invia richieste di voto agli altri nodi --------------
	for _, node := range nodeList {
		if node.ID != n.ID {
			// Invia la richiesta di voto in una goroutine
			go n.sendRequestVoteMessage(node, &votesReceived)
		}
	}
	// Avvia un timeout di attesa per le risposte
	time.Sleep(electionTimeout)
	electionMutex.Lock()
	if election == false { // Elezione interrotta durante il periodo di timeout
		electionMutex.Unlock()
		return // Il nodo è diventato follower in seguito all'invio di uno dei messaggi RequestVote
	}
	if votesReceived < len(nodeList)/2 {
		election = false // Se non ha ricevuto la maggioranza dei voti, avvia una nuova elezione
		electionMutex.Unlock()
		n.startRaftElection()
		return
	}
	// ------- Diventa Leader ----------
	electionMutex.Unlock()
	n.becomeLeader()
}

// Funzione InviaHeartBeat(): Invia un messaggio di HeartBeat agli altri nodi
func (n *RaftNode) sendHeartBeatRoutine() {
	fmt.Printf("Leader %d | Start Heart-Beat Routine\n", n.ID)
	for {
		if !hbState {
			return
		}
		fmt.Printf("Leader %d | Contacting all followers...\n", n.ID)
		for _, node := range nodeList {
			if node.ID != n.ID {
				// Invia il messaggio di HeartBeat in una goroutine
				go n.sendHeartBeatMessage(node)
			}
		}
		time.Sleep(time.Millisecond * 1000)
	}
}

func (n *RaftNode) sendHeartBeatMessage(node Node) {
	// Connessione al nodo remoto
	client, err := rpc.Dial("tcp", node.IPAddress+":"+strconv.Itoa(node.Port))
	if err != nil {
		fmt.Printf("Errore durante la connessione al nodo %d: %v\n", node.ID, err)
		return
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
			// Gestione dell'errore nella chiusura del client
		}
	}(client)
	// Effettua la chiamata RPC per inviare l'HeartBeat
	args := HeartBeatArgs{
		Term:     n.CurrentTerm,
		LeaderID: n.ID,
	}
	var reply HeartBeatReply
	err = client.Call("RaftNode.HEARTBEAT", args, &reply)
	if err != nil {
		fmt.Printf("Errore durante l'invio di HeartBeat al nodo %d: %v\n", node.ID, err)
		return
	}
	// Aggiorna il termine corrente del nodo in base alla risposta ricevuta
	if reply.Term > n.CurrentTerm {
		// Se il termine nella risposta è maggiore del termine corrente,
		// aggiorniamo il termine corrente del nodo, ma non diventiamo follower
		n.CurrentTerm = reply.Term
	}
}

func (n *RaftNode) sendRequestVoteMessage(node Node, votesReceived *int) {
	// ---------------- Connessione al nodo remoto -----------------
	client, err := rpc.Dial("tcp", node.IPAddress+":"+strconv.Itoa(node.Port))
	if err != nil {
		fmt.Printf("Errore durante la connessione al nodo %d: %v\n", node.ID, err)
		return
	}
	defer func(client *rpc.Client) {
		err := client.Close()
		if err != nil {
		}
	}(client)
	// ------------------- Effettua la chiamata RPC per richiedere il voto -----------------
	args := RequestVoteArgs{
		Node: *n,
	}
	var reply RequestVoteReply
	err = client.Call("RaftNode.REQUESTVOTE", args, &reply)
	if err != nil {
		fmt.Printf("Errore durante la richiesta di voto al nodo %d: %v\n", node.ID, err)
		return
	}
	// Aggiorna il termine corrente del nodo in base alla risposta ricevuta
	//if reply.Term > n.CurrentTerm {
	//	n.becomeFollower(reply.Term)
	//	return
	//}
	// Controlla se il voto è stato concesso
	if reply.VoteGranted {
		// Incrementa il conteggio dei voti ricevuti
		*votesReceived++
	}
}

// Funzione becomeFollower(): Avvia la routine di un nodo follower
func (n *RaftNode) becomeFollower(term int) {
	fmt.Printf("Node %d I'm a follower \n", n.ID)
	hbState = false
	electionMutex.Lock()
	election = false
	electionMutex.Unlock()
	n.CurrentTerm = term
	n.VotedFor = -1
}

// Funzione becomeLeader(): Avvia la routine di un nodo leader
func (n *RaftNode) becomeLeader() {
	fmt.Printf("Node %d I'm LEADER \n", n.ID)
	electionMutex.Lock()
	election = false
	electionMutex.Unlock()
	n.VotedFor = -1
	hbState = true
	// Inizia a inviare messaggi di HeartBeat agli altri nodi
	go n.sendHeartBeatRoutine()
}

// Funzione becomeCandidate(): Avvia la routine di un nodo candidato
func (n *RaftNode) becomeCandidate() {
	fmt.Printf("Node %d I'm a Candidate \n", n.ID)
	// Avvia un'elezione
	n.startRaftElection()
}

var (
	electionTimer *time.Timer // Timer per il conteggio dell'elezione
	randGen       = rand.New(rand.NewSource(time.Now().UnixNano()))
)

func (n *RaftNode) resetElectionTimer() {
	// Resetta il timer di elezione se esiste già uno attivo
	if electionTimer != nil {
		electionTimer.Stop()
	}

	// Genera un numero casuale compreso tra electionTimerMin e electionTimerMax
	randomDuration := electionTimerMin + time.Duration(randGen.Int63n(int64(electionTimerMax-electionTimerMin)))

	// Crea un nuovo timer con la durata casuale generata
	electionTimer = time.NewTimer(randomDuration)

	// Avvia una goroutine per attendere il timer e avviare un'elezione quando scade
	go func() {
		<-electionTimer.C
		fmt.Println("Election timer expired")
		n.startRaftElection()
	}()

	// Stampa il tempo rimanente prima che scada il timer (per scopi di debug)
	fmt.Printf("Election timer set to %v\n", randomDuration)
}
