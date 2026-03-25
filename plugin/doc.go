// Package plugin provides a bob DBInfoPlugin that generates a complete
// Go REST API using kiln's generators.
//
// Instead of running kiln as a standalone CLI, this plugin integrates
// directly into bob's generation pipeline. Bob reads the schema, kiln
// generates the API layer d- one command, one process.
//
// # Usage
//
// Create a gen/main.go in your project (based on bobgen-psql/main.go):
//
//	package main
//
//	import (
//		"context"
//		"log"
//		"os"
//		"os/signal"
//
//		"github.com/stephenafamo/bob/gen"
//		"github.com/stephenafamo/bob/gen/bobgen-psql/driver"
//		"github.com/stephenafamo/bob/gen/plugins"
//		kilnplugin "github.com/fisayoafolayan/kiln/plugin"
//	)
//
//	func main() {
//		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
//		defer cancel()
//
//		// Bob config
//		driverConfig := driver.Config{DSN: os.Getenv("DATABASE_URL")}
//		config := gen.Config{} // or load from file
//
//		// Bob's default plugins (models, factory, etc.)
//		bobPlugins := plugins.Setup[any, any, driver.IndexExtra](
//			plugins.Config{}, gen.PSQLTemplates,
//		)
//
//		// Kiln plugin - generates API layer
//		kiln := kilnplugin.New[any, any, driver.IndexExtra](kilnplugin.Options{
//			ConfigPath: "kiln.yaml",
//		})
//
//		state := &gen.State[any]{Config: config}
//		allPlugins := append(bobPlugins, kiln)
//
//		if err := gen.Run(ctx, state, driver.New(driverConfig), allPlugins...); err != nil {
//			log.Fatal(err)
//		}
//	}
//
// Then run:
//
//	DATABASE_URL="postgres://..." go run gen/main.go
//
// This generates bob models + kiln API layer in a single pass.
package plugin
