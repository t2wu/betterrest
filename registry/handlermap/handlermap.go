package handlermap

import (
	"reflect"
	"strings"

	"github.com/t2wu/betterrest/hookhandler"
)

func NewHandlerMap() *HandlerMap {
	return &HandlerMap{
		controllerMap: make(map[string]map[string][]reflect.Type),
	}
}

type HandlerMap struct {
	// HTTP method (CRUPD) -> Hook (BAT) -> Controllers
	// Method: CRUPD
	// Hook: JBAT (J for before json patch is applied, only valid for patch)
	controllerMap                     map[string]map[string][]reflect.Type
	hasAtLeastOneControllerRegistered bool
}

// RegisterHandler
// restMethod is CRUPD in any combination
// hookTypes is JBAT in any combination (where J is before JSON apply)
// The first available hook type of R is B
// The first available hook type for P is J
// BAT
// CRUPD, ABT --> Initalized with CB
// CR, B --> Initialized at create before or read before
// UP, A --> Initialized at Update after or patch after
// D, A --> Initialied at delete after
func (c *HandlerMap) RegisterHandler(hdlr hookhandler.IHookhandler, restMethods string) {
	typ := reflect.TypeOf(hdlr).Elem()
	if strings.Contains(restMethods, "C") {
		if firstHook := c.getFirstHookType(hookhandler.RESTOpCreate, hdlr); firstHook != "" {
			c.putControllerWithMethodAndHookInMap("C", firstHook, typ)
		}
	}
	if strings.Contains(restMethods, "R") {
		if firstHook := c.getFirstHookType(hookhandler.RESTOpRead, hdlr); firstHook != "" {
			c.putControllerWithMethodAndHookInMap("R", firstHook, typ)
		}
	}
	if strings.Contains(restMethods, "U") {
		if firstHook := c.getFirstHookType(hookhandler.RESTOpUpdate, hdlr); firstHook != "" {
			c.putControllerWithMethodAndHookInMap("U", firstHook, typ)
		}
	}
	if strings.Contains(restMethods, "P") {
		if firstHook := c.getFirstHookType(hookhandler.RESTOpPatch, hdlr); firstHook != "" {
			c.putControllerWithMethodAndHookInMap("P", firstHook, typ)
		}
	}
	if strings.Contains(restMethods, "D") {
		if firstHook := c.getFirstHookType(hookhandler.RESTOpDelete, hdlr); firstHook != "" {
			c.putControllerWithMethodAndHookInMap("D", firstHook, typ)
		}
	}
}

// InstantiateHandlersWithFirstHookAt instantiate new hookhandler if in this method and in this hook
// we should instantiate a new hookhandler.
// The first available hook type of R is B
// The first available hook type for P is J
func (c *HandlerMap) InstantiateHandlersWithFirstHookAt(method string, firstHook string) []hookhandler.IHookhandler {
	arr := make([]hookhandler.IHookhandler, 0)
	for _, item := range c.controllerMap[method][firstHook] {
		arr = append(arr, reflect.New(item).Interface().(hookhandler.IHookhandler))
	}

	return arr
}

func (c *HandlerMap) HasRegisteredAnyHandler() bool {
	return c.hasAtLeastOneControllerRegistered
}

// --- private ---
func (c *HandlerMap) getFirstHookType(op hookhandler.RESTOp, hdlr hookhandler.IHookhandler) string {
	if op == hookhandler.RESTOpPatch {
		if _, ok := hdlr.(hookhandler.IBeforeApply); ok {
			return "J"
		} else if _, ok := hdlr.(hookhandler.IBefore); ok {
			return "B"
		} else if _, ok := hdlr.(hookhandler.IAfter); ok {
			return "A"
		} else if _, ok := hdlr.(hookhandler.IAfterTransact); ok {
			return "T"
		}
	} else if op == hookhandler.RESTOpRead {
		if _, ok := hdlr.(hookhandler.IAfter); ok {
			return "A"
		} else if _, ok := hdlr.(hookhandler.IAfterTransact); ok {
			return "T"
		}
	}
	// Others, without valid before apply hook
	if _, ok := hdlr.(hookhandler.IBefore); ok {
		return "B"
	} else if _, ok := hdlr.(hookhandler.IAfter); ok {
		return "A"
	} else if _, ok := hdlr.(hookhandler.IAfterTransact); ok {
		return "T"
	}
	return ""
}

func (c *HandlerMap) putControllerWithMethodAndHookInMap(method, firstHook string, typ reflect.Type) {
	if _, ok := c.controllerMap[method]; !ok {
		c.controllerMap[method] = make(map[string][]reflect.Type)
	}

	if _, ok := c.controllerMap[method][firstHook]; !ok {
		c.controllerMap[method][firstHook] = make([]reflect.Type, 0)
	}

	c.controllerMap[method][firstHook] = append(c.controllerMap[method][firstHook], typ)
	c.hasAtLeastOneControllerRegistered = true
}
