@echo off

REM Avvia il server di registro dei nodi
start "" cmd /k "go run serverRegistry\node_registry_server.go"

REM Attendi finché il server di registro dei nodi non è completamente avviato
    :WAIT_FOR_REGISTRY
timeout /t 1 /nobreak >nul
tasklist | find /i "node_registry_server.exe" >nul
if errorlevel 1 goto :WAIT_FOR_REGISTRY

REM Ottieni il numero di nodi da avviare come argomento da riga di comando
set NUM_NODES=%1

REM Verifica che sia stato fornito un argomento valido per il numero di nodi
if "%NUM_NODES%"=="" (
    echo Numero di nodi non specificato.
    exit /b
)

REM Avvia i nodi in terminali separati
for /l %%i in (1, 1, %NUM_NODES%) do (
    start "" cmd /k "go run nodes\node.go"
)

exit
