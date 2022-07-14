package hfetcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/hook/rest"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/registry/handlermap"
)

type HandlerJBAT struct {
}

func (h *HandlerJBAT) Init(data *hook.InitData, args ...interface{}) {
}
func (h *HandlerJBAT) BeforeApply(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	return nil
}
func (h *HandlerJBAT) Before(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	return nil
}
func (h *HandlerJBAT) After(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	return nil
}
func (h *HandlerJBAT) AfterTransact(data *hook.Data, info *hook.EndPoint) {
}

type HandlerBAT struct {
	name       string // So we can wheck which handler
	checked    bool
	initCalled bool
}

func (h *HandlerBAT) Init(data *hook.InitData, args ...interface{}) {
	h.initCalled = true
}
func (h *HandlerBAT) Before(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	h.name = "beforeRun"
	return nil
}
func (h *HandlerBAT) After(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	if h.name == "beforeRun" {
		h.checked = true
	}
	return nil
}
func (h *HandlerBAT) AfterTransact(data *hook.Data, info *hook.EndPoint) {
}

type HandlerBA struct {
	name    string // So we can wheck which handler
	checked bool
}

func (h *HandlerBA) Init(data *hook.InitData, args ...interface{}) {
}
func (h *HandlerBA) Before(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	h.name = "beforeRun"
	return nil
}
func (h *HandlerBA) After(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	if h.name == "beforeRun" {
		h.checked = true
	}
	return nil
}

type HandlerAT struct {
	name       string // So we can wheck which handler
	checked    bool
	initCalled bool
}

func (h *HandlerAT) Init(data *hook.InitData, args ...interface{}) {
	h.initCalled = true
}
func (h *HandlerAT) After(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	h.name = "afterRun"
	return nil
}
func (h *HandlerAT) AfterTransact(data *hook.Data, info *hook.EndPoint) {
	if h.name == "afterRun" {
		h.checked = true
	}
}

type HandlerT struct {
}

func (h *HandlerT) Init(data *hook.InitData, args ...interface{}) {
}
func (h *HandlerT) AfterTransact(data *hook.Data, info *hook.EndPoint) {
}

func createInitData() *hook.InitData {
	return &hook.InitData{}
}

type HandlerNone struct {
}

func (h *HandlerNone) Init(data *hook.InitData, args ...interface{}) {
}

func TestHandlerFetcher_FetchHandler_ShouldGetOnesForRegistered(t *testing.T) {
	cm := handlermap.NewHandlerMap()
	cm.RegisterHandler(&HandlerBAT{}, "CRUPD") // BAT
	cm.RegisterHandler(&HandlerAT{}, "CRUPD")  // AT
	initData := createInitData()

	f := NewHandlerFetcher(cm, initData)
	handlers := f.FetchHandlersForOpAndHook(rest.OpCreate, "B")
	if assert.Len(t, handlers, 1) {
		_, ok := handlers[0].(*HandlerBAT)
		assert.True(t, ok)
	}
}

func TestHandlerFetcher_TheSameControllerIsRunInAnotherHook(t *testing.T) {
	cm := handlermap.NewHandlerMap()
	cm.RegisterHandler(&HandlerBA{}, "CRUPD")
	cm.RegisterHandler(&HandlerAT{}, "CRUPD")
	initData := createInitData()

	f := NewHandlerFetcher(cm, initData)
	handlers := f.FetchHandlersForOpAndHook(rest.OpCreate, "B")
	if !assert.Len(t, handlers, 1) {
		return
	}

	for _, hdlr := range handlers {
		hdlr := hdlr.(hook.IBefore)
		hdlr.Before(nil, nil)
	}

	handlers = f.FetchHandlersForOpAndHook(rest.OpCreate, "A")
	if !assert.Len(t, handlers, 2) {
		return
	}

	for _, hdlr := range handlers {
		hdlr := hdlr.(hook.IAfter)
		hdlr.After(nil, nil)
	}

	hdlr, ok := handlers[0].(*HandlerBA)
	if assert.True(t, ok) {
		assert.True(t, hdlr.checked)
	}

	handlers = f.FetchHandlersForOpAndHook(rest.OpCreate, "T")
	if !assert.Len(t, handlers, 1) {
		return
	}

	for _, hdlr := range handlers {
		hdlr := hdlr.(hook.IAfterTransact)
		hdlr.AfterTransact(nil, nil)
	}

	ctrl2, ok := handlers[0].(*HandlerAT)
	if assert.True(t, ok) {
		assert.True(t, ctrl2.checked)
	}
}

// GetAllInstantiatedHanders

func TestHandlerFetcher_RunControllers_CheckHandledInstances(t *testing.T) {
	cm := handlermap.NewHandlerMap()
	cm.RegisterHandler(&HandlerBAT{}, "CRUPD") // This handles CRUPD all at once
	cm.RegisterHandler(&HandlerBA{}, "RUPD")   // This handles RUPD all at once
	initData := createInitData()

	f := NewHandlerFetcher(cm, initData)                        // this only handles either C, R, U, P, or D at one time though, agnostic.
	handlers := f.FetchHandlersForOpAndHook(rest.OpCreate, "B") // make it handle create
	if !assert.Len(t, handlers, 1) {
		return
	}

	f2 := NewHandlerFetcher(cm, initData)
	handlers = f2.FetchHandlersForOpAndHook(rest.OpUpdate, "B") // make it handles update
	if !assert.Len(t, handlers, 2) {                            // two hooks cuz update is
		return
	}

	assert.Len(t, f2.GetAllInstantiatedHanders(), 2) // two is instantiated because there are two handles U

	handlers = f.FetchHandlersForOpAndHook(rest.OpCreate, "A")
	if !assert.Len(t, handlers, 1) {
		return
	}

	handlers = f2.FetchHandlersForOpAndHook(rest.OpUpdate, "A")
	if !assert.Len(t, handlers, 2) {
		return
	}

	assert.Len(t, f.GetAllInstantiatedHanders(), 1)  // only one is instantiated, and handled for C
	assert.Len(t, f2.GetAllInstantiatedHanders(), 2) // only one is instantiated, and handled for U
}

func TestHandlerFetcher_HasController_ReportHavingController(t *testing.T) {
	cm := handlermap.NewHandlerMap()
	cm.RegisterHandler(&HandlerBA{}, "CRUPD")
	initData := createInitData()

	f := NewHandlerFetcher(cm, initData)
	assert.True(t, f.HasRegisteredValidHandler())
}

func TestHandlerFetcher_HasNoControllerController_ReportHavingNoController(t *testing.T) {
	cm := handlermap.NewHandlerMap()
	initData := createInitData()

	f := NewHandlerFetcher(cm, initData)
	assert.False(t, f.HasRegisteredValidHandler())
}

func TestHandlerFetcher_HasControllerWithoutCallback_ReportHavingNoController(t *testing.T) {
	cm := handlermap.NewHandlerMap()
	cm.RegisterHandler(&HandlerNone{}, "CRUPD")
	initData := createInitData()

	f := NewHandlerFetcher(cm, initData)
	assert.False(t, f.HasRegisteredValidHandler())
}

func TestHandlerFetcher_ShouldCallInit(t *testing.T) {
	cm := handlermap.NewHandlerMap()
	cm.RegisterHandler(&HandlerBAT{}, "CRUPD") // BAT
	initData := createInitData()

	f := NewHandlerFetcher(cm, initData)
	handlers := f.FetchHandlersForOpAndHook(rest.OpCreate, "B")
	if assert.Len(t, handlers, 1) {
		assert.True(t, handlers[0].(*HandlerBAT).initCalled)
	}
}
