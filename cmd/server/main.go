// The HTTP/API process is intentionally out of scope for issue #4. This
// process exercises the persistence startup contract and keeps the database
// available for the API layer that will be added in a later issue.
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/efauncodes/equipment-manager-backend/db"
)

func main() {
	database, err := db.Open("")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	log.Printf("sqlite database ready at %s", db.ConfiguredPath())

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdown)
	<-shutdown
}
