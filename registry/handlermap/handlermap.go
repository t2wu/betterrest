package handlermap

import (
	"reflect"
	"strings"

	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/rest"
)

type HandlerTypeAndArgs struct {
	HandlerType reflect.Type
	Args        []interface{}
}

func NewHandlerMap() *HandlerMap {
	return &HandlerMap{
		controllerMap: make(map[string]map[string][]HandlerTypeAndArgs),
	}
}

type HandlerMap struct {
	// HTTP method (CRUPD) -> Hook (JBATR) -> Controllers
	// Method: CRUPD
	// Hook: JBATR (J for before json patch is applied, only valid for patch)
	// R is Render
	controllerMap                              map[string]map[string][]HandlerTypeAndArgs
	hasAtLeastOneControllerWithHooksRegistered bool
	hasAtLeastOneControllerAttemptRegistered   bool
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
func (h *HandlerMap) RegisterHandler(hdlr hook.IHook, restMethods string, args ...interface{}) {
	// func (h *HandlerMap) RegisterHandler(hdlr hook.IHook, restMethods string, args ...interface{}) {
	h.hasAtLeastOneControllerAttemptRegistered = true
	typ := reflect.TypeOf(hdlr).Elem()
	handlerTypeAndArg := HandlerTypeAndArgs{
		HandlerType: typ,
		Args:        args,
	}
	if strings.Contains(restMethods, "C") {
		if firstHook := h.getFirstHookType(rest.OpCreate, handlerTypeAndArg.HandlerType); firstHook != "" {
			h.putControllerWithMethodAndHookInMap("C", firstHook, handlerTypeAndArg)
		}
	}
	if strings.Contains(restMethods, "R") {
		if firstHook := h.getFirstHookType(rest.OpRead, handlerTypeAndArg.HandlerType); firstHook != "" {
			h.putControllerWithMethodAndHookInMap("R", firstHook, handlerTypeAndArg)
		}
	}
	if strings.Contains(restMethods, "U") {
		if firstHook := h.getFirstHookType(rest.OpUpdate, handlerTypeAndArg.HandlerType); firstHook != "" {
			h.putControllerWithMethodAndHookInMap("U", firstHook, handlerTypeAndArg)
		}
	}
	if strings.Contains(restMethods, "P") {
		if firstHook := h.getFirstHookType(rest.OpPatch, handlerTypeAndArg.HandlerType); firstHook != "" {
			h.putControllerWithMethodAndHookInMap("P", firstHook, handlerTypeAndArg)
		}
	}
	if strings.Contains(restMethods, "D") {
		if firstHook := h.getFirstHookType(rest.OpDelete, handlerTypeAndArg.HandlerType); firstHook != "" {
			h.putControllerWithMethodAndHookInMap("D", firstHook, handlerTypeAndArg)
		}
	}
}

// GetHandlerTypeAndArgWithFirstHookAt obtains relevant handler and args if in this method and in this hook
// we should instantiate a new hook.
// The first available hook type of R is B
// The first available hook type for P is J
func (h *HandlerMap) GetHandlerTypeAndArgWithFirstHookAt(method string, firstHook string) []HandlerTypeAndArgs {
	arr := make([]HandlerTypeAndArgs, 0)
	return append(arr, h.controllerMap[method][firstHook]...)
}

// func (h *HandlerMap) HasRegisteredAnyHandlerWithHooks() bool {
// 	return h.hasAtLeastOneControllerWithHooksRegistered
// }

// func (h *HandlerMap) HasAttemptRegisteringAnyHandler() bool {
// 	return h.hasAtLeastOneControllerAttemptRegistered
// }

// --- private ---
// func (h *HandlerMap) getFirstHookType(op rest.Op, hdlr hook.IHook) string {
func (h *HandlerMap) getFirstHookType(op rest.Op, handlerType reflect.Type) string {
	hdlr := reflect.New(handlerType).Interface()
	if op == rest.OpPatch {
		if _, ok := hdlr.(hook.IBeforeApply); ok {
			return "J"
		} else if _, ok := hdlr.(hook.IBefore); ok {
			return "B"
		} else if _, ok := hdlr.(hook.IAfter); ok {
			return "A"
		} else if _, ok := hdlr.(hook.IAfterTransact); ok {
			return "T"
		} else if _, ok := hdlr.(hook.IRender); ok {
			return "R"
		}
	} else if op == rest.OpRead {
		if _, ok := hdlr.(hook.IAfter); ok {
			return "A"
		} else if _, ok := hdlr.(hook.IAfterTransact); ok {
			return "T"
		} else if _, ok := hdlr.(hook.IRender); ok {
			return "R"
		}
	}
	// Others, without valid before apply hook
	if _, ok := hdlr.(hook.IBefore); ok {
		return "B"
	} else if _, ok := hdlr.(hook.IAfter); ok {
		return "A"
	} else if _, ok := hdlr.(hook.IAfterTransact); ok {
		return "T"
	} else if _, ok := hdlr.(hook.IRender); ok {
		return "R"
	}
	return ""
}

func (h *HandlerMap) putControllerWithMethodAndHookInMap(method, firstHook string, handlerTypeAndArgs HandlerTypeAndArgs) {
	if _, ok := h.controllerMap[method]; !ok {
		h.controllerMap[method] = make(map[string][]HandlerTypeAndArgs)
	}

	if _, ok := h.controllerMap[method][firstHook]; !ok {
		h.controllerMap[method][firstHook] = make([]HandlerTypeAndArgs, 0)
	}

	h.controllerMap[method][firstHook] = append(h.controllerMap[method][firstHook], handlerTypeAndArgs)
	h.hasAtLeastOneControllerWithHooksRegistered = true
}
