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
	g.POST("", guardMiddleWare(typeString), CreateHandler(typeString, dm))
	g.POST("/login", guardMiddleWare(typeString), UserLoginHandler()) // no crud on this one...access db itself

	// g := r.Group(nil)
	g.Use(BearerAuthMiddleware()) // The following one needs authentication

	n := g.Group("/:id")
	{
		n.GET("", guardMiddleWare(typeString), ReadOneHandler(typeString, dm))
		n.PUT("", guardMiddleWare(typeString), UpdateOneHandler(typeString, dm))
		n.DELETE("", guardMiddleWare(typeString), DeleteOneHandler(typeString, dm))
	}
}
