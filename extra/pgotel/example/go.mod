module example

go 1.16

replace github.com/yz89122/pgorm/v10 => ../../..

replace github.com/yz89122/pgorm/extra/pgotel/v10 => ../

require (
	github.com/yz89122/pgorm/extra/pgotel/v10 v10.10.6
	github.com/yz89122/pgorm/v10 v10.10.6
	go.opentelemetry.io/otel v1.0.0
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.0.0
	go.opentelemetry.io/otel/sdk v1.0.0
)
