package main

// --------------- Descrizione Algoritmo --------------------
// Quando è necessario eleggere un nuovo Leader, nel cluster viene avviato un nuovo mandato(Term).
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

// ---------------------- Scheletro algoritmo -------------------
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

// ---------------- AppendEntry Args and Reply ------------------

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
