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
		if strings.ContainsAny(reg.BatchMethods, "R") {
			g.GET("", GuardMiddleWare(typeString),
				GetAllHandler(typeString, mapper)) // e.g. GET /devices
		}

		if strings.ContainsAny(reg.BatchMethods, "C") {
			g.POST("", GuardMiddleWare(typeString),
				CreateHandler(typeString, mapper))
		}

		if strings.ContainsAny(reg.BatchMethods, "U") {
			g.PUT("", GuardMiddleWare(typeString),
				UpdateManyHandler(typeString, mapper))
		}

		if strings.ContainsAny(reg.BatchMethods, "P") {
			g.PATCH("", GuardMiddleWare(typeString),
				PatchManyHandler(typeString, mapper))
		}

		if strings.ContainsAny(reg.BatchMethods, "D") {
			g.DELETE("", GuardMiddleWare(typeString),
				DeleteManyHandler(typeString, mapper))
		}

		n := g.Group("/:id")
		{
			if strings.ContainsAny(reg.IdvMethods, "R") {
				// r.Use(OneMiddleWare(typeString))
				n.GET("", GuardMiddleWare(typeString),
					ReadOneHandler(typeString, mapper)) // e.g. GET /model/123
			}

			if strings.ContainsAny(reg.IdvMethods, "U") {
				n.PUT("", GuardMiddleWare(typeString),
					UpdateOneHandler(typeString, mapper)) // e.g. PUT /model/123
			}

			if strings.ContainsAny(reg.IdvMethods, "P") {
				n.PATCH("", GuardMiddleWare(typeString),
					PatchOneHandler(typeString, mapper)) // e.g. PATCH /model/123
			}

			if strings.ContainsAny(reg.IdvMethods, "D") {
				n.DELETE("", GuardMiddleWare(typeString),
					DeleteOneHandler(typeString, mapper)) // e.g. DELETE /model/123
			}
		}
	}
}

// AddRESTRoutes adds all routes
func AddRESTRoutes(r *gin.Engine) {
	models.CreateBetterRESTTable()
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
			break
		case models.MapperTypeUser:
			// don't add the user one
			break
		default:
			panic("adding unknow mapper")
		}
	}
}
