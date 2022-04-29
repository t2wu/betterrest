package datamapper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/controller"
	"github.com/t2wu/betterrest/libs/webrender"
	"github.com/t2wu/betterrest/registry/ctrlmap"
)

type Ctrl1 struct {
	name    string // So we can wheck which controller
	checked bool
}

// Initialize data for this REST operation
func (c *Ctrl1) Initialize(data *controller.ControllerInitData) {
}

func (c *Ctrl1) Before(data *controller.Data, info *controller.EndPointInfo) *webrender.RetError {
	c.name = "beforeRun"
	return nil
}

func (c *Ctrl1) After(data *controller.Data, info *controller.EndPointInfo) *webrender.RetError {
	if c.name == "beforeRun" {
		c.checked = true
	}
	return nil
}

type Ctrl2 struct {
	name    string // So we can wheck which controller
	checked bool
}

// Initialize data for this REST operation
func (c *Ctrl2) Initialize(data *controller.ControllerInitData) {
}

func (c *Ctrl2) After(data *controller.Data, info *controller.EndPointInfo) *webrender.RetError {
	c.name = "afterRun"
	return nil
}

func (c *Ctrl2) AfterTransact(data *controller.Data, info *controller.EndPointInfo) {
	if c.name == "afterRun" {
		c.checked = true
	}
}

func TestCtrlFetcher_FetchController_ShouldGetOnesForRegistered1(t *testing.T) {
	cm := ctrlmap.NewCtrlMap()
	cm.RegisterController(&Ctrl1{}, "CRUPD", "BAT")
	cm.RegisterController(&Ctrl2{}, "CRUPD", "A")

	f := NewCtrlFetcher(cm)
	controllers := f.FetchControllersForOpAndHook(controller.RESTOpCreate, "B")
	if assert.Len(t, controllers, 1) {
		_, ok := controllers[0].(*Ctrl1)
		assert.True(t, ok)
	}
}
func TestCtrlFetcher_FetchController_ShouldGetOnesForRegistered2(t *testing.T) {
	cm := ctrlmap.NewCtrlMap()
	cm.RegisterController(&Ctrl1{}, "CRUPD", "BAT")
	cm.RegisterController(&Ctrl2{}, "CRUPD", "A")

	f := NewCtrlFetcher(cm)
	controllers := f.FetchControllersForOpAndHook(controller.RESTOpCreate, "A")
	if assert.Len(t, controllers, 1) {
		_, ok := controllers[0].(*Ctrl2)
		assert.True(t, ok)
	}
}

func TestCtrlFetcher_TheSameControllerIsRunInAnotherHook(t *testing.T) {
	cm := ctrlmap.NewCtrlMap()
	cm.RegisterController(&Ctrl1{}, "CRUPD", "BA")
	cm.RegisterController(&Ctrl2{}, "CRUPD", "AT")

	f := NewCtrlFetcher(cm)
	controllers := f.FetchControllersForOpAndHook(controller.RESTOpCreate, "B")
	if !assert.Len(t, controllers, 1) {
		return
	}

	for _, ctrl := range controllers {
		ctrl := ctrl.(controller.IBefore)
		ctrl.Before(nil, nil)
	}

	controllers = f.FetchControllersForOpAndHook(controller.RESTOpCreate, "A")
	if !assert.Len(t, controllers, 2) {
		return
	}

	for _, ctrl := range controllers {
		ctrl := ctrl.(controller.IAfter)
		ctrl.After(nil, nil)
	}

	ctrl, ok := controllers[0].(*Ctrl1)
	if assert.True(t, ok) {
		assert.True(t, ctrl.checked)
	}

	controllers = f.FetchControllersForOpAndHook(controller.RESTOpCreate, "T")
	if !assert.Len(t, controllers, 1) {
		return
	}

	for _, ctrl := range controllers {
		ctrl := ctrl.(controller.IAfterTransact)
		ctrl.AfterTransact(nil, nil)
	}

	ctrl2, ok := controllers[0].(*Ctrl2)
	if assert.True(t, ok) {
		assert.True(t, ctrl2.checked)
	}
}

func TestCtrlFetcher_HasController_ReportHavingController(t *testing.T) {
	cm := ctrlmap.NewCtrlMap()
	cm.RegisterController(&Ctrl1{}, "CRUPD", "BA")

	f := NewCtrlFetcher(cm)
	assert.True(t, f.HasRegisteredController())
}

func TestCtrlFetcher_HasNoControllerController_ReportHavingNoController(t *testing.T) {
	cm := ctrlmap.NewCtrlMap()

	f := NewCtrlFetcher(cm)
	assert.False(t, f.HasRegisteredController())
}
