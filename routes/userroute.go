package routes

import (
	"github.com/t2wu/betterrest/datamapper"

	"github.com/gin-gonic/gin"
)

// UserRoutes gets routing for user
func UserRoutes(endpoint string, r *gin.Engine) {
	g := r.Group(endpoint)
	typeString := "users"
	dm := datamapper.SharedUserMapper()

	// r.Get("/", ReadAllHandler("users"))
	// r.With(paginate).Get("/", ListArticles)
	g.POST("/", CreateOneHandler(typeString, dm))
	g.POST("/login", UserLoginHandler()) // no crud on this one...access db itself

	// g := r.Group(nil)
	g.Use(BearerAuthMiddleware()) // The following one needs authentication

	n := g.Group("/:id")
	{
		n.GET("/", ReadOneHandler(typeString, dm))
		n.PUT("/", UpdateOneHandler(typeString, dm))
		n.DELETE("/", DeleteOneHandler(typeString, dm))
	}
}
