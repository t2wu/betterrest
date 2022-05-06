package hfetcher

import (
	"reflect"
	"runtime/debug"

	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/registry/handlermap"
)

// NewHandlerFetcher maintains a list of instantiated handlers, if not, instantiate it.
// Where as NewHandlerMap only handles hookhandler creation
// It doesn't care whether it's CRUPD, because the for each CRUPD a different HandlerFetcher is responsible.
func NewHandlerFetcher(handlerMap *handlermap.HandlerMap, initData *hookhandler.InitData) *HandlerFetcher {
	if handlerMap == nil {
		debug.PrintStack()
		panic("hookhandler fetcher has to have a controlmap")
	}

	return &HandlerFetcher{
		handlers:   make([]hookhandler.IHookhandler, 0),
		handlerMap: handlerMap,
		op:         hookhandler.RESTOpOther,
		initData:   initData,
	}
}

type HandlerFetcher struct {
	handlers   []hookhandler.IHookhandler
	handlerMap *handlermap.HandlerMap
	op         hookhandler.RESTOp
	initData   *hookhandler.InitData
}

// FetchHandlersForOpAndHook fetches the releveant hookhandler for this method and hook.
// If there is any hookhandler whose first hook is this one, instantiate it.
// If there are already instantiated hookhandler which handles this hook, fetch it as well.
// hook can be JBAT
func (h *HandlerFetcher) FetchHandlersForOpAndHook(op hookhandler.RESTOp, hook string) []hookhandler.IHookhandler {
	// Make sure it's only used for one hook
	if h.op != hookhandler.RESTOpOther && h.op != op {
		panic("HandlerFetcher should only handles one method")
	}

	if h.op == hookhandler.RESTOpOther {
		h.op = op
	}

	var method string
	switch op {
	case hookhandler.RESTOpCreate:
		method = "C"
	case hookhandler.RESTOpRead:
		method = "R"
	case hookhandler.RESTOpUpdate:
		method = "U"
	case hookhandler.RESTOpPatch:
		method = "P"
	case hookhandler.RESTOpDelete:
		method = "D"
	}

	// Fetch new handlers and instantiate them if any
	newHandlerTypeAndArgIfAny := h.handlerMap.GetHandlerTypeAndArgWithFirstHookAt(method, hook)
	for _, newHandlerTypeAndArg := range newHandlerTypeAndArgIfAny {
		newHandler := reflect.New(newHandlerTypeAndArg.HandlerType).Interface().(hookhandler.IHookhandler)
		newHandler.Init(h.initData, newHandlerTypeAndArg.Args...) // dependency injection with h.args
		h.handlers = append(h.handlers, newHandler)               // add to all handlers
	}

	// Check for all handlers with this interface and return it
	comformedHandlers := make([]hookhandler.IHookhandler, 0)
	// Any old handlers which handles this hookpoint?
	for _, handler := range h.handlers {
		if _, ok := handler.(hookhandler.IBeforeApply); ok && hook == "J" {
			comformedHandlers = append(comformedHandlers, handler)
		}
		if _, ok := handler.(hookhandler.IBefore); ok && hook == "B" {
			comformedHandlers = append(comformedHandlers, handler)
		}
		if _, ok := handler.(hookhandler.IAfter); ok && hook == "A" {
			comformedHandlers = append(comformedHandlers, handler)
		}
		if _, ok := handler.(hookhandler.IAfterTransact); ok && hook == "T" {
			comformedHandlers = append(comformedHandlers, handler)
		}
	}

	return comformedHandlers
}

func (h *HandlerFetcher) HasRegisteredValidHandler() bool {
	return h.handlerMap.HasRegisteredAnyHandlerWithHooks()
}

func (h *HandlerFetcher) HasAttemptRegisteringHandler() bool {
	return h.handlerMap.HasAttemptRegisteringAnyHandler()
}

func (h *HandlerFetcher) GetAllInstantiatedHanders() []hookhandler.IHookhandler {
	return h.handlers
}
