# Bob Plugin Example

This example shows how to use kiln as a [bob](https://github.com/stephenafamo/bob) plugin instead of the standalone CLI.

## When to use this

Most users should use `kiln generate` (the standalone CLI). Use the plugin approach when:

- You already use bob directly in your project
- You want to customize bob's generation pipeline
- You want a single `go run` command instead of installing kiln

## How it works

Instead of running `kiln generate`, you create a small `gen/main.go` that runs bob with kiln loaded as a plugin:

```
go run gen/main.go
```

Bob reads the schema and generates models. Kiln's plugin receives the parsed schema and generates the API layer. One command, one process.

## Using this in your own project

This example uses `replace` directives in `go.mod` to point at the local kiln
source. In your own project, remove the `replace` lines and use real versions:

```bash
go get github.com/fisayoafolayan/kiln/plugin@latest
go get github.com/stephenafamo/bob@latest
```

## Setup (running this example locally)

```bash
cd examples/bob-plugin
cp .env.example .env
docker compose up -d

# Wait for Postgres
until docker compose exec postgres pg_isready -U app -d app -q 2>/dev/null; do sleep 1; done

# Load env and generate everything
set -a && . ./.env && set +a
go run gen/main.go
go mod tidy

# Run the server
go run cmd/server/main.go
```

## Project Structure

```
bob-plugin/
  gen/main.go          <- runs bob + kiln plugin (this is the key file)
  cmd/server/main.go   <- generated server entry point (write-once)
  kiln.yaml            <- kiln config
  schema.sql           <- database schema
  docker-compose.yml   <- Postgres container
  .env                 <- DATABASE_URL
```

## The Key File: gen/main.go

```go
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

    // Bob driver config
    driverCfg := driver.Config{}
    driverCfg.Dsn = dsn

    // Bob's default plugins (models, dbinfo, etc.)
    pluginsCfg := plugins.Config{}
    pluginsCfg.Models.Destination = "./models"
    pluginsCfg.Models.Pkgname = "models"
    bobPlugins := plugins.Setup[any, any, driver.IndexExtra](
        pluginsCfg, gen.PSQLTemplates,
    )

    // Kiln plugin - generates the API layer
    kiln := kilnplugin.New[any, any, driver.IndexExtra](kilnplugin.Options{
        ConfigPath: "kiln.yaml",
    })

    // Run bob + kiln together
    state := &gen.State[any]{Config: gen.Config[any]{}}
    allPlugins := append(bobPlugins, kiln)

    if err := gen.Run(ctx, state, driver.New(driverCfg), allPlugins...); err != nil {
        log.Fatal(err)
    }
}
```

This is equivalent to running `kiln generate` but gives you full control over bob's pipeline.
