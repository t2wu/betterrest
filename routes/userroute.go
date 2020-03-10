package routes

import (
	"betterrest/datamapper"

	"github.com/go-chi/chi"
)

// UserRoute gets routing for user
func UserRoute() func(r chi.Router) {
	return func(r chi.Router) {
		typeString := "users"
		dm := datamapper.SharedUserMapper()

		r.Post("/", CreateOneHandler(typeString, dm))
		r.Post("/login", UserLoginHandler()) // no crud on this one...access db itself

		g := r.Group(nil)
		g.Use(BearerAuthMiddleware)

		g.Route("/{id}", func(r chi.Router) {
			// r.Use(OneMiddleWare(typeString))
			// r.Get("/", ReadOneHandler(typeString))
			// r.Put("/", UpdateOneHandler(typeString))
			// r.Delete("/", DeleteOneHandler(typeString))
		})
	}
}
