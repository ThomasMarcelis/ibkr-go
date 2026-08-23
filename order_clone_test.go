package ibkr

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestOrderClonesOwnAllMutableStorage(t *testing.T) {
	t.Parallel()

	t.Run("place order", func(t *testing.T) {
		assertOrderCloneOwnership(t, clonePlaceOrderRequest)
	})
	t.Run("place bracket", func(t *testing.T) {
		assertOrderCloneOwnership(t, clonePlaceBracketRequest)
	})
	t.Run("open order", func(t *testing.T) {
		assertOrderCloneOwnership(t, cloneOpenOrder)
	})
}

func assertOrderCloneOwnership[T any](t *testing.T, clone func(T) T) {
	t.Helper()

	// Generated values exercise the Go ownership graph only; they make no
	// claims about valid IBKR field combinations or wire values.
	original := new(T)
	populateOrderCloneValue(t, reflect.ValueOf(original).Elem(), "original")
	cloned := clone(*original)
	if !reflect.DeepEqual(*original, cloned) {
		t.Fatalf("clone changed value:\noriginal: %#v\nclone:    %#v", *original, cloned)
	}

	sourceStorage := make(map[uintptr]string)
	collectOrderCloneStorage(reflect.ValueOf(*original), "original", sourceStorage)
	cloneStorage := make(map[uintptr]string)
	collectOrderCloneStorage(reflect.ValueOf(cloned), "clone", cloneStorage)
	for address, sourcePath := range sourceStorage {
		if clonePath, shared := cloneStorage[address]; shared {
			t.Errorf("clone shares mutable storage at %s and %s", sourcePath, clonePath)
		}
	}
}

var (
	orderCloneDecimalType = reflect.TypeFor[decimal.Decimal]()
	orderCloneTimeType    = reflect.TypeFor[time.Time]()
)

func populateOrderCloneValue(t *testing.T, value reflect.Value, path string) {
	if value.Type() == orderCloneDecimalType {
		value.Set(reflect.ValueOf(decimal.RequireFromString("1.25")))
		return
	}
	if value.Type() == orderCloneTimeType {
		value.Set(reflect.ValueOf(time.Unix(1, 0)))
		return
	}
	if !value.CanSet() {
		t.Fatalf("cannot populate order clone field %s (%s)", path, value.Type())
	}

	switch value.Kind() {
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(1)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value.SetUint(1)
	case reflect.Float32, reflect.Float64:
		value.SetFloat(1)
	case reflect.String:
		value.SetString("value")
	case reflect.Pointer:
		value.Set(reflect.New(value.Type().Elem()))
		populateOrderCloneValue(t, value.Elem(), path+"*")
	case reflect.Slice:
		value.Set(reflect.MakeSlice(value.Type(), 1, 1))
		populateOrderCloneValue(t, value.Index(0), path+"[0]")
	case reflect.Struct:
		typeOfValue := value.Type()
		for i := range value.NumField() {
			populateOrderCloneValue(t, value.Field(i), path+"."+typeOfValue.Field(i).Name)
		}
	default:
		t.Fatalf("unsupported order clone field %s (%s)", path, value.Type())
	}
}

func collectOrderCloneStorage(value reflect.Value, path string, storage map[uintptr]string) {
	if value.Type() == orderCloneDecimalType || value.Type() == orderCloneTimeType {
		return
	}

	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return
		}
		storage[value.Pointer()] = path
		collectOrderCloneStorage(value.Elem(), path+"*", storage)
	case reflect.Slice:
		if value.IsNil() {
			return
		}
		if value.Len() > 0 {
			storage[value.Pointer()] = path
		}
		for i := range value.Len() {
			collectOrderCloneStorage(value.Index(i), fmt.Sprintf("%s[%d]", path, i), storage)
		}
	case reflect.Struct:
		typeOfValue := value.Type()
		for i := range value.NumField() {
			collectOrderCloneStorage(value.Field(i), path+"."+typeOfValue.Field(i).Name, storage)
		}
	}
}

func TestCloneOrderOwnsMutableInput(t *testing.T) {
	t.Parallel()

	// This ownership-only composite labels its independent evidence. The
	// transmit pointer comes from the 20260415 live false-then-true request
	// (events SHA-256 003abb59dfced54248d50644ec171c406aefc587141bdd7780fb44c4d59d0a45),
	// BAG identity/routing from the June paper combo order, and adaptive/price
	// condition values from their live campaigns. The 0.05 leg price and
	// explicit zero exempt code exercise official-schema presence laws; neither
	// is claimed as a live nondefault combo echo.
	original := PlaceOrderRequest{Contract: Contract{
		ConID: 28812380, SecType: SecTypeCombo, Strike: new(decimal.NewFromInt(0)),
		ComboLegs: []ComboLeg{{
			ConID: 878923092, Ratio: 1, Action: ActionSell, Exchange: "SMART", ExemptCode: new(0),
		}},
	}, Order: Order{
		Transmit:         new(false),
		AllOrNone:        new(true),
		Hedge:            OrderHedge{DisableAutomaticPrice: new(true)},
		UsePriceMgmtAlgo: new(true),
		Combo: OrderCombo{
			LegPrices:    []*decimal.Decimal{new(decimal.RequireFromString("0.05"))},
			SmartRouting: []TagValue{{Tag: "NonGuaranteed", Value: "1"}},
		},
		Algorithm:  OrderAlgorithm{Params: []TagValue{{Tag: "adaptivePriority", Value: "Normal"}}},
		Conditions: OrderConditions{Values: []OrderCondition{{Type: ConditionPrice}}},
	}}
	cloned := clonePlaceOrderRequest(original)

	original.Contract.ComboLegs[0].ConID = 886441502
	*original.Contract.ComboLegs[0].ExemptCode = 1
	*original.Contract.Strike = decimal.NewFromInt(1)
	*original.Order.Combo.LegPrices[0] = decimal.RequireFromString("291.09")
	original.Order.Combo.SmartRouting[0].Value = "0"
	original.Order.Algorithm.Params[0].Value = "Patient"
	original.Order.Conditions.Values[0].Type = ConditionTime
	*original.Order.Transmit = true
	*original.Order.AllOrNone = false
	*original.Order.Hedge.DisableAutomaticPrice = false
	*original.Order.UsePriceMgmtAlgo = false

	if cloned.Contract.ComboLegs[0].ConID != 878923092 || *cloned.Contract.ComboLegs[0].ExemptCode != 0 ||
		cloned.Contract.Strike == nil || !cloned.Contract.Strike.IsZero() || cloned.Order.Combo.LegPrices[0] == nil ||
		!cloned.Order.Combo.LegPrices[0].Equal(decimal.RequireFromString("0.05")) ||
		cloned.Order.Combo.SmartRouting[0].Value != "1" || cloned.Order.Algorithm.Params[0].Value != "Normal" ||
		cloned.Order.Conditions.Values[0].Type != ConditionPrice {
		t.Fatalf("clone shares nested slice storage: %#v", cloned)
	}

	pointers := []struct {
		name string
		got  *bool
		want bool
	}{
		{name: "Transmit", got: cloned.Order.Transmit, want: false},
		{name: "AllOrNone", got: cloned.Order.AllOrNone, want: true},
		{name: "Hedge.DisableAutomaticPrice", got: cloned.Order.Hedge.DisableAutomaticPrice, want: true},
		{name: "UsePriceMgmtAlgo", got: cloned.Order.UsePriceMgmtAlgo, want: true},
	}
	for _, pointer := range pointers {
		if pointer.got == nil || *pointer.got != pointer.want {
			t.Errorf("cloned %s = %v, want %t", pointer.name, pointer.got, pointer.want)
		}
	}
}
