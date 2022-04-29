package datamapper

import (
	"runtime/debug"

	"github.com/t2wu/betterrest/controller"
	"github.com/t2wu/betterrest/registry/ctrlmap"
)

func NewCtrlFetcher(controllerMap *ctrlmap.CtrlMap) *CtrlFetcher {
	if controllerMap == nil {
		debug.PrintStack()
		panic("controller fetcher has to have a controlmap")
	}

	return &CtrlFetcher{
		controllers:   make([]controller.IController, 0),
		controllerMap: controllerMap,
	}
}

type CtrlFetcher struct {
	controllers   []controller.IController
	controllerMap *ctrlmap.CtrlMap
}

// FetchControllersForOpAndHook fetches the releveant controller for this method and hook.
// If there is any controller whose first hook is this one, instantiate it.
// If there are already instantiated controller which handles this hook, fetch it as well.
// hook can be JBAT
func (c *CtrlFetcher) FetchControllersForOpAndHook(op controller.RESTOp, hook string) []controller.IController {
	var method string
	switch op {
	case controller.RESTOpCreate:
		method = "C"
	case controller.RESTOpRead:
		method = "R"
	case controller.RESTOpUpdate:
		method = "U"
	case controller.RESTOpPatch:
		method = "P"
	case controller.RESTOpDelete:
		method = "D"
	}

	// Fetch new controllers if any
	newControllersIfAny := c.controllerMap.InstantiateControllersWithFirstHookAt(method, hook)
	c.controllers = append(c.controllers, newControllersIfAny...) // add to all controllers

	// Check for all controllers with this interface and return it
	comformedCtrls := make([]controller.IController, 0)
	// Any old controllers which handles this hookpoint?
	for _, ctrl := range c.controllers {
		if _, ok := ctrl.(controller.IBeforeApply); ok && hook == "J" {
			comformedCtrls = append(comformedCtrls, ctrl)
		}
		if _, ok := ctrl.(controller.IBefore); ok && hook == "B" {
			comformedCtrls = append(comformedCtrls, ctrl)
		}
		if _, ok := ctrl.(controller.IAfter); ok && hook == "A" {
			comformedCtrls = append(comformedCtrls, ctrl)
		}
		if _, ok := ctrl.(controller.IAfterTransact); ok && hook == "T" {
			comformedCtrls = append(comformedCtrls, ctrl)
		}
	}

	return comformedCtrls
}

func (c *CtrlFetcher) HasRegisteredController() bool {
	return c.controllerMap.HasRegisteredAnyController()
}

func (c *CtrlFetcher) GetAllInstantiatedControllers() []controller.IController {
	return c.controllers
}
