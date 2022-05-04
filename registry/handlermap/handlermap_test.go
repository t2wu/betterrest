package handlermap

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/hookhandler"
	"github.com/t2wu/betterrest/libs/webrender"
)

type Handler1NoHook struct {
}

func (c *Handler1NoHook) Init(data *hookhandler.InitData) {
}

type Handler1FirstHookBeforeApply struct {
}

func (c *Handler1FirstHookBeforeApply) Init(data *hookhandler.InitData) {
}
func (c *Handler1FirstHookBeforeApply) BeforeApply(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	return nil
}
func (c *Handler1FirstHookBeforeApply) Before(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	return nil
}
func (c *Handler1FirstHookBeforeApply) After(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	return nil
}
func (c *Handler1FirstHookBeforeApply) AfterTransact(data *hookhandler.Data, info *hookhandler.EndPointInfo) {
}

type Handler1FirstHookBefore struct {
}

func (c *Handler1FirstHookBefore) Init(data *hookhandler.InitData) {
}
func (c *Handler1FirstHookBefore) Before(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	return nil
}
func (c *Handler1FirstHookBefore) After(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	return nil
}
func (c *Handler1FirstHookBefore) AfterTransact(data *hookhandler.Data, info *hookhandler.EndPointInfo) {
}

type Handler1FirstHookAfter struct {
}

func (c *Handler1FirstHookAfter) Init(data *hookhandler.InitData) {
}
func (c *Handler1FirstHookAfter) After(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	return nil
}
func (c *Handler1FirstHookAfter) AfterTransact(data *hookhandler.Data, info *hookhandler.EndPointInfo) {
}

type Handler1FirstHookTransact struct {
}

func (c *Handler1FirstHookTransact) Init(data *hookhandler.InitData) {
}
func (c *Handler1FirstHookTransact) AfterTransact(data *hookhandler.Data, info *hookhandler.EndPointInfo) {
}

type Handler2FirstHookBefore struct {
}

func (c *Handler2FirstHookBefore) Init(data *hookhandler.InitData) {
}
func (c *Handler2FirstHookBefore) Before(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	return nil
}
func (c *Handler2FirstHookBefore) After(data *hookhandler.Data, info *hookhandler.EndPointInfo) *webrender.RetError {
	return nil
}
func (c *Handler2FirstHookBefore) AfterTransact(data *hookhandler.Data, info *hookhandler.EndPointInfo) {
}

type Handler2FirstHookAfterTransact struct {
}

func (c *Handler2FirstHookAfterTransact) Init(data *hookhandler.InitData) {
}
func (c *Handler2FirstHookAfterTransact) AfterTransact(data *hookhandler.Data, info *hookhandler.EndPointInfo) {
}

func getType(obj interface{}) string {
	t := reflect.TypeOf(obj).String()
	return strings.Split(t, ".")[1]
}

func Test_ControllerMap_AddHookHandlerWhenNoHook_ShouldNotRegisterAnyHandler(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1NoHook{}, "C")
	arr := c.InstantiateHandlersWithFirstHookAt("C", "J") // has this, but shouldn't response to this
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	assert.Equal(t, 0, c.HasRegisteredAnyHandler())
}

func Test_ControllerMap_AddHookHandlerWhoseFirstHookIsBefore_ShouldReturnOnlyWhenBeforeQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "C")
	arr := c.InstantiateHandlersWithFirstHookAt("C", "J") // has this, but shouldn't response to this
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("C", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler1FirstHookBeforeApply", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerWhoseFirstHookIsReadBefore_ShouldReturnOnReadAfterIsQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "R") // should respond to after
	arr := c.InstantiateHandlersWithFirstHookAt("R", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "A")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler1FirstHookBeforeApply", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerWhoseFirstHookIsUpdate_ShouldReturnOnPatchBeforeIsQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "U")
	arr := c.InstantiateHandlersWithFirstHookAt("U", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler1FirstHookBeforeApply", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerWhoseFirstHookIsPatchJSON_ShouldReturnOnPatchJsonIsQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "P")
	arr := c.InstantiateHandlersWithFirstHookAt("P", "J")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler1FirstHookBeforeApply", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerWhoseFirstHookIsDeleteBefore_ShouldReturnOnDeleteBeforeIsQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "D")
	arr := c.InstantiateHandlersWithFirstHookAt("D", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler1FirstHookBeforeApply", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerWithNoMethod_NoReturnedController(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "")
	arr := c.InstantiateHandlersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("C", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerControllerWhoseFirstHookIsJson_ShouldReturnJsonQueriedExceptPatch(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBeforeApply{}, "CRUPD")
	arr := c.InstantiateHandlersWithFirstHookAt("C", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "J")
	assert.Len(t, arr, 0) // should not be there, because R has no before hook
	arr = c.InstantiateHandlersWithFirstHookAt("U", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "J")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "J")
	assert.Len(t, arr, 0)

	arr = c.InstantiateHandlersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0) // should not be there, because R has no before hook
	arr = c.InstantiateHandlersWithFirstHookAt("U", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "B")
	assert.Len(t, arr, 1)

	arr = c.InstantiateHandlersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "A")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.InstantiateHandlersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerControllerWhoseFirstHookIsBefore_ShouldReturnBeforeQueriedExceptRead(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBefore{}, "CRUPD")
	arr := c.InstantiateHandlersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0) // should not be there, because R has no before hook
	arr = c.InstantiateHandlersWithFirstHookAt("U", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "B")
	assert.Len(t, arr, 1)

	arr = c.InstantiateHandlersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "A")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.InstantiateHandlersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerControllerWhoseFirstHookIsAfter_ShouldReturnOnlyWhenAfterQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookAfter{}, "CRUPD")
	arr := c.InstantiateHandlersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "B")
	assert.Len(t, arr, 0)

	arr = c.InstantiateHandlersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "A")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 1)

	arr = c.InstantiateHandlersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddHookHandlerControllerWhoseFirstHookIsTransact_ShouldReturnOnlyWhenTransactQueried(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookTransact{}, "CRUPD")
	arr := c.InstantiateHandlersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "B")
	assert.Len(t, arr, 0)

	arr = c.InstantiateHandlersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.InstantiateHandlersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 1)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 1)
}

func Test_ControllerMap_AddMultipleControllerWhoseFirstHookIsBefore_ShouldReturnOnlyTwoController(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBefore{}, "CRUPD")
	c.RegisterHandler(&Handler2FirstHookBefore{}, "CRUPD")

	arr := c.InstantiateHandlersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 2)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "B")
	assert.Len(t, arr, 2)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 2)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "B")
	assert.Len(t, arr, 2)

	arr = c.InstantiateHandlersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "A")
	assert.Len(t, arr, 2)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.InstantiateHandlersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddMultipleControllerWithDifferentFirstHook(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookBefore{}, "CRUPD")
	c.RegisterHandler(&Handler2FirstHookAfterTransact{}, "CRUPD")

	arr := c.InstantiateHandlersWithFirstHookAt("C", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler1FirstHookBefore", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("U", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler1FirstHookBefore", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("P", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler1FirstHookBefore", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("D", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler1FirstHookBefore", getType(arr[0]))
	}

	arr = c.InstantiateHandlersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("R", "A")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler1FirstHookBefore", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateHandlersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.InstantiateHandlersWithFirstHookAt("C", "T")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler2FirstHookAfterTransact", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("R", "T")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler2FirstHookAfterTransact", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("U", "T")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler2FirstHookAfterTransact", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("P", "T")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler2FirstHookAfterTransact", getType(arr[0]))
	}
	arr = c.InstantiateHandlersWithFirstHookAt("D", "T")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Handler2FirstHookAfterTransact", getType(arr[0]))
	}
}

func Test_ControllerMap_AddHookHandler_ShouldReturnHavingController(t *testing.T) {
	c := NewHandlerMap()
	c.RegisterHandler(&Handler1FirstHookAfter{}, "C")
	assert.True(t, c.HasRegisteredAnyHandler())
}
