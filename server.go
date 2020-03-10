package main

import (
	"betterrest/routes"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
)

func main() {
	cors := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(300 * time.Second))
	r.Use(cors.Handler)

	// There needs to be client ID to create user
	r.Use(routes.ClientBasicAuthMiddleware)

	// Add user routes which is special
	r.Route("/users", routes.UserRoute())

	// Beyond this point there needs to be user login
	// r.Use(routes.BearerAuthMiddleware)
	// https://stackoverflow.com/questions/47957988/middleware-on-a-specific-route
	g := r.Group(nil)
	g.Use(routes.BearerAuthMiddleware)

	// Add all other routes for the model
	routes.AddAllRoutes(g)

	log.Println("Running on port 80")
	http.ListenAndServe(":80", r)
}
