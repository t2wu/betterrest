package routes

import (
	"strings"

	"github.com/t2wu/betterrest/datamapper"
	"github.com/t2wu/betterrest/models"

	"github.com/gin-gonic/gin"
)

func addRoute(r *gin.Engine, typeString string, mapper interface{}) {
	endpoint := strings.ToLower(typeString)
	g := r.Group("/" + endpoint)
	{
		g.GET("/", ReadAllHandler(typeString, mapper.(datamapper.IGetAllMapper))) // e.g. GET /devices
		// r.With(paginate).Get("/", ListArticles)
		g.POST("/", CreateOneHandler(typeString, mapper.(datamapper.ICreateOneMapper)))
		g.PUT("/", UpdateManyHandler(typeString, mapper.(datamapper.IUpdateManyMapper)))
		g.DELETE("/", DeleteManyHandler(typeString, mapper.(datamapper.IDeleteMany)))

		n := g.Group("/:id")
		{
			// r.Use(OneMiddleWare(typeString))
			n.GET("", ReadOneHandler(typeString, mapper.(datamapper.IGetOneWithIDMapper)))      // e.g. GET /model/123
			n.PUT("", UpdateOneHandler(typeString, mapper.(datamapper.IUpdateOneWithIDMapper))) // e.g. PUT /model/123
			n.PATCH("", PatchOneHandler(typeString, mapper.(datamapper.IPatchOneWithIDMapper))) // e.g. PATCH /model/123
			n.DELETE("", DeleteOneHandler(typeString, mapper.(datamapper.IDeleteOneWithID)))    // e.g. DELETE /model/123
		}
	}
}

// AddRESTRoutes adds all routes
func AddRESTRoutes(r *gin.Engine) {
	for typestring := range models.ModelRegistry {
		if typestring != "users" {
			// db.Shared().AutoMigrate(model) // how to make it work?
			dm := datamapper.SharedBasicMapper()
			addRoute(r, typestring, dm)
		}
	}
}
