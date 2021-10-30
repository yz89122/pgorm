package main

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	pg "github.com/yz89122/pgorm/v12"
	"github.com/yz89122/pgorm/v12/extra/pgotel"
)

var tracer = otel.Tracer("app_or_package_name")

func main() {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		panic(err)
	}

	provider := sdktrace.NewTracerProvider()
	provider.RegisterSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter))

	otel.SetTracerProvider(provider)

	db := pg.Connect(&pg.Options{
		Addr:     ":5432",
		User:     "postgres",
		Password: "postgres",
		Database: "example",
	})
	defer db.Close()

	db.AddQueryHook(pgotel.NewTracingHook())

	ctx, span := tracer.Start(context.TODO(), "main")
	defer span.End()

	if err := db.Ping(ctx); err != nil {
		panic(err)
	}
}
