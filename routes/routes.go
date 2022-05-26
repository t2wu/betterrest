package routes

import (
	"strings"

	"github.com/t2wu/betterrest/datamapper"
	"github.com/t2wu/betterrest/registry"

	"github.com/gin-gonic/gin"
)

func addRoute(r *gin.Engine, typeString string, reg *registry.Reg, mapper datamapper.IDataMapper) {
	endpoint := strings.ToLower(typeString)
	g := r.Group("/" + endpoint)
	{
		if strings.ContainsAny(reg.BatchMethods, "R") {
			g.GET("", w(GuardMiddleWare(typeString)),
				w(ReadManyHandler(typeString, mapper))) // e.g. GET /devices
		}

		if strings.ContainsAny(reg.BatchMethods, "C") {
			g.POST("", w(GuardMiddleWare(typeString)),
				w(CreateHandler(typeString, mapper)))
		}

		if strings.ContainsAny(reg.BatchMethods, "U") {
			g.PUT("", w(GuardMiddleWare(typeString)),
				w(UpdateManyHandler(typeString, mapper)))
		}

		if strings.ContainsAny(reg.BatchMethods, "P") {
			g.PATCH("", w(GuardMiddleWare(typeString)),
				w(PatchManyHandler(typeString, mapper)))
		}

		if strings.ContainsAny(reg.BatchMethods, "D") {
			g.DELETE("", w(GuardMiddleWare(typeString)),
				w(DeleteManyHandler(typeString, mapper)))
		}

		n := g.Group("/:id")
		{
			if strings.ContainsAny(reg.IdvMethods, "R") {
				// r.Use(OneMiddleWare(typeString))
				n.GET("", w(GuardMiddleWare(typeString)),
					w(ReadOneHandler(typeString, mapper))) // e.g. GET /model/123
			}

			if strings.ContainsAny(reg.IdvMethods, "U") {
				n.PUT("", w(GuardMiddleWare(typeString)),
					w(UpdateOneHandler(typeString, mapper))) // e.g. PUT /model/123
			}

			if strings.ContainsAny(reg.IdvMethods, "P") {
				n.PATCH("", w(GuardMiddleWare(typeString)),
					w(PatchOneHandler(typeString, mapper))) // e.g. PATCH /model/123
			}

			if strings.ContainsAny(reg.IdvMethods, "D") {
				n.DELETE("", w(GuardMiddleWare(typeString)),
					w(DeleteOneHandler(typeString, mapper))) // e.g. DELETE /model/123
			}
		}
	}
}

// AddRESTRoutes adds all routes
func AddRESTRoutes(r *gin.Engine) {
	registry.CreateBetterRESTTable()
	for typestring, reg := range registry.ModelRegistry {
		var dm datamapper.IDataMapper
		switch reg.Mapper {
		case registry.MapperTypeGlobal:
			dm = datamapper.SharedGlobalMapper()
			addRoute(r, typestring, reg, dm)
			break
		case registry.MapperTypeViaOrganization:
			dm = datamapper.SharedOrganizationMapper()
			addRoute(r, typestring, reg, dm)
			break
		case registry.MapperTypeViaOrgPartition:
			dm = datamapper.SharedOrgPartition()
			addRoute(r, typestring, reg, dm)
			break
		case registry.MapperTypeLinkTable:
			dm = datamapper.SharedLinkTableMapper()
			addRoute(r, typestring, reg, dm)
			break
		case registry.MapperTypeViaOwnership:
			dm = datamapper.SharedOwnershipMapper()
			addRoute(r, typestring, reg, dm)
			break
		case registry.MapperTypeUser:
			// don't add the user one
			break
		default:
			panic("adding unknow mapper")
		}
	}
}
