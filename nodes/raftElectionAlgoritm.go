package main

import (
	"fmt"
	"net/rpc"
	"strconv"
	"time"
)

// --------------- Descrizione Algoritmo --------------------
/* Quando è necessario eleggere un nuovo Leader, nel cluster viene avviato un nuovo mandato(Term).
// Un mandato è un periodo di tempo arbitrario sul nodo per il quale deve essere eletto un nuovo leader.
// Ogni mandato inizia con l'elezione di un leader.
// Se l'elezione viene completata con successo (cioè viene eletto un singolo leader), il mandato continua con le normali operazioni orchestrate dal nuovo leader.
// Se l'elezione è un fallimento, inizia un nuovo mandato, con una nuova elezione.
// L'elezione di un leader viene avviata da un nodo, nello stato "Candidato"
// Un nodo diventa un candidato se non riceve alcuna comunicazione dal leader per un periodo chiamato timeout elettorale,
// quindi presume che non ci sia più un leader in carica.
// Inizia l'elezione aumentando il contatore dei termini, votando per se stesso come nuovo leader e inviando un messaggio
// a tutti gli altri nodo richiedendo il loro voto. Un nodo voterà solo una volta per mandato, in base all'ordine di arrivo.
// Se un candidato riceve un messaggio da un altro nodo con un numero di termine superiore al termine corrente del candidato,
// l'elezione del candidato viene sconfitta e il candidato si trasforma in un follower e riconosce il leader come legittimo.
// Se un candidato riceve la maggioranza dei voti, diventa il nuovo leader.
// Se nessuna delle due cose accade, ad esempio a causa di un voto disgiunto, inizia un nuovo mandato e inizia una nuova elezione.
//
// Raft utilizza un timeout elettorale casuale per garantire che i problemi di voto diviso vengano risolti rapidamente.
// Questo dovrebbe ridurre la possibilità di un voto disgiunto perché i nodo non diventeranno candidati allo stesso tempo:
// un singolo nodo andrà in timeout, vincerà le elezioni,
// quindi diventerà leader e invierà messaggi heartbeat ad altri nodo prima che uno qualsiasi dei follower possa diventare candidato.
*/
// ---------------- Scheletro algoritmo -------------------
/*
Funzione InizioElezione():
	Se non ricevo comunicazioni dal leader per un tempo superiore al timeout:
		Diventa un candidato
		Incrementa il termine corrente
		Vota per te stesso
		Invia richieste di voto agli altri nodo
		Avvia un timeout di attesa per le risposte

Funzione RiceviMessaggioVoto(messaggio):
	Se il termine del messaggio è maggiore del termine corrente:
		Diventa follower
		Aggiorna il termine corrente
		Riconosci il leader come legittimo
		Interrompi l'elezione attuale
	Altrimenti:
		Ignora il messaggio

Funzione RiceviRispostaVoto(risposta):
	Se ricevo una risposta positiva dalla maggioranza dei nodi:
		Diventa il nuovo leader
		Interrompi l'elezione attuale
		Se scade il timeout di attesa:
			Se non ho ricevuto la maggioranza dei voti:
				Inizia una nuova elezione
			Interrompi l'elezione attuale
*/
// ----------------- Tempistiche -----------------------------
/* Tempistica e disponibilità
Il tempismo è fondamentale in Raft per eleggere e mantenere un leader stabile nel tempo,
al fine di avere una perfetta disponibilità del cluster.
La stabilità è garantita dal rispetto dei requisiti di temporizzazione dell'algoritmo:
	broadcastTime << electionTimeout << MTBF
broadcastTime è il tempo medio impiegato da un nodo per inviare una richiesta a tutti i nodo del cluster e ricevere le risposte.
È relativo all'infrastruttura utilizzata.
MTBF (Mean Time Between Failures) è il tempo medio tra i guasti per un nodo. È anche relativo all'infrastruttura.
electionTimeout è lo stesso descritto nella sezione Elezione del leader. È qualcosa che il programmatore deve scegliere.
I numeri tipici per questi valori possono essere compresi tra 0,5 ms e 20 ms per broadcastTime,
il che implica che il programmatore imposta electionTimeout tra 10 ms e 500 ms.
Possono essere necessarie diverse settimane o mesi tra un errore di un singolo nodo,
il che significa che i valori sono sufficienti per un cluster stabile.
*/

// ---------------- Strutture per Argomenti e valori di ritorno per Chiamate RPC ------------------

// HeartBeatArgs contiene gli argomenti per il messaggio di ?
type HeartBeatArgs struct {
	Term     int // Termine corrente del leader IMPORTANTE
	LeaderID int // ID del leader [DA 0 A N-1] con N = numero nodi nella rete
}

// HeartBeatReply contiene la risposta al messaggio di ?
type HeartBeatReply struct {
	Term    int  // Termine corrente del nodo ricevente
	Success bool // True se l'HeartBeat è stato accettato, false altrimenti
}

// RequestVoteArgs contiene gli argomenti per la richiesta di voto
type RequestVoteArgs struct {
	Term        int // Termine corrente del candidato
	CandidateID int // ID del candidato
}

// RequestVoteReply contiene la risposta alla richiesta di voto
type RequestVoteReply struct {
	Term        int  // Termine corrente del nodo ricevente
	VoteGranted bool // True se il voto è stato concesso, false altrimenti
}

// RaftNode ----------------------------
type RaftNode struct {
	ID              int
	IPAddress       string
	Port            int
	CurrentTerm     int
	VotedFor        int
	electionTimeout time.Duration
}

// --------------------- Metodi Esposti per RPC (Messaggi scambiabili tra nodi) ----------------------------

// REQUESTVOTE Gestisce la ricezione di un messaggio di Richiesta voto da parte di un altro nodo
func (n *RaftNode) REQUESTVOTE(args RequestVoteArgs, reply *RequestVoteReply) error {
	electionMutex.Lock()
	defer electionMutex.Unlock()

	// Se il termine del messaggio è maggiore del termine corrente
	// ? Dovrei verificare anche che VotedFor sia != da -1, non posso votare per più nodi nello stesso term
	if args.Term > n.CurrentTerm {
		n.becomeFollower(n.CurrentTerm) // Diventa follower
		n.CurrentTerm = args.Term
		n.VotedFor = -1 // Resetta il voto
		*reply = RequestVoteReply{
			Term:        n.CurrentTerm,
			VoteGranted: true, // Concede il voto
		}
	} else {
		// Ignora il messaggio
		*reply = RequestVoteReply{
			Term:        n.CurrentTerm,
			VoteGranted: false, // Non concede il voto
		}
	}
	return nil
}

// HEARTBEAT gestisce la ricezione di un messaggio di HeartBeat da parte di un altro nodo
func (n *RaftNode) HEARTBEAT(args HeartBeatArgs, reply *HeartBeatReply) error {
	electionMutex.Lock()
	defer electionMutex.Unlock()

	// Se il termine del messaggio è maggiore del termine corrente
	if args.Term > n.CurrentTerm {
		n.becomeFollower(args.Term) // Diventa follower e aggiorna il termine corrente
		resetElectionTimer()        // Resetta il timer di elezione
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

// Funzione InviaHeartBeat(): Invia un messaggio di HeartBeat agli altri nodi
func (n *RaftNode) sendHeartBeatRoutine() {
	for {
		for _, node := range nodeList {
			if node.ID != n.ID {
				// Invia il messaggio di HeartBeat in una goroutine
				go n.sendHeartBeatMessage(node)
			}
		}
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

// Funzione InizioElezione(): Avvia un'elezione se non ricevo comunicazioni dal leader per un tempo superiore al timeout
func (n *RaftNode) startRaftElection() {
	electionMutex.Lock()
	if election {
		electionMutex.Unlock()
		return // Se sono già in corso elezioni, termina
	}
	electionMutex.Unlock()
	// --------------- Avvia elezione ----------------
	election = true

	// Incrementa il termine corrente e vota per se stesso
	n.CurrentTerm++
	n.VotedFor = n.ID

	// Contatore per i voti ricevuti
	votesReceived := 1 // Voto per me stesso

	// --------- Invia richieste di voto agli altri nodi --------------
	for _, node := range nodeList {
		if node.ID != n.ID {
			// Invia la richiesta di voto in una goroutine
			go n.sendRequestVoteMessage(node, &votesReceived)
		}
	}

	// Avvia un timeout di attesa per le risposte
	time.Sleep(n.electionTimeout)
	if votesReceived <= len(nodeList)/2 {
		election = false // Se non ha ricevuto la maggioranza dei voti, avvia una nuova elezione
		n.startRaftElection()
	}
	// ------- Diventa Leader ----------
	n.becomeLeader()
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
		Term:        n.CurrentTerm,
		CandidateID: n.ID,
	}
	var reply RequestVoteReply
	err = client.Call("Node.REQUESTVOTE", args, &reply)
	if err != nil {
		fmt.Printf("Errore durante la richiesta di voto al nodo %d: %v\n", node.ID, err)
		return
	}
	// Aggiorna il termine corrente del nodo in base alla risposta ricevuta
	if reply.Term > n.CurrentTerm {
		n.becomeFollower(reply.Term)
	}

	// Controlla se il voto è stato concesso
	if reply.VoteGranted {
		// Incrementa il conteggio dei voti ricevuti
		*votesReceived++

		// Verifica se è stata ricevuta la maggioranza dei voti
		if *votesReceived > len(nodeList)/2 {
			// Diventa il nuovo leader
			n.becomeLeader()
		}
	}
}

// Funzione becomeFollower(): Trasforma il nodo in uno stato di Follower
func (n *RaftNode) becomeFollower(term int) {
	n.CurrentTerm = term
	n.VotedFor = -1
}

// Funzione becomeLeader(): Trasforma il nodo in uno stato di Leader
func (n *RaftNode) becomeLeader() {
	n.VotedFor = -1
	// Inizia a inviare messaggi di HeartBeat agli altri nodi
	n.sendHeartBeatRoutine()
}

// Funzione becomeCandidate(): Trasforma il nodo in uno stato di Candidato
func (n *RaftNode) becomeCandidate() {
	// Incrementa il termine corrente e vota per se stesso
	n.CurrentTerm++
	n.VotedFor = n.ID
	// Avvia un'elezione
	n.startRaftElection()
}

// Funzione resetElectionTimer(): Resetta il timer di elezione
func resetElectionTimer() {
	// TODO
}
