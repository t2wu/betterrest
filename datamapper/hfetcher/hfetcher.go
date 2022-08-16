package hfetcher

import (
	"reflect"
	"runtime/debug"

	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/rest"
	"github.com/t2wu/betterrest/registry/handlermap"
)

// NewHandlerFetcher maintains a list of instantiated handlers, if not, instantiate it.
// Where as NewHandlerMap only handles hook creation
// It doesn't care whether it's CRUPD, because the for each CRUPD a different HandlerFetcher is responsible.
func NewHandlerFetcher(handlerMap *handlermap.HandlerMap, initData *hook.InitData) *HandlerFetcher {
	if handlerMap == nil {
		debug.PrintStack()
		panic("hook fetcher has to have a controlmap")
	}

	return &HandlerFetcher{
		handlers:   make([]hook.IHook, 0),
		handlerMap: handlerMap,
		op:         rest.OpOther,
		initData:   initData,
	}
}

type HandlerFetcher struct {
	handlers   []hook.IHook
	handlerMap *handlermap.HandlerMap
	op         rest.Op
	initData   *hook.InitData
}

// FetchHandlersForOpAndHook fetches the releveant hook for this method and hookstr.
// If there is any hook whose first hookstr is this one, instantiate it.
// If there are already instantiated hook which handles this hookstr, fetch it as well.
// hookstr can be JBCATR (C is cache)
func (h *HandlerFetcher) FetchHandlersForOpAndHook(op rest.Op, hookstr string) []hook.IHook {
	// Make sure it's only used for one hookstr
	if h.op != rest.OpOther && h.op != op {
		panic("HandlerFetcher should only handles one method")
	}

	if h.op == rest.OpOther {
		h.op = op
	}

	var method string
	switch op {
	case rest.OpCreate:
		method = "C"
	case rest.OpRead:
		method = "R"
	case rest.OpUpdate:
		method = "U"
	case rest.OpPatch:
		method = "P"
	case rest.OpDelete:
		method = "D"
	}

	// Fetch new handlers and instantiate them if any
	newHandlerTypeAndArgIfAny := h.handlerMap.GetHandlerTypeAndArgWithFirstHookAt(method, hookstr)
	for _, newHandlerTypeAndArg := range newHandlerTypeAndArgIfAny {
		newHandler := reflect.New(newHandlerTypeAndArg.HandlerType).Interface().(hook.IHook)
		newHandler.Init(h.initData, newHandlerTypeAndArg.Args...) // dependency injection with h.args
		h.handlers = append(h.handlers, newHandler)               // add to all handlers
	}

	// Check for all handlers with this interface and return it
	comformedHandlers := make([]hook.IHook, 0)
	// Any old handlers which handles this hookpoint?
	for _, handler := range h.handlers {
		if _, ok := handler.(hook.IBeforeApply); ok && hookstr == "J" {
			comformedHandlers = append(comformedHandlers, handler)
		}
		if _, ok := handler.(hook.ICache); ok && hookstr == "C" {
			comformedHandlers = append(comformedHandlers, handler)
		}
		if _, ok := handler.(hook.IBefore); ok && hookstr == "B" {
			comformedHandlers = append(comformedHandlers, handler)
		}
		if _, ok := handler.(hook.IAfter); ok && hookstr == "A" {
			comformedHandlers = append(comformedHandlers, handler)
		}
		if _, ok := handler.(hook.IAfterTransact); ok && hookstr == "T" {
			comformedHandlers = append(comformedHandlers, handler)
		}
		if _, ok := handler.(hook.IRender); ok && hookstr == "R" {
			comformedHandlers = append(comformedHandlers, handler)
		}
	}

	return comformedHandlers
}

// func (h *HandlerFetcher) HasRegisteredValidHandler() bool {
// 	return h.handlerMap.HasRegisteredAnyHandlerWithHooks()
// }

// func (h *HandlerFetcher) HasAttemptRegisteringHandler() bool {
// 	return h.handlerMap.HasAttemptRegisteringAnyHandler()
// }

func (h *HandlerFetcher) GetAllInstantiatedHanders() []hook.IHook {
	return h.handlers
}
