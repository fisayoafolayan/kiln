package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/stephenafamo/bob/gen"
	"github.com/stephenafamo/bob/gen/bobgen-psql/driver"
	"github.com/stephenafamo/bob/gen/plugins"

	kilnplugin "github.com/fisayoafolayan/kiln/plugin"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is required")
	}

	driverCfg := driver.Config{}
	driverCfg.Dsn = dsn

	pluginsCfg := plugins.Config{}
	pluginsCfg.Models.Destination = "./models"
	pluginsCfg.Models.Pkgname = "models"
	bobPlugins := plugins.Setup[any, any, driver.IndexExtra](
		pluginsCfg, gen.PSQLTemplates,
	)

	kiln := kilnplugin.New[any, any, driver.IndexExtra](kilnplugin.Options{
		ConfigPath: "kiln.yaml",
	})

	state := &gen.State[any]{Config: gen.Config[any]{}}
	allPlugins := append(bobPlugins, kiln)

	if err := gen.Run(ctx, state, driver.New(driverCfg), allPlugins...); err != nil {
		log.Fatal(err)
	}
}
