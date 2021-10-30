module github.com/yz89122/pgorm/extra/pgotel/v12

go 1.15

replace github.com/yz89122/pgorm/v12 => ../..

require (
	github.com/yz89122/pgorm/v12 v12.0.0
	go.opentelemetry.io/otel v1.0.0
	go.opentelemetry.io/otel/trace v1.0.0
)
