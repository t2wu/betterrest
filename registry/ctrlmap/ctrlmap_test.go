package ctrlmap

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/t2wu/betterrest/controller"
)

type Ctrl1 struct {
}

// Initialize data for this REST operation
func (c *Ctrl1) Initialize(data *controller.ControllerInitData) {
}

type Ctrl2 struct {
}

// Initialize data for this REST operation
func (c *Ctrl2) Initialize(data *controller.ControllerInitData) {
}

func getType(obj interface{}) string {
	t := reflect.TypeOf(obj).String()
	return strings.Split(t, ".")[1]
}

func Test_ControllerMap_AddControllerWhoseFirstHookIsBefore_ShouldReturnOnlyWhenBeforeQueried(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "C", "JBAT")
	arr := c.InstantiateControllersWithFirstHookAt("C", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl1", getType(arr[0]))
	}
	arr = c.InstantiateControllersWithFirstHookAt("C", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddControllerWhoseFirstHookIsReadBefore_ShouldReturnOnReadAfterIsQueried(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "R", "JBAT")
	arr := c.InstantiateControllersWithFirstHookAt("R", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "A")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl1", getType(arr[0]))
	}
	arr = c.InstantiateControllersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddControllerWhoseFirstHookIsUpdate_ShouldReturnOnPatchBeforeIsQueried(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "U", "JBAT")
	arr := c.InstantiateControllersWithFirstHookAt("U", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl1", getType(arr[0]))
	}
	arr = c.InstantiateControllersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddControllerWhoseFirstHookIsPatchJSON_ShouldReturnOnPatchJsonIsQueried(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "P", "JBAT")
	arr := c.InstantiateControllersWithFirstHookAt("P", "J")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl1", getType(arr[0]))
	}
	arr = c.InstantiateControllersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddControllerWhoseFirstHookIsDeleteBefore_ShouldReturnOnDeleteBeforeIsQueried(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "D", "JBAT")
	arr := c.InstantiateControllersWithFirstHookAt("D", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl1", getType(arr[0]))
	}
	arr = c.InstantiateControllersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddControllerWithNoHookType_NoReturnedController(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "CRUPD", "")
	arr := c.InstantiateControllersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("C", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddControllerWithNoMethodAndNoHookType_NoReturnedController(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "", "")
	arr := c.InstantiateControllersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("C", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddControllerControllerWhoseFirstHookIsJson_ShouldReturnJsonQueriedExceptPatch(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "CRUPD", "JBAT")
	arr := c.InstantiateControllersWithFirstHookAt("C", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "J")
	assert.Len(t, arr, 0) // should not be there, because R has no before hook
	arr = c.InstantiateControllersWithFirstHookAt("U", "J")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "J")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("D", "J")
	assert.Len(t, arr, 0)

	arr = c.InstantiateControllersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0) // should not be there, because R has no before hook
	arr = c.InstantiateControllersWithFirstHookAt("U", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "B")
	assert.Len(t, arr, 1)

	arr = c.InstantiateControllersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "A")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.InstantiateControllersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddControllerControllerWhoseFirstHookIsBefore_ShouldReturnBeforeQueriedExceptRead(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "CRUPD", "BAT")
	arr := c.InstantiateControllersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0) // should not be there, because R has no before hook
	arr = c.InstantiateControllersWithFirstHookAt("U", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("D", "B")
	assert.Len(t, arr, 1)

	arr = c.InstantiateControllersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "A")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.InstantiateControllersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddControllerControllerWhoseFirstHookIsBefore_ShouldReturnBeforeQueriedExceptRead2(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "CRUPD", "BT")
	arr := c.InstantiateControllersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0) // should not be there, because R has no before hook
	arr = c.InstantiateControllersWithFirstHookAt("U", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("D", "B")
	assert.Len(t, arr, 1)

	arr = c.InstantiateControllersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.InstantiateControllersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddControllerControllerWhoseFirstHookIsAfter_ShouldReturnOnlyWhenAfterQueried(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "CRUPD", "AT")
	arr := c.InstantiateControllersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "B")
	assert.Len(t, arr, 0)

	arr = c.InstantiateControllersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("R", "A")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 1)

	arr = c.InstantiateControllersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddControllerControllerWhoseFirstHookIsTransact_ShouldReturnOnlyWhenTransactQueried(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "CRUPD", "T")
	arr := c.InstantiateControllersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "B")
	assert.Len(t, arr, 0)

	arr = c.InstantiateControllersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.InstantiateControllersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 1)
	arr = c.InstantiateControllersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 1)
}

func Test_ControllerMap_AddMultipleControllerWhoseFirstHookIsTransact_ShouldReturnOnlyTwoController(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "CRUPD", "BAT")
	c.RegisterController(&Ctrl2{}, "CRUPD", "BAT")

	arr := c.InstantiateControllersWithFirstHookAt("C", "B")
	assert.Len(t, arr, 2)
	arr = c.InstantiateControllersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "B")
	assert.Len(t, arr, 2)
	arr = c.InstantiateControllersWithFirstHookAt("P", "B")
	assert.Len(t, arr, 2)
	arr = c.InstantiateControllersWithFirstHookAt("D", "B")
	assert.Len(t, arr, 2)

	arr = c.InstantiateControllersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "A")
	assert.Len(t, arr, 2)
	arr = c.InstantiateControllersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.InstantiateControllersWithFirstHookAt("C", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "T")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "T")
	assert.Len(t, arr, 0)
}

func Test_ControllerMap_AddMultipleControllerWithDifferentFirstHook(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "CRUPD", "BT")
	c.RegisterController(&Ctrl2{}, "CRUPD", "T")

	arr := c.InstantiateControllersWithFirstHookAt("C", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl1", getType(arr[0]))
	}
	arr = c.InstantiateControllersWithFirstHookAt("R", "B")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl1", getType(arr[0]))
	}
	arr = c.InstantiateControllersWithFirstHookAt("P", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl1", getType(arr[0]))
	}
	arr = c.InstantiateControllersWithFirstHookAt("D", "B")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl1", getType(arr[0]))
	}

	arr = c.InstantiateControllersWithFirstHookAt("C", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("R", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("U", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("P", "A")
	assert.Len(t, arr, 0)
	arr = c.InstantiateControllersWithFirstHookAt("D", "A")
	assert.Len(t, arr, 0)

	arr = c.InstantiateControllersWithFirstHookAt("C", "T")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl2", getType(arr[0]))
	}
	arr = c.InstantiateControllersWithFirstHookAt("R", "T")
	if assert.Len(t, arr, 2) {
		assert.Equal(t, "Ctrl1", getType(arr[0]))
		assert.Equal(t, "Ctrl2", getType(arr[1]))
	}
	arr = c.InstantiateControllersWithFirstHookAt("U", "T")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl2", getType(arr[0]))
	}
	arr = c.InstantiateControllersWithFirstHookAt("P", "T")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl2", getType(arr[0]))
	}
	arr = c.InstantiateControllersWithFirstHookAt("D", "T")
	if assert.Len(t, arr, 1) {
		assert.Equal(t, "Ctrl2", getType(arr[0]))
	}
}

func Test_ControllerMap_AddController_ShouldReturnHavingController(t *testing.T) {
	c := NewCtrlMap()
	c.RegisterController(&Ctrl1{}, "C", "A")
	assert.True(t, c.HasRegisteredAnyController())
}
