package routes

import (
	"betterrest/datamapper"
	"betterrest/typeregistry"

	"github.com/go-chi/chi"
)

func addRoute(r chi.Router, typeString string, mapper interface{}) {
	endpoint := typeString
	r.Route("/"+endpoint, func(r chi.Router) {
		r.Get("/", ReadAllHandler(typeString, mapper.(datamapper.IGetAllMapper))) // e.g. GET /classes
		// r.With(paginate).Get("/", ListArticles)
		r.Post("/", CreateOneHandler(typeString, mapper.(datamapper.ICreateOneMapper)))
		r.Put("/", UpdateManyHandler(typeString, mapper.(datamapper.IUpdateManyMapper)))
		r.Delete("/", DeleteManyHandler(typeString, mapper.(datamapper.IDeleteMany)))

		r.Route("/{id}", func(r chi.Router) {
			// r.Use(OneMiddleWare(typeString))
			r.Get("/", ReadOneHandler(typeString, mapper.(datamapper.IGetOneWithIDMapper)))      // e.g. GET /classes/123
			r.Put("/", UpdateOneHandler(typeString, mapper.(datamapper.IUpdateOneWithIDMapper))) // e.g. PUT /classes/123
			r.Delete("/", DeleteOneHandler(typeString, mapper.(datamapper.IDeleteOneWithID)))    // e.g. DELETE /classes/123
		})
	})
}

// AddAllRoutes adds all routes
func AddAllRoutes(r chi.Router) {
	for typestring := range typeregistry.NewRegistry {
		if typestring != "users" {
			// db.Shared().AutoMigrate(model) // how to make it work?
			dm := datamapper.SharedBasicMapper()
			addRoute(r, typestring, dm)
		}
	}
}
