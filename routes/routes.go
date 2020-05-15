package routes

import (
	"strings"

	"github.com/t2wu/betterrest/datamapper"
	"github.com/t2wu/betterrest/models"

	"github.com/gin-gonic/gin"
)

func addRoute(r *gin.Engine, typeString string, reg *models.Reg, mapper interface{}) {
	endpoint := strings.ToLower(typeString)
	g := r.Group("/" + endpoint)
	{
		if strings.ContainsAny(reg.BatchEndpoints, "R") {
			g.GET("", ReadAllHandler(typeString, mapper.(datamapper.IGetAllMapper))) // e.g. GET /devices
		}

		if strings.ContainsAny(reg.BatchEndpoints, "C") {
			g.POST("", CreateOneHandler(typeString, mapper.(datamapper.ICreateOneMapper)))
		}

		if strings.ContainsAny(reg.BatchEndpoints, "U") {
			g.PUT("", UpdateManyHandler(typeString, mapper.(datamapper.IUpdateManyMapper)))
		}

		if strings.ContainsAny(reg.BatchEndpoints, "D") {
			g.DELETE("", DeleteManyHandler(typeString, mapper.(datamapper.IDeleteMany)))
		}

		n := g.Group("/:id")
		{
			if strings.ContainsAny(reg.IDEndPoints, "R") {
				// r.Use(OneMiddleWare(typeString))
				n.GET("", ReadOneHandler(typeString, mapper.(datamapper.IGetOneWithIDMapper))) // e.g. GET /model/123
			}

			if strings.ContainsAny(reg.IDEndPoints, "U") {
				n.PUT("", UpdateOneHandler(typeString, mapper.(datamapper.IUpdateOneWithIDMapper))) // e.g. PUT /model/123
			}

			if strings.ContainsAny(reg.IDEndPoints, "P") {
				n.PATCH("", PatchOneHandler(typeString, mapper.(datamapper.IPatchOneWithIDMapper))) // e.g. PATCH /model/123
			}

			if strings.ContainsAny(reg.IDEndPoints, "D") {
				n.DELETE("", DeleteOneHandler(typeString, mapper.(datamapper.IDeleteOneWithID))) // e.g. DELETE /model/123
			}
		}
	}
}

// AddRESTRoutes adds all routes
func AddRESTRoutes(r *gin.Engine) {
	for typestring, reg := range models.ModelRegistry {
		if typestring != "users" {
			// db.Shared().AutoMigrate(model) // how to make it work?
			dm := datamapper.SharedOwnershipMapper()
			addRoute(r, typestring, reg, dm)
		}
	}
}
