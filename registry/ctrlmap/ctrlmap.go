package ctrlmap

import (
	"reflect"
	"strings"

	"github.com/t2wu/betterrest/controller"
)

func NewCtrlMap() *CtrlMap {
	return &CtrlMap{
		controllerMap: make(map[string]map[string][]reflect.Type),
	}
}

type CtrlMap struct {
	// HTTP method (CRUPD) -> Hook (BAT) -> Controllers
	// Method: CRUPD
	// Hook: JBAT (J for before json patch is applied, only valid for patch)
	controllerMap                     map[string]map[string][]reflect.Type
	hasAtLeastOneControllerRegistered bool
}

// RegisterController
// restMethod is CRUPD in any combination
// hookTypes is JBAT in any combination (where J is before JSON apply)
// The first available hook type of R is B
// The first available hook type for P is J
// BAT
// CRUPD, ABT --> Initalized with CB
// CR, B --> Initialized at create before or read before
// UP, A --> Initialized at Update after or patch after
// D, A --> Initialied at delete after
func (c *CtrlMap) RegisterController(ctrl controller.IController, restMethods string, hookTypes string) {
	firstHook := c.getFirstHookType(hookTypes)
	typ := reflect.TypeOf(ctrl).Elem()
	if strings.Contains(restMethods, "C") {
		c.putControllerWithMethodAndHookInMap("C", firstHook, typ)
	}
	if strings.Contains(restMethods, "R") {
		if firstHook == "B" {
			// read has no before controller, so add it to after if it has it, or T
			if strings.Contains(hookTypes, "A") {
				c.putControllerWithMethodAndHookInMap("R", "A", typ)
			} else if strings.Contains(hookTypes, "T") {
				c.putControllerWithMethodAndHookInMap("R", "T", typ)
			}
		} else {
			c.putControllerWithMethodAndHookInMap("R", firstHook, typ)
		}
	}
	if strings.Contains(restMethods, "U") {
		c.putControllerWithMethodAndHookInMap("U", firstHook, typ)
	}
	if strings.Contains(restMethods, "P") {
		// Well, unless there is a "J" in which case the first hook is always J
		if strings.Contains(hookTypes, "J") { // J is treated specially
			c.putControllerWithMethodAndHookInMap("P", "J", typ)
		} else {
			c.putControllerWithMethodAndHookInMap("P", firstHook, typ)
		}
	}
	if strings.Contains(restMethods, "D") {
		c.putControllerWithMethodAndHookInMap("D", firstHook, typ)
	}
}

// InstantiateControllersWithFirstHookAt instantiate new controller if in this method and in this hook
// we should instantiate a new controller.
// The first available hook type of R is B
// The first available hook type for P is J
func (c *CtrlMap) InstantiateControllersWithFirstHookAt(method string, firstHook string) []controller.IController {
	arr := make([]controller.IController, 0)
	for _, item := range c.controllerMap[method][firstHook] {
		arr = append(arr, reflect.New(item).Interface().(controller.IController))
	}

	return arr
}

func (c *CtrlMap) HasRegisteredAnyController() bool {
	return c.hasAtLeastOneControllerRegistered
}

// --- private ---

func (c *CtrlMap) getFirstHookType(hookType string) string {
	if strings.Contains(hookType, "B") {
		return "B"
	} else if strings.Contains(hookType, "A") {
		return "A"
	} else if strings.Contains(hookType, "T") {
		return "T"
	}
	return ""
}

func (c *CtrlMap) putControllerWithMethodAndHookInMap(method, firstHook string, typ reflect.Type) {
	if _, ok := c.controllerMap[method]; !ok {
		c.controllerMap[method] = make(map[string][]reflect.Type)
	}

	if _, ok := c.controllerMap[method][firstHook]; !ok {
		c.controllerMap[method][firstHook] = make([]reflect.Type, 0)
	}

	c.controllerMap[method][firstHook] = append(c.controllerMap[method][firstHook], typ)
	c.hasAtLeastOneControllerRegistered = true
}
