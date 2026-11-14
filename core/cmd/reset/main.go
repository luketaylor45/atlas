package main

import (
	"flag"
	"log"

	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
)

// reset drops every Atlas table. It exists for development only and never
// touches game files: those live on the nodes and are removed by deleting
// servers through the panel.
func main() {
	confirm := flag.String("confirm", "", "must be exactly: delete-all-atlas-data")
	flag.Parse()

	if *confirm != "delete-all-atlas-data" {
		log.Fatal("Refusing to run. This permanently deletes every user, node, server record and egg.\n" +
			"Back up the database first, then re-run with: -confirm delete-all-atlas-data")
	}

	config.LoadConfig()
	database.Connect()

	tables := append([]interface{}{&database.SchemaMigration{}}, database.AllModels()...)
	// Dropped in reverse so tables that reference others go first.
	for i := len(tables) - 1; i >= 0; i-- {
		if err := database.DB.Migrator().DropTable(tables[i]); err != nil {
			log.Printf("Warning: failed to drop a table: %v", err)
		}
	}
	log.Println("All Atlas tables were dropped. Start Core to create a fresh database.")
}
