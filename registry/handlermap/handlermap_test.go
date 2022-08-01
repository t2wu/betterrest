package handlermap

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/hook"
	"github.com/t2wu/betterrest/libs/webrender"
)

// IRender has not yet been tested

type Handler1NoHook struct {
}

func (c *Handler1NoHook) Init(data *hook.InitData, args ...interface{}) {
}

type Handler1FirstHookBeforeApply struct {
}

func (c *Handler1FirstHookBeforeApply) Init(data *hook.InitData, args ...interface{}) {
}
func (c *Handler1FirstHookBeforeApply) BeforeApply(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	return nil
}
func (c *Handler1FirstHookBeforeApply) Before(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	return nil
}
func (c *Handler1FirstHookBeforeApply) After(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	return nil
}
func (c *Handler1FirstHookBeforeApply) AfterTransact(data *hook.Data, info *hook.EndPoint) {
}

type Handler1FirstHookBefore struct {
}

func (c *Handler1FirstHookBefore) Init(data *hook.InitData, args ...interface{}) {
}
func (c *Handler1FirstHookBefore) Before(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	return nil
}
func (c *Handler1FirstHookBefore) After(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	return nil
}
func (c *Handler1FirstHookBefore) AfterTransact(data *hook.Data, info *hook.EndPoint) {
}

type Handler1FirstHookAfter struct {
}

func (c *Handler1FirstHookAfter) Init(data *hook.InitData, args ...interface{}) {
}
func (c *Handler1FirstHookAfter) After(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	return nil
}
func (c *Handler1FirstHookAfter) AfterTransact(data *hook.Data, info *hook.EndPoint) {
}

type Handler1FirstHookTransact struct {
}

func (c *Handler1FirstHookTransact) Init(data *hook.InitData, args ...interface{}) {
}
func (c *Handler1FirstHookTransact) AfterTransact(data *hook.Data, info *hook.EndPoint) {
}

type Handler2FirstHookBefore struct {
}

func (c *Handler2FirstHookBefore) Init(data *hook.InitData, args ...interface{}) {
}
func (c *Handler2FirstHookBefore) Before(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	return nil
}
func (c *Handler2FirstHookBefore) After(data *hook.Data, info *hook.EndPoint) *webrender.RetError {
	return nil
}
func (c *Handler2FirstHookBefore) AfterTransact(data *hook.Data, info *hook.EndPoint) {
}

type Handler2FirstHookAfterTransact struct {
}

func (c *Handler2FirstHookAfterTransact) Init(data *hook.InitData, args ...interface{}) {
}
func (c *Handler2FirstHookAfterTransact) AfterTransact(data *hook.Data, info *hook.EndPoint) {
}

func getType(obj interface{}) string {
	t := reflect.TypeOf(obj).String()
	return strings.Split(t, ".")[1]
}

// func Test_ControllerMap_AddHookHandlerWhenNoHook_ShouldNotRegisterAnyHandler(t *testing.T) {
// 	c := NewHandlerMap()
// 	c.RegisterHandler(&Handler1NoHook{}, "C")
// 	arr := c.GetHandlerTypeAndArgWithFirstHookAt("C", "J") // has this, but shouldn't response to this
// 	assert.Len(t, arr, 0)
// 	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "B")
// 	assert.Len(t, arr, 0)
// 	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "A")
// 	assert.Len(t, arr, 0)
// 	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "T")
// 	assert.Len(t, arr, 0)
// 	assert.False(t, c.HasRegisteredAnyHandlerWithHooks())
// }

func Test_ControllerMap_AddHookHandlerWhoseFirstHookIsBefore_ShouldReturnOnlyWhenBeforeQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "C")
	arr := c.GetHandlerTypeAndArgWithFirstHookAt("C", "J") // has this, but shouldn't response to this
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "B")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler1FirstHookBeforeApply)
		assert.True(t, ok)
	}
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerWhoseFirstHookIsReadBefore_ShouldReturnOnReadAfterIsQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "R") // should respond to after
	arr := c.GetHandlerTypeAndArgWithFirstHookAt("R", "J")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "A")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler1FirstHookBeforeApply)
		assert.True(t, ok)
	}
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerWhoseFirstHookIsUpdate_ShouldReturnOnPatchBeforeIsQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "U")
	arr := c.GetHandlerTypeAndArgWithFirstHookAt("U", "J")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "B")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler1FirstHookBeforeApply)
		assert.True(t, ok)
	}
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerWhoseFirstHookIsPatchJSON_ShouldReturnOnPatchJsonIsQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "P")
	arr := c.GetHandlerTypeAndArgWithFirstHookAt("P", "J")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler1FirstHookBeforeApply)
		assert.True(t, ok)
	}
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerWhoseFirstHookIsDeleteBefore_ShouldReturnOnDeleteBeforeIsQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "D")
	arr := c.GetHandlerTypeAndArgWithFirstHookAt("D", "J")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "B")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler1FirstHookBeforeApply)
		assert.True(t, ok)
	}
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerWithNoMethod_NoReturnedController(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "")
	arr := c.GetHandlerTypeAndArgWithFirstHookAt("C", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "J")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerControllerWhoseFirstHookIsJson_ShouldReturnJsonQueriedExceptPatch(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "CRUPD")
	arr := c.GetHandlerTypeAndArgWithFirstHookAt("C", "J")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "J")
	assert.Len(t, arr, 0) // should not be there, because R has no before hook
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "J")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "J")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "J")
	assert.Len(t, arr, 0)

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "B")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0) // should not be there, because R has no before hook
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "B")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "B")
	assert.Len(t, arr, 1)

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "A")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerControllerWhoseFirstHookIsBefore_ShouldReturnBeforeQueriedExceptRead(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBefore{}, "CRUPD")
	arr := c.GetHandlerTypeAndArgWithFirstHookAt("C", "B")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0) // should not be there, because R has no before hook
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "B")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "B")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "B")
	assert.Len(t, arr, 1)

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "A")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerControllerWhoseFirstHookIsAfter_ShouldReturnOnlyWhenAfterQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookAfter{}, "CRUPD")
	arr := c.GetHandlerTypeAndArgWithFirstHookAt("C", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "B")
	assert.Len(t, arr, 0)

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "A")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "A")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "A")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "A")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "A")
	assert.Len(t, arr, 1)

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerControllerWhoseFirstHookIsTransact_ShouldReturnOnlyWhenTransactQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookTransact{}, "CRUPD")
	arr := c.GetHandlerTypeAndArgWithFirstHookAt("C", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "B")
	assert.Len(t, arr, 0)

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "T")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "T")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "T")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "T")
	assert.Len(t, arr, 1)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "T")
	assert.Len(t, arr, 1)
}

func Test_ControllerMap_AddMultipleControllerWhoseFirstHookIsBefore_ShouldReturnOnlyTwoController(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBefore{}, "CRUPD")
	c.RegisterHandler(&Handler2FirstHookBefore{}, "CRUPD")

	arr := c.GetHandlerTypeAndArgWithFirstHookAt("C", "B")
	assert.Len(t, arr, 2)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "B")
	assert.Len(t, arr, 2)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "B")
	assert.Len(t, arr, 2)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "B")
	assert.Len(t, arr, 2)

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "A")
	assert.Len(t, arr, 2)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddMultipleControllerWithDifferentFirstHook(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBefore{}, "CRUPD")
	c.RegisterHandler(&Handler2FirstHookAfterTransact{}, "CRUPD")

	arr := c.GetHandlerTypeAndArgWithFirstHookAt("C", "B")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler1FirstHookBefore)
		assert.True(t, ok)
	}
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "B")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler1FirstHookBefore)
		assert.True(t, ok)
	}
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "B")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler1FirstHookBefore)
		assert.True(t, ok)
	}
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler1FirstHookBefore)
		assert.True(t, ok)
	}

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "A")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler1FirstHookBefore)
		assert.True(t, ok)
	}

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.GetHandlerTypeAndArgWithFirstHookAt("C", "T")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler2FirstHookAfterTransact)
		assert.True(t, ok)
	}
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("R", "T")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler2FirstHookAfterTransact)
		assert.True(t, ok)
	}
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("U", "T")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler2FirstHookAfterTransact)
		assert.True(t, ok)
	}
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("P", "T")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler2FirstHookAfterTransact)
		assert.True(t, ok)
	}
	arr = c.GetHandlerTypeAndArgWithFirstHookAt("D", "T")
	if assert.Len(t, arr, 1) {
		_, ok := reflect.New(arr[0].HandlerType).Interface().(*Handler2FirstHookAfterTransact)
		assert.True(t, ok)
	}
}

// func Test_ControllerMap_AddHookHandler_ShouldReturnHavingController(t *testing.T) {
// 	c := NewHandlerMap()
// 	c.RegisterHandler(&Handler1FirstHookAfter{}, "C")
// 	assert.True(t, c.HasRegisteredAnyHandlerWithHooks())
// }
