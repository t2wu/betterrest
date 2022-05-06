package handlermap

import (
	"reflect"
	"strings"

	"github.com/t2wu/betterrest/hookhandler"
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
	// HTTP method (CRUPD) -> Hook (BAT) -> Controllers
	// Method: CRUPD
	// Hook: JBAT (J for before json patch is applied, only valid for patch)
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
func (h *HandlerMap) RegisterHandler(hdlr hookhandler.IHookhandler, restMethods string, args ...interface{}) {
	// func (h *HandlerMap) RegisterHandler(hdlr hookhandler.IHookhandler, restMethods string, args ...interface{}) {
	h.hasAtLeastOneControllerAttemptRegistered = true
	typ := reflect.TypeOf(hdlr).Elem()
	handlerTypeAndArg := HandlerTypeAndArgs{
		HandlerType: typ,
		Args:        args,
	}
	if strings.Contains(restMethods, "C") {
		if firstHook := h.getFirstHookType(hookhandler.RESTOpCreate, handlerTypeAndArg.HandlerType); firstHook != "" {
			h.putControllerWithMethodAndHookInMap("C", firstHook, handlerTypeAndArg)
		}
	}
	if strings.Contains(restMethods, "R") {
		if firstHook := h.getFirstHookType(hookhandler.RESTOpRead, handlerTypeAndArg.HandlerType); firstHook != "" {
			h.putControllerWithMethodAndHookInMap("R", firstHook, handlerTypeAndArg)
		}
	}
	if strings.Contains(restMethods, "U") {
		if firstHook := h.getFirstHookType(hookhandler.RESTOpUpdate, handlerTypeAndArg.HandlerType); firstHook != "" {
			h.putControllerWithMethodAndHookInMap("U", firstHook, handlerTypeAndArg)
		}
	}
	if strings.Contains(restMethods, "P") {
		if firstHook := h.getFirstHookType(hookhandler.RESTOpPatch, handlerTypeAndArg.HandlerType); firstHook != "" {
			h.putControllerWithMethodAndHookInMap("P", firstHook, handlerTypeAndArg)
		}
	}
	if strings.Contains(restMethods, "D") {
		if firstHook := h.getFirstHookType(hookhandler.RESTOpDelete, handlerTypeAndArg.HandlerType); firstHook != "" {
			h.putControllerWithMethodAndHookInMap("D", firstHook, handlerTypeAndArg)
		}
	}
}

// GetHandlerTypeAndArgWithFirstHookAt obtains relevant handler and args if in this method and in this hook
// we should instantiate a new hookhandler.
// The first available hook type of R is B
// The first available hook type for P is J
func (h *HandlerMap) GetHandlerTypeAndArgWithFirstHookAt(method string, firstHook string) []HandlerTypeAndArgs {
	arr := make([]HandlerTypeAndArgs, 0)
	return append(arr, h.controllerMap[method][firstHook]...)
}

func (h *HandlerMap) HasRegisteredAnyHandlerWithHooks() bool {
	return h.hasAtLeastOneControllerWithHooksRegistered
}

func (h *HandlerMap) HasAttemptRegisteringAnyHandler() bool {
	return h.hasAtLeastOneControllerAttemptRegistered
}

// --- private ---
// func (h *HandlerMap) getFirstHookType(op hookhandler.RESTOp, hdlr hookhandler.IHookhandler) string {
func (h *HandlerMap) getFirstHookType(op hookhandler.RESTOp, handlerType reflect.Type) string {
	hdlr := reflect.New(handlerType).Interface()
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
