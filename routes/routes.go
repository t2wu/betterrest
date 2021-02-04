package routes

import (
	"strings"

	"github.com/t2wu/betterrest/datamapper"
	"github.com/t2wu/betterrest/models"

	"github.com/gin-gonic/gin"
)

func addRoute(r *gin.Engine, typeString string, reg *models.Reg, mapper datamapper.IDataMapper) {
	endpoint := strings.ToLower(typeString)
	g := r.Group("/" + endpoint)
	{
		if strings.ContainsAny(reg.BatchEndpoints, "R") {
			g.GET("", guardMiddleWare(typeString),
				ReadAllHandler(typeString, mapper)) // e.g. GET /devices
		}

		if strings.ContainsAny(reg.BatchEndpoints, "C") {
			g.POST("", guardMiddleWare(typeString),
				CreateHandler(typeString, mapper))
		}

		if strings.ContainsAny(reg.BatchEndpoints, "U") {
			g.PUT("", guardMiddleWare(typeString),
				UpdateManyHandler(typeString, mapper))
		}

		if strings.ContainsAny(reg.BatchEndpoints, "P") {
			g.PATCH("", guardMiddleWare(typeString),
				PatchManyHandler(typeString, mapper))
		}

		if strings.ContainsAny(reg.BatchEndpoints, "D") {
			g.DELETE("", guardMiddleWare(typeString),
				DeleteManyHandler(typeString, mapper))
		}

		n := g.Group("/:id")
		{
			if strings.ContainsAny(reg.IDEndPoints, "R") {
				// r.Use(OneMiddleWare(typeString))
				n.GET("", guardMiddleWare(typeString),
					ReadOneHandler(typeString, mapper)) // e.g. GET /model/123
			}

			if strings.ContainsAny(reg.IDEndPoints, "U") {
				n.PUT("", guardMiddleWare(typeString),
					UpdateOneHandler(typeString, mapper)) // e.g. PUT /model/123
			}

			if strings.ContainsAny(reg.IDEndPoints, "P") {
				n.PATCH("", guardMiddleWare(typeString),
					PatchOneHandler(typeString, mapper)) // e.g. PATCH /model/123
			}

			if strings.ContainsAny(reg.IDEndPoints, "D") {
				n.DELETE("", guardMiddleWare(typeString),
					DeleteOneHandler(typeString, mapper)) // e.g. DELETE /model/123
			}
		}
	}
}

// AddRESTRoutes adds all routes
func AddRESTRoutes(r *gin.Engine) {
	for typestring, reg := range models.ModelRegistry {
		var dm datamapper.IDataMapper
		switch reg.Mapper {
		case models.MapperTypeGlobal:
			dm = datamapper.SharedGlobalMapper()
			addRoute(r, typestring, reg, dm)
			break
		case models.MapperTypeViaOrganization:
			dm = datamapper.SharedOrganizationMapper()
			addRoute(r, typestring, reg, dm)
			break
		case models.MapperTypeLinkTable:
			dm = datamapper.SharedLinkTableMapper()
			addRoute(r, typestring, reg, dm)
			break
		case models.MapperTypeViaOwnership:
			dm = datamapper.SharedOwnershipMapper()
			addRoute(r, typestring, reg, dm)
			fallthrough
		case models.MapperTypeUser:
			// don't add user one
			break
		default:
			panic("adding unknow mapper")
		}
	}
}
