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

	// nasty issue: https://github.com/gin-gonic/gin/issues/1301
	// The following :id is bogus
	// I want the user to just do /#/sendverificationemail
	g.POST("/sendverificationemail", guardMiddleWare(typeString), SendVerificationEmailHandler(typeString, dm))
	g.POST("/verifyemail/:id/:code", guardMiddleWare(typeString), EmailVerificationHandler(typeString, dm))

	g.POST("/sendresetpasswordemail", guardMiddleWare(typeString), SendResetPasswordHandler(typeString, dm))
	g.POST("/resetpassword/:id/:code", guardMiddleWare(typeString), PasswordResetHandler(typeString, dm))

	g.POST("/login", guardMiddleWare(typeString), UserLoginHandler(typeString)) // no crud on this one...access db itself

	// g := r.Group(nil)
	g.Use(BearerAuthMiddleware()) // The following one needs authentication

	n := g.Group("/:id")
	{
		n.GET("", guardMiddleWare(typeString), ReadOneHandler(typeString, dm))
		n.PUT("", guardMiddleWare(typeString), UpdateOneHandler(typeString, dm))
		n.PUT("/changeemailpassword", guardMiddleWare(typeString), EmailChangePasswordHandler(typeString, dm))
		n.DELETE("", guardMiddleWare(typeString), DeleteOneHandler(typeString, dm))
	}
}

// UserRoutes gets routing for user
// func UserRoutes(endpoint string, r *gin.Engine) {
// 	g := r.Group(endpoint)
// 	typeString := "users"
// 	dm := datamapper.SharedUserMapper()

// 	// r.Get("/", ReadAllHandler("users"))
// 	// r.With(paginate).Get("/", ListArticles)
// 	g.POST("", guardMiddleWare(typeString), CreateHandler(typeString, dm))
// 	g.GET("/:id/verifyemail/:code", guardMiddleWare(typeString), EmailVerificationHandler(typeString))
// 	g.POST("/login", guardMiddleWare(typeString), UserLoginHandler(typeString)) // no crud on this one...access db itself

// 	// g := r.Group(nil)
// 	g.Use(BearerAuthMiddleware()) // The following one needs authentication

// 	n := g.Group("/:id")
// 	{
// 		n.GET("", guardMiddleWare(typeString), ReadOneHandler(typeString, dm))
// 		n.PUT("", guardMiddleWare(typeString), UpdateOneHandler(typeString, dm))
// 		n.PUT("/changeemailpassword", guardMiddleWare(typeString), EmailChangePasswordHandler(typeString, dm))
// 		n.DELETE("", guardMiddleWare(typeString), DeleteOneHandler(typeString, dm))
// 	}
// }
