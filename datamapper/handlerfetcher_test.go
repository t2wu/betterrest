package datamapper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/registry/handlermap"
)

type HandlerJBAT struct {
}

func (c *HandlerJBAT) Init(data *hookhandler.InitData) {
}
func (c *HandlerJBAT) BeforeApply(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	return nil
}
func (c *HandlerJBAT) Before(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	return nil
}
func (c *HandlerJBAT) After(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	return nil
}
func (c *HandlerJBAT) AfterTransact(data *hookhandler.Data, info *hookhandler.EndPointInfo) {
}

type HandlerBAT struct {
	name    string // So we can wheck which handler
	checked bool
}

func (c *HandlerBAT) Init(data *hookhandler.InitData) {
}
func (c *HandlerBAT) Before(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	c.name = "beforeRun"
	return nil
}
func (c *HandlerBAT) After(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	if c.name == "beforeRun" {
		c.checked = true
	}
	return nil
}
func (c *HandlerBAT) AfterTransact(data *hookhandler.Data, info *hookhandler.EndPointInfo) {
}

type HandlerBA struct {
	name    string // So we can wheck which handler
	checked bool
}

func (c *HandlerBA) Init(data *hookhandler.InitData) {
}
func (c *HandlerBA) Before(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	c.name = "beforeRun"
	return nil
}
func (c *HandlerBA) After(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	if c.name == "beforeRun" {
		c.checked = true
	}
	return nil
}

type HandlerAT struct {
	name    string // So we can wheck which handler
	checked bool
}

func (c *HandlerAT) Init(data *hookhandler.InitData) {
}
func (c *HandlerAT) After(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	c.name = "afterRun"
	return nil
}
func (c *HandlerAT) AfterTransact(data *hookhandler.Data, info *hookhandler.EndPointInfo) {
	if c.name == "afterRun" {
		c.checked = true
	}
}

type HandlerT struct {
}

func (c *HandlerT) Init(data *hookhandler.InitData) {
}
func (c *HandlerT) AfterTransact(data *hookhandler.Data, info *hookhandler.EndPointInfo) {
}

func TestHandlerFetcher_FetchHandler_ShouldGetOnesForRegistered(t *testing.T) {
	cm := handlermap.NewHandlerMap()
	cm.RegisterHandler(&HandlerBAT{}, "CRUPD") // BAT
	cm.RegisterHandler(&HandlerAT{}, "CRUPD")  // AT

	f := NewHandlerFetcher(cm)
	handlers := f.FetchHandlersForOpAndHook(hookhandler.RESTOpCreate, "B")
	if assert.Len(t, handlers, 1) {
		_, ok := handlers[0].(*HandlerBAT)
		assert.True(t, ok)
	}
}

func TestHandlerFetcher_TheSameControllerIsRunInAnotherHook(t *testing.T) {
	cm := handlermap.NewHandlerMap()
	cm.RegisterHandler(&HandlerBA{}, "CRUPD")
	cm.RegisterHandler(&HandlerAT{}, "CRUPD")

	f := NewHandlerFetcher(cm)
	handlers := f.FetchHandlersForOpAndHook(hookhandler.RESTOpCreate, "B")
	if !assert.Len(t, handlers, 1) {
		return
	}

	for _, hdlr := range handlers {
		hdlr := hdlr.(hookhandler.IBefore)
		hdlr.Before(nil, nil)
	}

	handlers = f.FetchHandlersForOpAndHook(hookhandler.RESTOpCreate, "A")
	if !assert.Len(t, handlers, 2) {
		return
	}

	for _, hdlr := range handlers {
		hdlr := hdlr.(hookhandler.IAfter)
		hdlr.After(nil, nil)
	}

	hdlr, ok := handlers[0].(*HandlerBA)
	if assert.True(t, ok) {
		assert.True(t, hdlr.checked)
	}

	handlers = f.FetchHandlersForOpAndHook(hookhandler.RESTOpCreate, "T")
	if !assert.Len(t, handlers, 1) {
		return
	}

	for _, hdlr := range handlers {
		hdlr := hdlr.(hookhandler.IAfterTransact)
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

	f := NewHandlerFetcher(cm)                                             // this only handles either C, R, U, P, or D at one time though, agnostic.
	handlers := f.FetchHandlersForOpAndHook(hookhandler.RESTOpCreate, "B") // make it handle create
	if !assert.Len(t, handlers, 1) {
		return
	}

	f2 := NewHandlerFetcher(cm)
	handlers = f2.FetchHandlersForOpAndHook(hookhandler.RESTOpUpdate, "B") // make it handles update
	if !assert.Len(t, handlers, 2) {                                       // two hooks cuz update is
		return
	}

	assert.Len(t, f2.GetAllInstantiatedHanders(), 2) // two is instantiated because there are two handles U

	handlers = f.FetchHandlersForOpAndHook(hookhandler.RESTOpCreate, "A")
	if !assert.Len(t, handlers, 1) {
		return
	}

	handlers = f2.FetchHandlersForOpAndHook(hookhandler.RESTOpUpdate, "A")
	if !assert.Len(t, handlers, 2) {
		return
	}

	assert.Len(t, f.GetAllInstantiatedHanders(), 1)  // only one is instantiated, and handled for C
	assert.Len(t, f2.GetAllInstantiatedHanders(), 2) // only one is instantiated, and handled for U
}

func TestHandlerFetcher_HasController_ReportHavingController(t *testing.T) {
	cm := handlermap.NewHandlerMap()
	cm.RegisterHandler(&HandlerBA{}, "CRUPD")

	f := NewHandlerFetcher(cm)
	assert.True(t, f.HasRegisteredHandler())
}

func TestHandlerFetcher_HasNoControllerController_ReportHavingNoController(t *testing.T) {
	cm := handlermap.NewHandlerMap()

	f := NewHandlerFetcher(cm)
	assert.False(t, f.HasRegisteredHandler())
}
