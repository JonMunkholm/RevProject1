package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/JonMunkholm/RevProject1/internal/application"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()

	app := application.New()

	//Graceful shutdown of server, allows for DB to finish update or will notify of failure.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := app.Start(ctx)
	if err != nil {
		fmt.Println("failed to start app:", err)
	}

	// 		r.Use(middleware.RequestID)
	// 		r.Use(middleware.RealIP)

	// 		r.Post("/", cfg.MakeUser)
	// 	})

	// })

}
