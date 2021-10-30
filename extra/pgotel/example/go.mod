module example

go 1.16

replace github.com/yz89122/pgorm/v12 => ../../..

replace github.com/yz89122/pgorm/extra/pgotel/v12 => ../

require (
	github.com/yz89122/pgorm/extra/pgotel/v12 v12.0.0
	github.com/yz89122/pgorm/v12 v12.0.0
	go.opentelemetry.io/otel v1.0.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.0.0
	go.opentelemetry.io/otel/sdk v1.0.0
)
