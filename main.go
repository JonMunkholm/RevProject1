package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/JonMunkholm/RevProject1/internal/application"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	app, err := application.New()
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}

	// Graceful shutdown of server, allows for DB to finish update or will notify of failure.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := app.Start(ctx); err != nil {
		log.Fatalf("failed to start app: %v", err)
	}

	// 		r.Use(middleware.RequestID)
	// 		r.Use(middleware.RealIP)

	// 		r.Post("/", cfg.MakeUser)
	// 	})

	// })

}
