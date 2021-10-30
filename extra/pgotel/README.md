# OpenTelemetry instrumentation for go-pg

## Installation

```bash
go get github.com/yz89122/pgorm/extra/pgotel/v10
```

## Usage

Tracing is enabled by adding a query hook:

```go
import (
	"github.com/yz89122/pgorm/v10"
	"github.com/yz89122/pgorm/extra/pgotel/v10"
)

db := pg.Connect(&pg.Options{...})

db.AddQueryHook(pgotel.NewTracingHook())
```

See [documentation](https://pg.uptrace.dev/tracing/) for more details.
